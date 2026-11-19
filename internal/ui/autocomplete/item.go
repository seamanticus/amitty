package autocomplete

import (
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	tuiimage "github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// Item is a suggestion.
type Item interface {
	// Insert returns the text that replaces the word being completed.
	Insert() string
	// Line returns the item as drawn, with images by URL, which may not be loaded yet.
	Line(images map[discord.URL]tuiimage.Widget) []text.Span
}
