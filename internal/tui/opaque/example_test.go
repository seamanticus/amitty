package opaque_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/opaque"
	"github.com/seamanticus/amitty/internal/tui/stack"
	"github.com/seamanticus/amitty/internal/tui/text"
)

type deleteMsg struct{}

// A click on a panel stacked over a button would reach the button below it, unless the panel is opaque.
func ExampleNew() {
	del := button.New(text.New("Delete")).OnClick(deleteMsg{})
	panel := box.New(text.New("notes")).Borders(tui.BordersAll)
	area := tui.Rectangle{Width: 10, Height: 3}
	click := tui.MouseMsg{EventMouse: tcell.NewEventMouse(4, 1, tcell.ButtonPrimary, tcell.ModNone), Action: tui.MouseLeftDown}

	fmt.Printf("%T\n", stack.New(del, panel).Handle(click, area))
	fmt.Printf("%v\n", stack.New(del, opaque.New(panel)).Handle(click, area))
	// Output:
	// opaque_test.deleteMsg
	// <nil>
}
