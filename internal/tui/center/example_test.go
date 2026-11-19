package center_test

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/center"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// A box that shrinks to fit its text, in the middle of a window.
func ExampleNew() {
	b := box.New(text.New("loading")).Borders(tui.BordersAll).Width(layout.Shrink).Height(layout.Shrink)
	window := box.New(center.New(b)).Borders(tui.BordersAll)
	screentest.Print(window, 15, 7)
	// Output:
	// ┌─────────────┐
	// │             │
	// │  ┌───────┐  │
	// │  │loading│  │
	// │  └───────┘  │
	// │             │
	// └─────────────┘
}
