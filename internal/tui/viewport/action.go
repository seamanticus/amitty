package viewport

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

// Action is what a key makes a viewport do.
type Action int

const (
	ActionNone Action = iota
	ActionUp
	ActionDown
	ActionLeft
	ActionRight
	ActionTop
	ActionBottom
	ActionPageUp
	ActionPageDown
)

// DefaultKeybind is the default keybind.
func DefaultKeybind(key tui.KeyMsg) Action {
	switch keybind.String(key) {
	case "up":
		return ActionUp
	case "down":
		return ActionDown
	case "left":
		return ActionLeft
	case "right":
		return ActionRight
	case "home":
		return ActionTop
	case "end":
		return ActionBottom
	case "pgup":
		return ActionPageUp
	case "pgdn":
		return ActionPageDown
	}
	return ActionNone
}
