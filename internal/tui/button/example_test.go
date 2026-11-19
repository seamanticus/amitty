package button_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/text"
)

type saveMsg struct{}

// A focused button turns enter into its OnClick message, which Update receives.
func ExampleNew() {
	save := button.New(text.New("Save")).OnClick(saveMsg{}).Focused(true)
	screentest.Print(save, 8, 1)

	enter := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	msg := save.Handle(enter, tui.Rectangle{Width: 8, Height: 1})
	fmt.Printf("%T\n", msg)
	// Output:
	// Save
	// button_test.saveMsg
}
