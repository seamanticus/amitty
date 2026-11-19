// Package center places a widget in the middle of its area.
package center

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

// Widget places its child in the middle of its area.
type Widget struct {
	child tui.Widget
}

var _ tui.Widget = Widget{}

// New places child in the middle of its area at the size of its Layout. A child that fills along an axis takes the whole area along it.
func New(child tui.Widget) Widget {
	return Widget{child: child}
}

// Size returns Fill, as the center takes its whole area.
func (Widget) Size() (width, height layout.Length) {
	return layout.Fill, layout.Fill
}

// Layout returns the size of limits, as the center takes its whole area.
func (Widget) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}

// Draw draws the child in the middle of area.
func (w Widget) Draw(screen tui.Screen, area tui.Rectangle) {
	w.child.Draw(screen, w.area(area))
}

// Handle passes msg to the child with the child's area.
func (w Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	return w.child.Handle(msg, w.area(area))
}

func (w Widget) area(area tui.Rectangle) tui.Rectangle {
	size := w.child.Layout(layout.Limits{Max: layout.Size{Width: area.Width, Height: area.Height}})
	area.X += (area.Width - size.Width) / 2
	area.Y += (area.Height - size.Height) / 2
	area.Width, area.Height = size.Width, size.Height
	return area
}
