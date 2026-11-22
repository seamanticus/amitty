// Package box draws a border with a title and footer around a widget.
package box

import (
	"slices"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

// Widget draws a background, border, title, and footer, with its child inside them.
type Widget struct {
	child         tui.Widget
	width, height layout.Length

	background                      tcell.Color
	borders                         tui.Borders
	borderSet                       tui.BorderSet
	borderStyle                     tui.Style
	paddingTop, paddingBottom       int
	paddingLeft, paddingRight       int
	title, footer                   tui.Widget
	titleAlignment, footerAlignment tui.HorizontalAlignment
	// labelInset is the border cells kept between a corner and the title or footer.
	labelInset int
}

var _ tui.Widget = Widget{}

// New returns a box around child, without a border, that fills its parent.
func New(child tui.Widget) Widget {
	return Widget{
		child:           child,
		width:           layout.Fill,
		height:          layout.Fill,
		borderSet:       tui.BorderSetPlain(),
		titleAlignment:  tui.HorizontalAlignmentCenter,
		footerAlignment: tui.HorizontalAlignmentCenter,
	}
}

// Width sets the width of the box.
func (w Widget) Width(width layout.Length) Widget {
	w.width = width
	return w
}

// Height sets the height of the box.
func (w Widget) Height(height layout.Length) Widget {
	w.height = height
	return w
}

// Background sets the background color.
func (w Widget) Background(color tcell.Color) Widget {
	w.background = color
	return w
}

// Borders sets which sides have a border.
func (w Widget) Borders(borders tui.Borders) Widget {
	w.borders = borders
	return w
}

// BorderSet sets the characters the border is drawn with.
func (w Widget) BorderSet(set tui.BorderSet) Widget {
	w.borderSet = set
	return w
}

// BorderStyle sets the style of the border.
func (w Widget) BorderStyle(style tui.Style) Widget {
	w.borderStyle = style
	return w
}

// Padding sets the empty cells between the border and the child.
func (w Widget) Padding(top, bottom, left, right int) Widget {
	w.paddingTop, w.paddingBottom, w.paddingLeft, w.paddingRight = top, bottom, left, right
	return w
}

// Title sets the widget drawn in the top border, one row tall and as wide as its content, such as text or tabs, or nil for none.
func (w Widget) Title(title tui.Widget) Widget {
	w.title = title
	return w
}

// LabelInset sets the border cells kept between each corner and the title or footer, while the box is wide enough to show them.
func (w Widget) LabelInset(cells int) Widget {
	w.labelInset = max(cells, 0)
	return w
}

// TitleAlignment sets the alignment of the title.
func (w Widget) TitleAlignment(align tui.HorizontalAlignment) Widget {
	w.titleAlignment = align
	return w
}

// Footer sets the widget drawn in the bottom border, one row tall and as wide as its content, or nil for none.
func (w Widget) Footer(footer tui.Widget) Widget {
	w.footer = footer
	return w
}

// FooterAlignment sets the alignment of the footer.
func (w Widget) FooterAlignment(align tui.HorizontalAlignment) Widget {
	w.footerAlignment = align
	return w
}

// Size returns the width and height of the box.
func (w Widget) Size() (width, height layout.Length) {
	return w.width, w.height
}

// Layout returns the size of the box within limits: that of its child inside the frame, plus the frame.
func (w Widget) Layout(limits layout.Limits) layout.Size {
	limits = limits.Width(w.width).Height(w.height)
	inner := w.InnerArea(tui.Rectangle{Width: layout.Infinity, Height: layout.Infinity})
	frame := layout.Size{Width: layout.Infinity - inner.Width, Height: layout.Infinity - inner.Height}
	content := w.child.Layout(limits.Shrink(frame).Loose())
	size := limits.Shrink(frame).Resolve(w.width, w.height, content)
	return layout.Size{Width: size.Width + frame.Width, Height: size.Height + frame.Height}
}

// InnerArea returns the part of area inside the border and padding, where the child is drawn.
func (w Widget) InnerArea(area tui.Rectangle) tui.Rectangle {
	if w.title != nil || w.borders.Has(tui.BordersTop) {
		area.Y++
		area.Height--
	}
	if w.footer != nil || w.borders.Has(tui.BordersBottom) {
		area.Height--
	}
	if w.borders.Has(tui.BordersLeft) {
		area.X++
		area.Width--
	}
	if w.borders.Has(tui.BordersRight) {
		area.Width--
	}
	area.X += w.paddingLeft
	area.Y += w.paddingTop
	area.Width = max(area.Width-w.paddingLeft-w.paddingRight, 0)
	area.Height = max(area.Height-w.paddingTop-w.paddingBottom, 0)
	return area
}

// Draw draws the background, border, title, and footer in area and the child inside them.
func (w Widget) Draw(screen tui.Screen, area tui.Rectangle) {
	if area.Width <= 0 || area.Height <= 0 {
		return
	}
	left, top := area.X, area.Y
	right, bottom := area.X+area.Width-1, area.Y+area.Height-1

	// A cell keeps what it holds from the last frame unless it changes, which spares the terminal screen measuring it again, so cells are not filled only to be drawn over.
	labels := w.labels(area)
	fill := func(x0, y0, x1, y1 int, str string, style tui.Style) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if !isUnderLabel(labels, x, y) {
					screen.Put(x, y, str, style)
				}
			}
		}
	}

	inset := func(side tui.Borders) int {
		if w.borders.Has(side) {
			return 1
		}
		return 0
	}
	x0, x1 := left+inset(tui.BordersLeft), right-inset(tui.BordersRight)
	y0, y1 := top+inset(tui.BordersTop), bottom-inset(tui.BordersBottom)
	background := tcell.StyleDefault.Background(w.background)
	screen.FillArea(x0, y0, x1-x0+1, y1-y0+1, ' ', background)

	// A box one row tall or one column wide still draws its borders, so a top border alone makes a line, its title on it.
	if w.borders != tui.BordersNone {
		set := w.borderSet
		// A side reaches into a corner unless the side it meets there is drawn too, which makes the corner.
		for _, part := range []struct {
			sides          tui.Borders
			x0, y0, x1, y1 int
			glyph          string
		}{
			{tui.BordersTop, x0, top, x1, top, set.Top},
			{tui.BordersBottom, x0, bottom, x1, bottom, set.Bottom},
			{tui.BordersLeft, left, y0, left, y1, set.Left},
			{tui.BordersRight, right, y0, right, y1, set.Right},
			{tui.BordersTop | tui.BordersLeft, left, top, left, top, set.TopLeft},
			{tui.BordersTop | tui.BordersRight, right, top, right, top, set.TopRight},
			{tui.BordersBottom | tui.BordersLeft, left, bottom, left, bottom, set.BottomLeft},
			{tui.BordersBottom | tui.BordersRight, right, bottom, right, bottom, set.BottomRight},
		} {
			if w.borders&part.sides == part.sides {
				fill(part.x0, part.y0, part.x1, part.y1, part.glyph, w.borderStyle)
			}
		}
	}

	for _, l := range labels {
		// A label merges with the cells beneath it, so they take the box's background, keeping what they hold so it is not measured again.
		for x := l.area.X; x < l.area.X+l.area.Width; x++ {
			str, _, _ := screen.Get(x, l.area.Y)
			screen.Put(x, l.area.Y, str, background)
		}

		l.widget.Draw(screen, l.area)
	}
	w.child.Draw(screen, w.InnerArea(area))
}

