// Package backdrop restyles what is drawn behind it, typically to dim the content behind a dialog.
package backdrop

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

// Widget merges its style into everything already drawn within its area and hides the cursor.
type Widget struct {
	style tui.Style
}

var _ tui.Widget = Widget{}

// New returns a backdrop that dims what is behind it.
func New() Widget {
	return Widget{style: tcell.StyleDefault.Dim(true)}
}

// Style sets the style merged into the cells behind the backdrop with tui.MergeStyle.
func (w Widget) Style(style tui.Style) Widget {
	w.style = style
	return w
}

// Size returns Fill, as the backdrop takes its whole area.
func (Widget) Size() (width, height layout.Length) {
	return layout.Fill, layout.Fill
}

// Layout returns the size of limits, as the backdrop takes its whole area.
func (Widget) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}

// Draw restyles the cells already drawn within area.
func (w Widget) Draw(screen tui.Screen, area tui.Rectangle) {
	screen.HideCursor()
	for y := area.Y; y < area.Y+area.Height; y++ {
		for x := area.X; x < area.X+area.Width; x++ {
			str, style, width := screen.Get(x, y)
			screen.Put(x, y, str, tui.MergeStyle(style, w.style))
			// Wide characters occupy the following cells too.
			x += max(width, 1) - 1
		}
	}
}

// Handle passes msg through unchanged.
func (Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	return msg
}
