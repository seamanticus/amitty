package slider

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes a focused slider do.
type Action int

const (
	ActionNone Action = iota
	ActionDecrease
	ActionIncrease
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	switch keybind.String(key) {
	case "left", "shift+left":
		return ActionDecrease
	case "right", "shift+right":
		return ActionIncrease
	}

	return ActionNone
}
