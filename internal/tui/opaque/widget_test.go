package opaque

import (
	"github.com/seamanticus/amitty/internal/tui/layout"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
)

type clicked struct{}

// clicker returns clicked for a left click and passes anything else through.
type clicker struct{}

func (clicker) Draw(tui.Screen, tui.Rectangle) {}

func (clicker) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	if mouse, ok := msg.(tui.MouseMsg); ok && mouse.Action == tui.MouseLeftClick {
		return clicked{}
	}
	return msg
}

func mouse(x, y int, action tui.MouseAction) tui.MouseMsg {
	return tui.MouseMsg{EventMouse: tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone), Action: action}
}

func TestWidgetHandle(t *testing.T) {
	area := tui.Rectangle{Width: 4, Height: 2}
	o := New(clicker{})
	t.Run("drops mouse inside", func(t *testing.T) {
		if got := o.Handle(mouse(1, 1, tui.MouseScrollUp), area); got != nil {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("passes mouse outside", func(t *testing.T) {
		outside := mouse(9, 1, tui.MouseScrollUp)
		if got := o.Handle(outside, area); got != outside {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("keeps child message", func(t *testing.T) {
		if got := o.Handle(mouse(1, 1, tui.MouseLeftClick), area); got != (clicked{}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("passes keys", func(t *testing.T) {
		enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
		if got := o.Handle(enter, area); got != enter {
			t.Fatalf("got %v", got)
		}
	})
}

func (clicker) Size() (width, height layout.Length) { return layout.Fill, layout.Fill }

func (clicker) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}
