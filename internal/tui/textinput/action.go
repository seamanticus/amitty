package textinput

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes a focused text input do. Typing a character inserts it.
type Action int

const (
	ActionNone Action = iota
	ActionLeft
	ActionRight
	ActionHome
	ActionEnd
	ActionBackspace
	ActionDelete
	ActionSubmit
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	switch keybind.String(key) {
	case "left":
		return ActionLeft
	case "right":
		return ActionRight
	case "home":
		return ActionHome
	case "end":
		return ActionEnd
	case "backspace":
		return ActionBackspace
	case "delete":
		return ActionDelete
	case "enter":
		return ActionSubmit
	}
	return ActionNone
}
