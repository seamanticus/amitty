package box_test

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/text"
)

func ExampleNew() {
	b := box.New(text.New("hello")).
		Borders(tui.BordersAll).
		Title(text.New("Inbox")).
		Footer(text.New("1 new")).
		FooterAlignment(tui.HorizontalAlignmentRight)
	screentest.Print(b, 12, 3)
	// Output:
	// ┌───Inbox──┐
	// │hello     │
	// └─────1 new┘
}
