package dialog

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes a dialog do.
type Action int

const (
	ActionNone Action = iota
	ActionPrevious
	ActionNext
	ActionPress
	ActionCancel
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	switch keybind.String(key) {
	case "shift+tab", "up", "left":
		return ActionPrevious
	case "tab", "down", "right":
		return ActionNext
	case "enter":
		return ActionPress
	case "esc":
		return ActionCancel
	}
	return ActionNone
}

// noKeys binds no keys, since the dialog presses its buttons itself.
func noKeys(tui.KeyMsg) button.Action {
	return button.ActionNone
}
