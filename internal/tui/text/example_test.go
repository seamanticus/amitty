package text_test

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// Text wraps between words to the width it is drawn in.
func ExampleNew() {
	screentest.Print(text.New("a line too long to fit"), 12, 2)
	// Output:
	// a line too
	// long to fit
}

// Rich text has spans in styles of their own.
func ExampleRich() {
	screentest.Print(text.Rich(
		text.Span{Text: "Note: ", Style: tcell.StyleDefault.Bold(true)},
		text.Span{Text: "changes take effect after a restart."},
	), 16, 3)
	// Output:
	// Note: changes
	// take effect
	// after a restart.
}
