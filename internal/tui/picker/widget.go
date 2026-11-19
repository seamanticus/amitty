// Package picker provides a declarative widget that filters a list of items with a query.
package picker

import (
	"github.com/gdamore/tcell/v3"
	"github.com/sahilm/fuzzy"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/list"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/scrollbar"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
	"github.com/seamanticus/amitty/internal/tui/viewport"
)

// bottom border + value
const (
	prompt    = "> "
	separator = "─"
)

const inputHeight = 2

// Widget is a query above the list of items that match it.
type Widget struct {
	items       Items
	searchState *SearchState

	scrollBar           scrollbar.Widget
	scrollBarVisibility viewport.ScrollBarVisibility

	keybind     func(tui.KeyMsg) Action
	listKeybind func(tui.KeyMsg) list.Action

	onChange func(*SearchState) tui.Msg
	onSelect func(Item) tui.Msg
	onCancel tui.Msg
}

var _ tui.Widget = Widget{}

// New returns a picker of items, with searchState as its query, the matches, and the selection.
func New(items Items, searchState *SearchState) Widget {
	return Widget{items: items, searchState: searchState, keybind: DefaultKeybind, listKeybind: list.DefaultKeybind, scrollBar: scrollbar.New(), scrollBarVisibility: viewport.ScrollBarVisibilityAutomatic}
}

// Keybind sets the function that turns keys into Actions, or ActionNone for keys it does not bind, DefaultKeybind unless set.
func (w Widget) Keybind(f func(tui.KeyMsg) Action) Widget {
	w.keybind = f
	return w
}

// ListKeybind sets the function that turns keys into the list's Actions, list.DefaultKeybind unless set.
func (w Widget) ListKeybind(f func(tui.KeyMsg) list.Action) Widget {
	w.listKeybind = f
	return w
}

// ScrollBar sets the list's scroll bar and when it is shown.
func (w Widget) ScrollBar(scrollBar scrollbar.Widget, visibility viewport.ScrollBarVisibility) Widget {
	w.scrollBar, w.scrollBarVisibility = scrollBar, visibility
	return w
}

// OnChange sets the function that turns the search state that typing, moving the selection or scrolling leaves into a message.
func (w Widget) OnChange(onChange func(*SearchState) tui.Msg) Widget {
	w.onChange = onChange
	return w
}

// OnSelect sets the function that turns the selected item into a message.
func (w Widget) OnSelect(onSelect func(Item) tui.Msg) Widget {
	w.onSelect = onSelect
	return w
}

// OnCancel sets the message the picker returns when it is cancelled.
func (w Widget) OnCancel(msg tui.Msg) Widget {
	w.onCancel = msg
	return w
}

// Size returns Fill, as the picker takes its whole area.
func (Widget) Size() (width, height layout.Length) {
	return layout.Fill, layout.Fill
}

// Layout returns the size of limits, as the picker takes its whole area.
func (Widget) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}

// Draw draws the query above the list.
func (w Widget) Draw(screen tui.Screen, area tui.Rectangle) {
	w.layout().Draw(screen, area)
}

// Handle returns the OnSelect and OnCancel messages for their Actions, sends the list's keys to the list, and passes other messages to the query and the list.
func (w Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	if key, ok := msg.(tui.KeyMsg); ok {
		switch w.keybind(key) {
		case ActionSelect:
			if item, ok := w.searchState.Selected(w.items); ok && w.onSelect != nil {
				return w.onSelect(item)
			}
			return nil
		case ActionCancel:
			return w.onCancel
		}
		if w.listKeybind(key) != list.ActionNone {
			area.Y, area.Height = area.Y+inputHeight, max(area.Height-inputHeight, 0)
			return w.listView().Handle(msg, area)
		}
	}
	return w.layout().Handle(msg, area)
}

func (w Widget) layout() tui.Widget {
	query := textinput.New(w.searchState.query, w.searchState.queryCursor).
		Focused(true).
		OnInput(w.queryChange)
	// A line below the query separates it from the list.
	var line tui.BorderSet
	line.Bottom = separator
	header := box.New(row.New(text.New(prompt), query)).Borders(tui.BordersBottom).BorderSet(line).BorderStyle(tcell.StyleDefault.Dim(true))
	return column.New(column.New(header).Height(layout.Fixed(inputHeight)), w.listView())
}

func (w Widget) listView() viewport.Widget {
	s, items := w.searchState, w.items
	l := list.New(s.cursor, s.count(items), func(i int) tui.Widget { return items[s.index(i)].Content }).
		SelectedStyle(tcell.StyleDefault.Reverse(true)).
		Keybind(w.listKeybind).
		Focused(true).
		OnChange(func(cursor int) tui.Msg {
			return w.change(func(s *SearchState) {
				s.cursor = cursor
				s.scroll.ScrollToTarget()
			})
		})
	return viewport.New(l, s.scroll).
		Target(l.Target(s.cursor)).
		ScrollBar(w.scrollBar, w.scrollBarVisibility).
		OnChange(func(scroll *viewport.ScrollState) tui.Msg {
			return w.change(func(s *SearchState) { s.scroll = *scroll })
		})
}

// queryChange turns an edit of the query into the OnChange message, matching the items against the query if the edit changed it.
func (w Widget) queryChange(value string, cursor int) tui.Msg {
	return w.change(func(s *SearchState) {
		changed := value != s.query
		s.query, s.queryCursor = value, cursor
		if !changed {
			return
		}

		s.matches = nil
		count := len(w.items)
		if value != "" {
			for _, match := range fuzzy.FindFrom(value, w.items) {
				s.matches = append(s.matches, match.Index)
			}

			count = len(s.matches)
		}

		s.cursor = min(0, count-1)
		s.scroll.ScrollToTarget()
	})
}

// change returns the OnChange message for the search state that edit leaves.
func (w Widget) change(edit func(*SearchState)) tui.Msg {
	if w.onChange == nil {
		return nil
	}

	s := *w.searchState
	edit(&s)
	return w.onChange(&s)
}
