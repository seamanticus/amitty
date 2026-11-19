package tui

import (
	"fmt"
	"image"
	"slices"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/internal/graphics"
)

// sixelImage is an image that sixel placeholders show, at the size it is drawn.
type sixelImage struct {
	pixels *image.Paletted
	cell   image.Point
}

// placement is a rectangle of the screen that shows a crop of an image, in cells of the image.
type placement struct {
	id     int
	screen Rectangle
	crop   image.Rectangle
}

// cropKey names the sixels of a crop, which stay the same wherever it is placed.
type cropKey struct {
	id   int
	crop image.Rectangle
}

// sixels writes the images of sixel placeholders over them. The terminal keeps a sixel until a cell drawn on top erases it, so the cells under each shown placement are locked against tcell, and unlocked when the placement goes.
type sixels struct {
	images map[int]sixelImage
	shown  []placement
	// encoded holds the sixels of the crops shown, so a scroll moves them without encoding again.
	encoded map[cropKey][]byte
}

func newSixels() sixels {
	return sixels{images: make(map[int]sixelImage), encoded: make(map[cropKey][]byte)}
}

// set adds, replaces or, for a nil image, forgets the image of id. Its shown placements are unlocked, so the next frame writes or erases them.
func (s *sixels) set(screen Screen, c cmd.SixelImage) {
	for key := range s.encoded {
		if key.id == c.ID {
			delete(s.encoded, key)
		}
	}
	s.shown = slices.DeleteFunc(s.shown, func(p placement) bool {
		if p.id != c.ID {
			return false
		}
		screen.LockRegion(p.screen.X, p.screen.Y, p.screen.Width, p.screen.Height, false)
		return true
	})
	if c.Image == nil {
		delete(s.images, c.ID)
		return
	}
	s.images[c.ID] = sixelImage{pixels: c.Image, cell: c.Cell}
}

// forget unlocks every shown placement, for a terminal that lost them, as after a resize or a suspend.
func (s *sixels) forget(screen Screen) {
	for _, p := range s.shown {
		screen.LockRegion(p.screen.X, p.screen.Y, p.screen.Width, p.screen.Height, false)
	}
	s.shown = nil
}

// show replaces the sixel placeholders on screen with blanks and shows the frame, then writes the placements not shown before.
// A placement shown before stays locked, so its sixels stay; one that went is unlocked, so tcell draws over it.
func (s *sixels) show(screen Screen) {
	placements := s.placements(screen)
	for _, p := range s.shown {
		if !slices.Contains(placements, p) {
			screen.LockRegion(p.screen.X, p.screen.Y, p.screen.Width, p.screen.Height, false)
		}
	}
	var added []placement
	for _, p := range placements {
		if !slices.Contains(s.shown, p) {
			added = append(added, p)
		}
	}
	// The blanks under added placements are drawn first, so no earlier text shows through their transparent pixels.
	screen.Show()
	s.keep(placements)

	tty, ok := screen.Tty()
	if !ok || len(added) == 0 {
		return
	}
	// Synchronized, with the cursor saved and restored around it, and without scrolling when an image ends at the bottom row.
	out := []byte("\x1b[?2026h\x1b7\x1b[?8452h")
	for _, p := range added {
		out = fmt.Appendf(out, "\x1b[%d;%dH", p.screen.Y+1, p.screen.X+1)
		out = append(out, s.sixel(p)...)
		screen.LockRegion(p.screen.X, p.screen.Y, p.screen.Width, p.screen.Height, true)
	}
	out = append(out, "\x1b8\x1b[?2026l"...)
	_, _ = tty.Write(out)
}

// keep records placements as shown and drops the sixels of crops no longer shown.
func (s *sixels) keep(placements []placement) {
	s.shown = placements
	live := make(map[cropKey]bool, len(placements))
	for _, p := range placements {
		live[cropKey{p.id, p.crop}] = true
	}
	for key := range s.encoded {
		if !live[key] {
			delete(s.encoded, key)
		}
	}
}

func (s *sixels) sixel(p placement) []byte {
	key := cropKey{p.id, p.crop}
	if data, ok := s.encoded[key]; ok {
		return data
	}
	img := s.images[p.id]
	pixels := image.Rect(p.crop.Min.X*img.cell.X, p.crop.Min.Y*img.cell.Y, p.crop.Max.X*img.cell.X, p.crop.Max.Y*img.cell.Y)
	data := graphics.Sixel(img.pixels, pixels)
	s.encoded[key] = data
	return data
}

// placements blanks the sixel placeholders on screen and returns the rectangles they cover, in reading order, for images that are set.
// Cells covered by something drawn later are gone, so a partly covered image becomes several rectangles.
func (s *sixels) placements(screen Screen) []placement {
	type origin struct {
		id   int
		x, y int
	}
	// runs holds, for each image and where its top-left cell would be, the runs of the row before with them.
	type run struct {
		start, end, row int
	}
	var placements []placement
	open := make(map[origin][]int)

	width, height := screen.Size()
	for y := range height {
		var row []origin
		var runs []run
		for x := range width {
			str, style, _ := screen.Get(x, y)
			if !graphics.IsSixelCell(str) {
				continue
			}
			screen.Put(x, y, " ", style.Foreground(tcell.ColorDefault))
			id, imageRow, imageCol, ok := graphics.ParseSixelCell(str, style.GetForeground())
			if _, set := s.images[id]; !ok || !set {
				continue
			}
			o := origin{id, x - imageCol, y - imageRow}
			if n := len(row); n > 0 && row[n-1] == o && runs[n-1].end == x {
				runs[n-1].end++
				continue
			}
			row = append(row, o)
			runs = append(runs, run{x, x + 1, imageRow})
		}

		// A run continues the rectangle above it with the same columns, or starts one.
		next := make(map[origin][]int)
		for i, o := range row {
			r := runs[i]
			extended := false
			for _, index := range open[o] {
				p := &placements[index]
				if p.screen.X == r.start && p.screen.X+p.screen.Width == r.end && p.screen.Y+p.screen.Height == y {
					p.screen.Height++
					p.crop.Max.Y++
					next[o] = append(next[o], index)
					extended = true
					break
				}
			}
			if !extended {
				col := r.start - o.x
				next[o] = append(next[o], len(placements))
				placements = append(placements, placement{
					id:     o.id,
					screen: Rectangle{X: r.start, Y: y, Width: r.end - r.start, Height: 1},
					crop:   image.Rect(col, r.row, col+r.end-r.start, r.row+1),
				})
			}
		}
		open = next
	}
	return placements
}
