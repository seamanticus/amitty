package list_test

import (
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/list"
	"github.com/seamanticus/amitty/internal/tui/text"
)

type cursorMsg int

// A list builds each item from its index, and reports where a key moves the cursor with OnChange.
func ExampleNew() {
	servers := []string{"Gophers", "Rustaceans", "Zig"}
	l := list.New(0, len(servers), func(i int) tui.Widget { return text.New(servers[i]) }).
		SelectedStyle(tcell.StyleDefault.Reverse(true)).
		Spacing(func(int) int { return 1 }).
		Focused(true).
		OnChange(func(cursor int) tui.Msg { return cursorMsg(cursor) })
	screentest.Print(l, 12, 5)

	fmt.Println(l.Handle(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone), tui.Rectangle{Width: 12, Height: 5}))
	// Output:
	// Gophers
	//
	// Rustaceans
	//
	// Zig
	// 1
}
