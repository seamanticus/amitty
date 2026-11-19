package autocomplete

import (
	"github.com/seamanticus/amitty/internal/tui/viewport"
)

// SetItemsMsg replaces the items; none hides the list.
type SetItemsMsg struct {
	Title string
	Items []Item
}

// Msg is a message for the list.
type Msg interface{ isMsg() }

type listMsg int

func (listMsg) isMsg() {}

type scrollMsg viewport.ScrollState

func (scrollMsg) isMsg() {}
