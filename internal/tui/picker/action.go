package picker

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes a picker do. Keys that move the list are bound with ListKeybind.
type Action int

const (
	ActionNone Action = iota
	ActionSelect
	ActionCancel
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	switch keybind.String(key) {
	case "enter":
		return ActionSelect
	case "esc":
		return ActionCancel
	}
	return ActionNone
}
