package textinput_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/textinput"
)

type inputMsg struct {
	text   string
	cursor int
}

type submitMsg struct{}

// The model owns the text and the cursor, and the input reports the edits a key makes with OnInput.
func ExampleNew() {
	input := textinput.New("hunter", 6).
		Mask("*").
		Focused(true).
		OnInput(func(text string, cursor int) tui.Msg { return inputMsg{text, cursor} }).
		OnSubmit(submitMsg{})
	screentest.Print(input, 10, 1)

	area := tui.Rectangle{Width: 10, Height: 1}
	fmt.Printf("%+v\n", input.Handle(tcell.NewEventKey(tcell.KeyRune, "2", tcell.ModNone), area))
	fmt.Printf("%T\n", input.Handle(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone), area))
	// Output:
	// ******
	// {text:hunter2 cursor:7}
	// textinput_test.submitMsg
}
