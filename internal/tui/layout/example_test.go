package layout_test

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// A sidebar of fixed width beside content that takes the rest, and a preview that takes half as much as the content.
func ExampleFillPortion() {
	panes := row.New(
		box.New(text.New("nav")).Borders(tui.BordersAll).Width(layout.Fixed(5)),
		box.New(text.New("body")).Borders(tui.BordersAll).Width(layout.FillPortion(2)),
		box.New(text.New("info")).Borders(tui.BordersAll).Width(layout.FillPortion(1)),
	)
	screentest.Print(panes, 23, 3)
	// Output:
	// ┌───┐┌──────────┐┌────┐
	// │nav││body      ││info│
	// └───┘└──────────┘└────┘
}
