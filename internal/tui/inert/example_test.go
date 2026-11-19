package inert_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/inert"
	"github.com/seamanticus/amitty/internal/tui/text"
)

type sendMsg struct{}

// A button behind a dialog is still drawn, but enter no longer presses it.
func ExampleNew() {
	send := button.New(text.New("Send")).OnClick(sendMsg{}).Focused(true)
	area := tui.Rectangle{Width: 6, Height: 1}
	enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)

	fmt.Printf("%T\n", send.Handle(enter, area))
	fmt.Printf("%T\n", inert.New(send).Handle(enter, area))
	// Output:
	// inert_test.sendMsg
	// *tcell.EventKey
}
