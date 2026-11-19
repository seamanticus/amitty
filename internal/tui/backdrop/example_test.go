package backdrop_test

import (
	"github.com/seamanticus/amitty/internal/tui/backdrop"
	"github.com/seamanticus/amitty/internal/tui/dialog"
	"github.com/seamanticus/amitty/internal/tui/inert"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/stack"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// A modal dialog: the content behind it is dimmed by the backdrop and takes no input, being inert.
func ExampleNew() {
	content := text.New("draft: hello world")
	modal := stack.New(
		inert.New(content),
		backdrop.New(),
		dialog.New().Text("Discard draft?").Buttons("Discard", "Keep"),
	)
	screentest.Print(modal, 30, 11)
	// Output:
	// draft: hello world
	//
	// ┌────────────────────────────┐
	// │                            │
	// │       Discard draft?       │
	// │                            │
	// │     Discard      Keep      │
	// │                            │
	// └────────────────────────────┘
}
