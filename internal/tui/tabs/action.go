package tabs

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes tabs do.
type Action int

const (
	ActionNone Action = iota
	ActionPrevious
	ActionNext
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	switch keybind.String(key) {
	case "ctrl+h":
		return ActionPrevious
	case "ctrl+l":
		return ActionNext
	}
	return ActionNone
}
