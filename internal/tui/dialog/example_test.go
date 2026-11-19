package dialog_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/dialog"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
)

type doneMsg struct {
	index int
	label string
}

// A confirmation whose OnDone message tells Update which button was pressed.
func ExampleNew() {
	confirm := dialog.New().
		Text("Leave the server?").
		Buttons("Leave", "Cancel").
		Focus(1).
		OnDone(func(index int, label string) tui.Msg { return doneMsg{index, label} })
	screentest.Print(confirm, 26, 7)

	enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	fmt.Printf("%+v\n", confirm.Handle(enter, tui.Rectangle{Width: 26, Height: 7}))
	// Output:
	// ┌────────────────────────┐
	// │                        │
	// │   Leave the server?    │
	// │                        │
	// │   Leave      Cancel    │
	// │                        │
	// └────────────────────────┘
	// {index:1 label:Cancel}
}
