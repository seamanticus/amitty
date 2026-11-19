// Package opaque keeps mouse messages from passing through a widget to the ones stacked below it.
package opaque

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

// Widget drops the mouse messages within its area that its child does not turn into another message.
type Widget struct {
	child tui.Widget
}

var _ tui.Widget = Widget{}

// New wraps child so that mouse messages within its area do not reach the children of a stack below it.
func New(child tui.Widget) Widget {
	return Widget{child: child}
}

// Size returns the size of the child.
func (w Widget) Size() (width, height layout.Length) {
	return w.child.Size()
}

// Layout lays out the child.
func (w Widget) Layout(limits layout.Limits) layout.Size {
	return w.child.Layout(limits)
}

// Draw draws the child.
func (w Widget) Draw(screen tui.Screen, area tui.Rectangle) {
	w.child.Draw(screen, area)
}

// Handle passes msg to the child and drops it if it is still a mouse message within area.
func (w Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	msg = w.child.Handle(msg, area)
	if mouse, ok := msg.(tui.MouseMsg); ok && area.Contains(mouse.Position()) {
		return nil
	}
	return msg
}
