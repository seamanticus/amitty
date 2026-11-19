package inert

import (
	"fmt"
	"testing"

	"github.com/seamanticus/amitty/internal/tui/layout"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
)

type taken struct{}

// taker turns every message into taken.
type taker struct{}

func (taker) Size() (width, height layout.Length) { return layout.Fill, layout.Fill }

func (taker) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}

func (taker) Draw(tui.Screen, tui.Rectangle) {}

func (taker) Handle(tui.Msg, tui.Rectangle) tui.Msg { return taken{} }

func TestWidgetHandle(t *testing.T) {
	w := New(taker{})
	area := tui.Rectangle{Width: 4, Height: 2}
	// The child gets no input.
	for _, msg := range []tui.Msg{
		tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone),
		tui.MouseMsg{EventMouse: tcell.NewEventMouse(1, 1, tcell.ButtonPrimary, tcell.ModNone), Action: tui.MouseLeftClick},
		tui.PasteMsg("text"),
	} {
		t.Run(fmt.Sprintf("%T", msg), func(t *testing.T) {
			if got := w.Handle(msg, area); got != msg {
				t.Fatalf("got %v", got)
			}
		})
	}
}
