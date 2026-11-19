// Package emojis completes emojis.
package emojis

import (
	"github.com/sahilm/fuzzy"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	tuiimage "github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/ui/autocomplete"
	uiemojis "github.com/seamanticus/amitty/internal/ui/emojis"
	"github.com/seamanticus/amitty/internal/ui/images"
)

// Title is the autocomplete's title for emojis.
const Title = "Emojis"

type item struct {
	emoji discord.Emoji
	name  string
	style tui.Style
	// preview reports whether the image of a custom emoji is shown, so that none loaded for other panes is drawn when images are off.
	preview bool
}

func (i item) Insert() string { return i.emoji.String() }

func (i item) Line(images map[discord.URL]tuiimage.Widget) []text.Span {
	if !i.preview {
		images = nil
	}
	return uiemojis.Line(i.emoji, i.name, i.style, images)
}

// Items returns up to limit emojis matching query, best first, each once, and the command that loads their images, or nil.
func Items(cfg *config.Config, s *ningen.State, gID discord.GuildID, query string, limit int) ([]autocomplete.Item, tui.Cmd) {
	var names []string
	var emojis []discord.Emoji
	for name, emoji := range uiemojis.All(s, gID) {
		names, emojis = append(names, name), append(emojis, emoji)
	}

	matches := fuzzy.Find(query, names)
	// fuzzy matches nothing for an empty query.
	if query == "" {
		for i, name := range names {
			matches = append(matches, fuzzy.Match{Str: name, Index: i})
		}
	}

	preview := cfg.UI.Autocomplete.Emojis.Preview
	var items []autocomplete.Item
	load := images.RequestedMsg{Squares: map[discord.URL]uint{}}
	shown := make(map[discord.APIEmoji]bool)
	for _, match := range matches {
		if len(items) == limit {
			break
		}

		// An emoji has aliases.
		emoji := emojis[match.Index]
		if shown[emoji.APIString()] {
			continue
		}
		shown[emoji.APIString()] = true

		if preview && !emoji.IsUnicode() {
			load.Squares[uiemojis.URLFromID(emoji.ID)] = uiemojis.Rows
		}
		items = append(items, item{emoji, cfg.UI.MessagesList.Emojis.Name(match.Str), cfg.Theme.MessagesList.EmojiStyle.Style, preview})
	}

	if len(load.Squares) == 0 {
		return items, nil
	}
	return items, cmd.Task(func() tui.Msg { return load })
}
