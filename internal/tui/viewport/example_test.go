package viewport_test

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/list"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/viewport"
)

// Scrolling one text suits a single block of text, such as a help page. The text wraps all of its lines on every draw, so long text that changes often is better split into a list of texts.
func ExampleNew_text() {
	type model struct{ scrollState viewport.ScrollState }
	view := func(m model) viewport.Widget {
		return viewport.New(text.New("one\ntwo\nthree\nfour"), m.scrollState).
			Focused(true).
			OnChange(func(s *viewport.ScrollState) tui.Msg { return *s })
	}

	area := tui.Rectangle{Width: 5, Height: 2}
	var m model
	// The model stores the state the viewport produces, as Update would.
	if s, ok := view(m).Handle(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone), area).(viewport.ScrollState); ok {
		m.scrollState = s
	}
	screentest.Print(view(m), area.Width, area.Height)
	// Output:
	// two
	// three
}

// A list of texts suits blocks of text that are each their own item, such as chat messages. Each can be selected and styled on its own, and the viewport keeps the one at the cursor in view.
func ExampleNew_list() {
	messages := []string{"hi", "hello", "how are you?", "good"}
	type model struct {
		cursor      int
		scrollState viewport.ScrollState
	}
	view := func(m model) viewport.Widget {
		l := list.New(m.cursor, len(messages), func(i int) tui.Widget {
			return text.New(messages[i])
		})
		return viewport.New(l, m.scrollState).
			Target(l.Target(m.cursor)).
			OnChange(func(s *viewport.ScrollState) tui.Msg { return *s })
	}

	area := tui.Rectangle{Width: 6, Height: 2}
	m := model{cursor: 3}
	m.scrollState.ScrollToTarget()
	screentest.Print(view(m), area.Width, area.Height)
	// Output:
	// you?
	// good
}
