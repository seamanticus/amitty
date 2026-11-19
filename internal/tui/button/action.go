package button

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes a focused button do.
type Action int

const (
	ActionNone Action = iota
	ActionPress
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	if keybind.String(key) == "enter" {
		return ActionPress
	}
	return ActionNone
}