// label is a title or footer and where it is drawn in the border.
type label struct {
	widget tui.Widget
	area   tui.Rectangle
}

// isUnderLabel reports whether the cell at x, y is under one of labels.
func isUnderLabel(labels []label, x, y int) bool {
	return slices.ContainsFunc(labels, func(l label) bool { return l.area.Contains(x, y) })
}

// labels returns where the title and footer are drawn on the top and bottom rows of area, inside the corners, as wide as their content and placed by their alignments.
func (w Widget) labels(area tui.Rectangle) []label {
	if area.Width < 4 {
		return nil
	}

	var labels []label
	for _, l := range []struct {
		widget tui.Widget
		y      int
		align  tui.HorizontalAlignment
	}{
		{w.title, area.Y, w.titleAlignment},
		{w.footer, area.Y + area.Height - 1, w.footerAlignment},
	} {
		if l.widget == nil {
			continue
		}

		// The inset gives way before the label is cut, from both sides so a centered label stays centered.
		natural := l.widget.Layout(layout.Limits{Max: layout.Size{Width: area.Width - 2, Height: 1}}).Width
		inset := min(w.labelInset, max(area.Width-2-natural, 0)/2)
		room := area.Width - 2 - 2*inset
		width := min(natural, room)
		x := area.X + 1 + inset
		switch l.align {
		case tui.HorizontalAlignmentLeft:
		case tui.HorizontalAlignmentCenter:
			x += room/2 - width/2
		case tui.HorizontalAlignmentRight:
			x += room - width
		}

		labels = append(labels, label{widget: l.widget, area: tui.Rectangle{X: x, Y: l.y, Width: width, Height: 1}})
	}

	return labels
}

// Handle passes a mouse message on the title or footer to it, and other messages to the child with the area inside the frame.
func (w Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	if mouse, ok := msg.(tui.MouseMsg); ok {
		for _, l := range w.labels(area) {
			if l.area.Contains(mouse.Position()) {
				return l.widget.Handle(msg, l.area)
			}
		}
	}

	return w.child.Handle(msg, w.InnerArea(area))
}
