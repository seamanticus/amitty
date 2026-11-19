package graphics

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	tcolor "github.com/gdamore/tcell/v3/color"
)

func TestParseSixelCell(t *testing.T) {
	// proof: exhaustive
	t.Run("round trips every id", func(t *testing.T) {
		for id := 1; id <= MaxID; id++ {
			for _, cell := range [][2]int{{0, 0}, {MaxCells - 1, MaxCells - 1}, {id % MaxCells, (id / MaxCells) % MaxCells}} {
				str, fg := Cell(id, cell[0], cell[1], true)
				gotID, row, col, ok := ParseSixelCell(str, fg)
				if !ok || gotID != id || row != cell[0] || col != cell[1] {
					t.Fatalf("ParseSixelCell(Cell(%d, %d, %d)) = %d, %d, %d, %v", id, cell[0], cell[1], gotID, row, col, ok)
				}
			}
		}
	})
	t.Run("round trips every row and column", func(t *testing.T) {
		for n := range MaxCells {
			str, fg := Cell(1, n, MaxCells-1-n, true)
			if _, row, col, ok := ParseSixelCell(str, fg); !ok || row != n || col != MaxCells-1-n {
				t.Fatalf("cell %d = %d, %d, %v", n, row, col, ok)
			}
		}
	})

	// evidence: witnesses
	kitty, kittyFg := Cell(1, 0, 0, false)
	sixel, fg := Cell(1, 0, 0, true)
	for _, test := range []struct {
		name string
		text string
		fg   tcolor.Color
	}{
		{"empty", "", fg},
		{"kitty's placeholder", kitty, kittyFg},
		{"a basic color", sixel, tcolor.PaletteColor(7)},
		{"a true color", sixel, tcolor.NewRGBColor(0, 0, 9)},
		{"the default color", sixel, tcolor.Default},
		{"no diacritics", string(SixelPlaceholder), fg},
		{"too few diacritics", string([]rune(sixel)[:3]), fg},
		{"too many diacritics", sixel + string(diacritics[0]), fg},
		{"not a diacritic", string([]rune{SixelPlaceholder, 'a', diacritics[0], diacritics[0]}), fg},
		{"an id past MaxID", string([]rune{SixelPlaceholder, diacritics[0], diacritics[0], diacritics[256]}), fg},
	} {
		t.Run("rejects "+test.name, func(t *testing.T) {
			if _, _, _, ok := ParseSixelCell(test.text, test.fg); ok {
				t.Fatal("parsed")
			}
		})
	}
}

func TestQuantize(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		src.Set(x, 0, color.NRGBA{R: 255, A: 255})
		src.Set(x, 1, color.NRGBA{R: 255, A: MinAlpha - 1})
	}
	img := Quantize(src, 4, 2)
	if img.Bounds() != image.Rect(0, 0, 4, 2) || len(img.Palette) != 256 {
		t.Fatalf("bounds %v with %d colors, want 4x2 with 256", img.Bounds(), len(img.Palette))
	}
	for x := range 4 {
		if top := img.Palette[img.ColorIndexAt(x, 0)]; top != (color.RGBA{0xff, 0x00, 0x00, 0xff}) {
			t.Fatalf("pixel %d, 0 = %v, want red", x, top)
		}
		if img.ColorIndexAt(x, 1) != transparent {
			t.Fatalf("pixel %d, 1 is drawn, want it transparent", x)
		}
	}
}

func TestSixel(t *testing.T) {
	palette := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	for len(palette) < transparent {
		palette = append(palette, color.RGBA{0, 0, 0, 255})
	}
	palette = append(palette, color.Transparent)

	// 6 wide by 7 tall: white on top, black in the middle row of the first band, a transparent bottom band but for one black pixel.
	img := image.NewPaletted(image.Rect(0, 0, 6, 7), palette)
	for i := range img.Pix {
		img.Pix[i] = transparent
	}
	for x := range 6 {
		img.SetColorIndex(x, 0, 1)
		img.SetColorIndex(x, 1, 0)
	}
	img.SetColorIndex(0, 6, 0)

	// evidence: witnesses
	for _, test := range []struct {
		name string
		r    image.Rectangle
		want string
	}{
		{
			"the whole image",
			img.Bounds(),
			"\x1bP0;1;0q\"1;1;6;7#0;2;0;0;0#1;2;100;100;100" +
				"#0!6A$#1!6@" + "-#0@!5?" + "\x1b\\",
		},
		{
			"a crop",
			image.Rect(1, 0, 3, 2),
			"\x1bP0;1;0q\"1;1;2;2#0;2;0;0;0#1;2;100;100;100" + "#0AA$#1@@" + "\x1b\\",
		},
		{
			"an empty crop",
			image.Rect(9, 9, 10, 10),
			"\x1bP0;1;0q\"1;1;0;0\x1b\\",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Sixel(img, test.r); !bytes.Equal(got, []byte(test.want)) {
				t.Fatalf("Sixel = %q, want %q", got, test.want)
			}
		})
	}
}
