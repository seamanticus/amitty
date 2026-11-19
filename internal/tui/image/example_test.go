package image_test

import (
	stdimage "image"
	"image/color"

	"github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
)

// Without kitty's graphics protocol, each cell shows two pixels stacked with a half block.
func ExampleNew() {
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 4, 2))
	for x := range 4 {
		img.Set(x, 0, color.RGBA{R: 255, A: 255})
		img.Set(x, 1, color.RGBA{B: 255, A: 255})
	}
	screentest.Print(image.New(img), 4, 1)
	// Output:
	// ▀▀▀▀
}
