package scrollbar_test

import (
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/scrollbar"
)

// A scroll bar over 20 rows of content, 5 of which are in view after scrolling 10 rows down.
func ExampleNew() {
	screentest.Print(scrollbar.New().Lengths(20, 5).Offset(10), 1, 6)
	// Output:
	// ▲
	// ║
	// ║
	// █
	// ║
	// ▼
}
