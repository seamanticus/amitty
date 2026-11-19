package column_test

import (
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
)

// A form of labeled inputs, one under another with a row between them.
func ExampleNew() {
	form := column.New(
		text.New("Email"),
		textinput.New("me@example.com", 0),
		text.New("Password"),
		textinput.New("hunter2", 0).Mask("*"),
	).Spacing(1)
	screentest.Print(form, 16, 7)
	// Output:
	// Email
	//
	// me@example.com
	//
	// Password
	//
	// *******
}
