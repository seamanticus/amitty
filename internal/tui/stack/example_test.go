package stack_test

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/center"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/stack"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// A popup drawn over the content behind it.
func ExampleNew() {
	content := text.New("first line of content\nsecond line of content\nthird line of content")
	popup := center.New(box.New(text.New("Saved")).Borders(tui.BordersAll).Width(layout.Shrink).Height(layout.Shrink))
	screentest.Print(stack.New(content, popup), 22, 3)
	// Output:
	// first l┌─────┐content
	// second │Saved│ content
	// third l└─────┘content
}
