package picker

import "github.com/seamanticus/amitty/internal/tui"

// Item is an entry of a picker, shown as its Content, such as text, and matched against the query by its FilterText.
type Item struct {
	Content    tui.Widget
	FilterText string
	Reference  any
}

type Items []Item

func (is Items) String(index int) string {
	return is[index].FilterText
}

func (is Items) Len() int {
	return len(is)
}
