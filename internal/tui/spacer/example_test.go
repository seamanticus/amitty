package spacer_test

import (
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/spacer"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// A spacer that fills the width pushes the buttons after it to the right edge, away from the title.
func ExampleNew() {
	buttons := row.New(
		text.New("Draft"),
		spacer.New().Width(layout.Fill),
		button.New(text.New("Cancel")).Width(layout.Fixed(8)),
		button.New(text.New("OK")).Width(layout.Fixed(4)),
	).Spacing(1)
	screentest.Print(buttons, 24, 1)
	// Output:
	// Draft       Cancel   OK
}
