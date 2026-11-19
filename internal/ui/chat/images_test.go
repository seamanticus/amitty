package chat

import (
	"bytes"
	"compress/zlib"
	"image"
	"io"
	"testing"
)

func TestPreviewCols(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		width, height         uint
		cellWidth, cellHeight uint
		want                  int
	}{
		{"landscape", 1600, 900, 1, 2, 40},
		{"portrait", 900, 1600, 1, 2, 22},
		{"small", 10, 10, 1, 2, 10},
		{"very tall", 1, 10000, 1, 2, 1},
		{"own width in cells", 200, 100, 10, 20, 20},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := previewCols(tt.width, tt.height, tt.cellWidth, tt.cellHeight); got != tt.want {
				t.Fatalf("previewCols() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSquare(t *testing.T) {
	t.Run("square", func(t *testing.T) {
		img := image.NewAlpha(image.Rect(0, 0, 20, 20))
		if got := square(img); got != image.Image(img) {
			t.Fatal("square image was copied")
		}
	})
	t.Run("tall", func(t *testing.T) {
		img := image.NewAlpha(image.Rect(0, 0, 10, 30))
		img.Pix[0] = 0xff
		got := square(img)
		if got.Bounds() != image.Rect(0, 0, 30, 30) {
			t.Fatalf("bounds = %v, want 30 by 30", got.Bounds())
		}
		if _, _, _, a := got.At(10, 0).RGBA(); a == 0 {
			t.Fatal("image is not centered")
		}
	})
}

func TestNewPooledCompressor(t *testing.T) {
	want := []byte("pixels")
	// The second round reuses the pooled writer.
	for _, round := range []string{"new", "pooled"} {
		t.Run(round, func(t *testing.T) {
			var buf bytes.Buffer
			compressor, err := newPooledCompressor(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := compressor.Write(want); err != nil {
				t.Fatal(err)
			}
			if err := compressor.Close(); err != nil {
				t.Fatal(err)
			}

			reader, err := zlib.NewReader(&buf)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("got %q, %v, want %q", got, err, want)
			}
		})
	}
}

func TestShape(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	for i := range src.Pix {
		src.Pix[i] = 0xff
	}
	alpha := func(img image.Image, x, y int) uint32 {
		_, _, _, a := img.At(x, y).RGBA()
		return a >> 8
	}

	// evidence: witnesses
	for _, test := range []struct {
		shape                string
		corner, edge, centre uint32
	}{
		{"square", 0xff, 0xff, 0xff},
		{"", 0xff, 0xff, 0xff},
		{"rounded", 0, 0xff, 0xff},
		{"circle", 0, 0xff, 0xff},
	} {
		t.Run(test.shape, func(t *testing.T) {
			img := shape(src, test.shape)
			if got := alpha(img, 0, 0); got != test.corner {
				t.Fatalf("corner alpha = %d, want %d", got, test.corner)
			}
			// The middle of the top edge stays nearly opaque, as its pixel only touches a circle's arc.
			if got := alpha(img, 10, 0); got < test.edge-8 {
				t.Fatalf("edge alpha = %d, want about %d", got, test.edge)
			}
			if got := alpha(img, 10, 10); got != test.centre {
				t.Fatalf("centre alpha = %d, want %d", got, test.centre)
			}
		})
	}
	t.Run("a circle cuts more than a rounded square", func(t *testing.T) {
		if rounded, circle := alpha(shape(src, "rounded"), 2, 2), alpha(shape(src, "circle"), 2, 2); rounded <= circle {
			t.Fatalf("alpha at 2, 2: rounded %d, circle %d", rounded, circle)
		}
	})
	t.Run("an empty image", func(t *testing.T) {
		if !shape(&image.Alpha{}, "circle").Bounds().Empty() {
			t.Fatal("not empty")
		}
	})
}
