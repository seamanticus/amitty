package image

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/internal/graphics"
)

// chunkSize is the most encoded data kitty takes in one escape code, which encodes rawChunkSize bytes.
const (
	chunkSize    = 4096
	rawChunkSize = chunkSize / 4 * 3
)

// headerSize is room for the keys and numbers of a transmission's first escape code.
const headerSize = 80

// MaxID is the highest id an image drawn with placeholders can have.
const MaxID = graphics.MaxID

// Kitty draws the image with kitty's graphics protocol as image n, from 1 to MaxID.
// The image must be sent to the terminal with Transmit.
func (w Widget) Kitty(n int) Widget {
	w.id, w.sixel = n, false
	return w
}

// Sixel draws the image as sixels, which the application writes over its placeholders, as image n, from 1 to MaxID.
// The image must be given to the application with Transmit.
func (w Widget) Sixel(n int) Widget {
	w.id, w.sixel = n, true
	return w
}

// Compressor returns a writer to w in kitty's compression format, which is not used after Close.
// See https://sw.kovidgoyal.net/kitty/graphics-protocol/#compression.
type Compressor func(w io.Writer) (io.WriteCloser, error)

// Compressor sets how a kitty image is compressed; it is sent uncompressed without one or if compressing does not shrink it.
func (w Widget) Compressor(compressor Compressor) Widget {
	w.compressor = compressor
	return w
}

// Transmit returns a command that sends the image to the terminal, or to the application for sixels, for Draw to show, or nil if it is drawn with half blocks.
func (w Widget) Transmit() tui.Cmd {
	switch {
	case w.id == 0:
		return nil
	case w.sixel:
		cols, _ := w.cells()
		cellWidth, cellHeight := w.cell()
		b := w.src.Bounds()
		width := cols * cellWidth
		height := width * b.Dy() / max(b.Dx(), 1)
		return cmd.Task(func() tui.Msg {
			return cmd.SixelImage{ID: w.id, Image: graphics.Quantize(w.src, width, height), Cell: image.Pt(cellWidth, cellHeight)}
		})
	}
	return cmd.Task(func() tui.Msg {
		data, err := w.transmission()
		if err != nil {
			return nil
		}
		return cmd.WriteRaw(data)
	})
}

// Delete returns a command that frees the image's data in the terminal or the application, or nil if it is drawn with half blocks.
func (w Widget) Delete() tui.Cmd {
	switch {
	case w.id == 0:
		return nil
	case w.sixel:
		return cmd.SixelImage{ID: w.id}
	}
	return cmd.WriteRaw(fmt.Sprintf("\x1b_Ga=d,d=I,q=2,i=%d\x1b\\", graphics.TerminalID(w.id)))
}

// transmission uses raw pixels, which encode several times faster than PNG.
func (w Widget) transmission() ([]byte, error) {
	bounds := w.src.Bounds()
	if bounds.Empty() {
		return nil, errEmpty
	}

	payload, compression := pixels(w.src), ""
	if w.compressor != nil {
		compressed, err := w.compress(payload)
		if err != nil {
			return nil, err
		}
		if len(compressed) < len(payload) {
			payload, compression = compressed, "o=z,"
		}
	}

	cols, rows := w.cells()
	encodedLen := base64.StdEncoding.EncodedLen(len(payload))

	// Room for the data and the few bytes of escape codes around each of its chunks.
	out := make([]byte, 0, headerSize+encodedLen+encodedLen/64)
	out = fmt.Appendf(out, "\x1b_Ga=T,U=1,f=32,%ss=%d,v=%d,q=2,i=%d,c=%d,r=%d,", compression, bounds.Dx(), bounds.Dy(), graphics.TerminalID(w.id), cols, rows)
	for len(payload) > rawChunkSize {
		out = append(out, "m=1;"...)
		out = base64.StdEncoding.AppendEncode(out, payload[:rawChunkSize])
		out = append(out, "\x1b\\\x1b_G"...)
		payload = payload[rawChunkSize:]
	}

	out = append(out, "m=0;"...)
	out = base64.StdEncoding.AppendEncode(out, payload)
	return append(out, "\x1b\\"...), nil
}

var errEmpty = errors.New("image is empty")

func (w Widget) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := w.compressor(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pixels returns src as rows of non-premultiplied RGBA, the pixel format the transmission declares.
func pixels(src image.Image) []byte {
	bounds := src.Bounds()
	if img, ok := src.(*image.NRGBA); ok && img.Stride == 4*bounds.Dx() {
		return img.Pix[:img.Stride*bounds.Dy()]
	}

	img := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(img, img.Bounds(), src, bounds.Min, draw.Src)
	return img.Pix
}
