package row_test

import (
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// A status bar with a label at the start and a button that as wide as its label and padding.
func ExampleNew() {
	bar := row.New(
		text.New("3 unread"),
		button.New(text.New("Mark read")).Width(layout.Fixed(11)),
	).Spacing(1)
	screentest.Print(bar, 24, 1)
	// Output:
	// 3 unread  Mark read
}
