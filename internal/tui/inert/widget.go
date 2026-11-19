// Package inert draws a widget without letting it take input, like HTML's inert attribute, such as for the content behind a dialog.
package inert

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

// Widget draws its child but passes every message by it unchanged.
type Widget struct {
	child tui.Widget
}

var _ tui.Widget = Widget{}

// New wraps child so that it is drawn but takes no input.
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

// Handle returns msg without passing it to the child.
func (Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	return msg
}
