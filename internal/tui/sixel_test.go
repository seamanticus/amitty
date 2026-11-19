package tui

import (
	"image"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/internal/graphics"
)

func sixelScreen(t *testing.T, width, height int) Screen {
	t.Helper()
	screen, err := tcell.NewTerminfoScreenFromTty(vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)}))
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	return screen
}

// putImage draws the sixel placeholders of image id's rows and columns within crop at x, y.
func putImage(screen Screen, id, x, y int, crop image.Rectangle) {
	for row := crop.Min.Y; row < crop.Max.Y; row++ {
		for col := crop.Min.X; col < crop.Max.X; col++ {
			str, fg := graphics.Cell(id, row, col, true)
			screen.Put(x+col-crop.Min.X, y+row-crop.Min.Y, str, tcell.StyleDefault.Foreground(fg))
		}
	}
}

func TestSixelsPlacements(t *testing.T) {
	set := newSixels()
	set.images[1] = sixelImage{}
	set.images[2] = sixelImage{}

	// evidence: witnesses
	for _, test := range []struct {
		name string
		draw func(Screen)
		want []placement
	}{
		{"none", func(Screen) {}, nil},
		{
			"a whole image",
			func(s Screen) { putImage(s, 1, 1, 1, image.Rect(0, 0, 2, 2)) },
			[]placement{{1, Rectangle{1, 1, 2, 2}, image.Rect(0, 0, 2, 2)}},
		},
		{
			"an image scrolled up past the top",
			func(s Screen) { putImage(s, 1, 0, 0, image.Rect(0, 1, 3, 3)) },
			[]placement{{1, Rectangle{0, 0, 3, 2}, image.Rect(0, 1, 3, 3)}},
		},
		{
			"an image with a cell drawn over its middle",
			func(s Screen) {
				putImage(s, 1, 0, 0, image.Rect(0, 0, 3, 1))
				s.Put(1, 0, "x", tcell.StyleDefault)
			},
			[]placement{{1, Rectangle{0, 0, 1, 1}, image.Rect(0, 0, 1, 1)}, {1, Rectangle{2, 0, 1, 1}, image.Rect(2, 0, 3, 1)}},
		},
		{
			"an image with a row drawn over",
			func(s Screen) {
				putImage(s, 1, 0, 0, image.Rect(0, 0, 2, 3))
				s.Put(0, 1, "x", tcell.StyleDefault)
				s.Put(1, 1, "x", tcell.StyleDefault)
			},
			[]placement{{1, Rectangle{0, 0, 2, 1}, image.Rect(0, 0, 2, 1)}, {1, Rectangle{0, 2, 2, 1}, image.Rect(0, 2, 2, 3)}},
		},
		{
			"two images side by side",
			func(s Screen) {
				putImage(s, 1, 0, 0, image.Rect(0, 0, 2, 2))
				putImage(s, 2, 2, 0, image.Rect(0, 0, 2, 2))
			},
			[]placement{{1, Rectangle{0, 0, 2, 2}, image.Rect(0, 0, 2, 2)}, {2, Rectangle{2, 0, 2, 2}, image.Rect(0, 0, 2, 2)}},
		},
		{
			"the same image twice",
			func(s Screen) {
				putImage(s, 1, 0, 0, image.Rect(0, 0, 1, 1))
				putImage(s, 1, 0, 2, image.Rect(0, 0, 1, 1))
			},
			[]placement{{1, Rectangle{0, 0, 1, 1}, image.Rect(0, 0, 1, 1)}, {1, Rectangle{0, 2, 1, 1}, image.Rect(0, 0, 1, 1)}},
		},
		{"an image that is not set", func(s Screen) { putImage(s, 3, 0, 0, image.Rect(0, 0, 2, 2)) }, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			screen := sixelScreen(t, 5, 4)
			test.draw(screen)
			if got := set.placements(screen); !slices.Equal(got, test.want) {
				t.Fatalf("placements = %+v, want %+v", got, test.want)
			}
			// The terminal never sees a placeholder.
			for y := range 4 {
				for x := range 5 {
					if str, _, _ := screen.Get(x, y); graphics.IsSixelCell(str) {
						t.Fatalf("cell %d, %d is still a placeholder", x, y)
					}
				}
			}
		})
	}
}

func TestSixelsShow(t *testing.T) {
	screen := sixelScreen(t, 4, 2)
	set := newSixels()
	pixels := graphics.Quantize(image.NewGray(image.Rect(0, 0, 4, 4)), 4, 4)
	set.set(screen, cmd.SixelImage{ID: 1, Image: pixels, Cell: image.Pt(2, 2)})
	frame := func() {
		screen.Clear()
		putImage(screen, 1, 0, 0, image.Rect(0, 0, 2, 2))
		set.show(screen)
	}

	frame()
	shown := []placement{{1, Rectangle{0, 0, 2, 2}, image.Rect(0, 0, 2, 2)}}
	if !slices.Equal(set.shown, shown) || len(set.encoded) != 1 {
		t.Fatalf("shown %+v with %d encoded, want %+v with 1", set.shown, len(set.encoded), shown)
	}
	t.Run("an unchanged frame keeps the placement", func(t *testing.T) {
		frame()
		if !slices.Equal(set.shown, shown) {
			t.Fatalf("shown = %+v", set.shown)
		}
	})
	t.Run("replacing the image drops its placements and sixels", func(t *testing.T) {
		set.set(screen, cmd.SixelImage{ID: 1, Image: pixels, Cell: image.Pt(2, 2)})
		if len(set.shown) != 0 || len(set.encoded) != 0 {
			t.Fatalf("shown %+v with %d encoded, want none", set.shown, len(set.encoded))
		}
	})
	t.Run("a frame without the image drops its sixels", func(t *testing.T) {
		frame()
		screen.Clear()
		set.show(screen)
		if len(set.shown) != 0 || len(set.encoded) != 0 {
			t.Fatalf("shown %+v with %d encoded, want none", set.shown, len(set.encoded))
		}
	})
	t.Run("forgetting the image", func(t *testing.T) {
		set.set(screen, cmd.SixelImage{ID: 1})
		if _, ok := set.images[1]; ok {
			t.Fatal("the image is kept")
		}
	})
}
