package textarea_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/textarea"
)

type inputMsg struct {
	text   string
	cursor int
}

// A message composer that wraps on words, and reports each edit with OnInput.
func ExampleNew() {
	text := "see you at the station"
	composer := textarea.New(text, len(text)).
		Focused(true).
		OnInput(func(text string, cursor int) tui.Msg { return inputMsg{text, cursor} })
	screentest.Print(composer, 12, 2)

	fmt.Printf("%+v\n", composer.Handle(tcell.NewEventKey(tcell.KeyRune, "!", tcell.ModNone), tui.Rectangle{Width: 12, Height: 2}))
	// Output:
	// see you at
	// the station
	// {text:see you at the station! cursor:23}
}
