package graphics

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"slices"

	xdraw "golang.org/x/image/draw"
)

// transparent is the palette index of pixels left undrawn; the other 255 are opaque.
const transparent = 255

// Quantize returns src scaled to width by height pixels and dithered to 255 colors, with pixels less opaque than MinAlpha at a transparent index.
func Quantize(src image.Image, width, height int) *image.Paletted {
	bounds := image.Rect(0, 0, width, height)
	scaled := image.NewRGBA(bounds)
	xdraw.CatmullRom.Scale(scaled, bounds, src, src.Bounds(), draw.Src, nil)

	// Dithering sees each pixel at its own color rather than darkened by its alpha.
	opaque := image.NewNRGBA(bounds)
	for i := 0; i < len(scaled.Pix); i += 4 {
		c := color.NRGBAModel.Convert(color.RGBA{scaled.Pix[i], scaled.Pix[i+1], scaled.Pix[i+2], scaled.Pix[i+3]}).(color.NRGBA)
		opaque.Pix[i], opaque.Pix[i+1], opaque.Pix[i+2], opaque.Pix[i+3] = c.R, c.G, c.B, 255
	}

	// Plan9's second color is the closest to another, so it makes way for transparency.
	colors := append(slices.Concat(palette.Plan9[:1], palette.Plan9[2:]), color.Transparent)
	dst := image.NewPaletted(bounds, colors)
	draw.FloydSteinberg.Draw(dst, bounds, opaque, image.Point{})
	for i := range dst.Pix {
		if scaled.Pix[4*i+3] < MinAlpha {
			dst.Pix[i] = transparent
		}
	}
	return dst
}

// Sixel returns the pixels of img within r as a sixel image, which leaves transparent pixels undrawn.
// See https://vt100.net/docs/vt3xx-gp/chapter14.html.
func Sixel(img *image.Paletted, r image.Rectangle) []byte {
	r = r.Intersect(img.Bounds())
	width := r.Dx()
	// Raster attributes give the size, so the image takes its own cells and no more.
	out := fmt.Appendf(nil, "\x1bP0;1;0q\"1;1;%d;%d", width, r.Dy())
	if r.Empty() {
		return append(out, "\x1b\\"...)
	}

	var used [transparent]bool
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for _, index := range img.Pix[img.PixOffset(r.Min.X, y):img.PixOffset(r.Max.X, y)] {
			if index != transparent {
				used[index] = true
			}
		}
	}
	for index, ok := range used {
		if ok {
			c := color.NRGBAModel.Convert(img.Palette[index]).(color.NRGBA)
			out = fmt.Appendf(out, "#%d;2;%d;%d;%d", index, percent(c.R), percent(c.G), percent(c.B))
		}
	}

	// Each band is six rows; each color in it is a row of sixels, whose bits are the pixels of that color from the top.
	var bits [transparent][]byte
	for top := r.Min.Y; top < r.Max.Y; top += 6 {
		// A graphics newline only between bands, as one after the last could scroll the screen.
		if top > r.Min.Y {
			out = append(out, '-')
		}
		var inBand []int
		for y := top; y < min(top+6, r.Max.Y); y++ {
			for x := range width {
				index := img.Pix[img.PixOffset(r.Min.X+x, y)]
				if index == transparent {
					continue
				}
				if bits[index] == nil {
					bits[index] = make([]byte, width)
					inBand = append(inBand, int(index))
				}
				bits[index][x] |= 1 << (y - top)
			}
		}
		slices.Sort(inBand)
		for i, index := range inBand {
			if i > 0 {
				out = append(out, '$')
			}
			out = fmt.Appendf(out, "#%d", index)
			out = appendRuns(out, bits[index])
			bits[index] = nil
		}
	}
	return append(out, "\x1b\\"...)
}

// appendRuns appends sixels, writing a run of more than three as a repeat.
func appendRuns(out, sixels []byte) []byte {
	for i := 0; i < len(sixels); {
		run := 1
		for i+run < len(sixels) && sixels[i+run] == sixels[i] {
			run++
		}
		char := '?' + sixels[i]
		if run > 3 {
			out = fmt.Appendf(out, "!%d%c", run, char)
		} else {
			for range run {
				out = append(out, char)
			}
		}
		i += run
	}
	return out
}

// percent returns an 8-bit channel in sixel's 0 to 100.
func percent(channel uint8) int {
	return (int(channel)*100 + 127) / 255
}
