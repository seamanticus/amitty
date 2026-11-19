package picker

import (
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/picker"
)

func pickerKeybind(keybinds config.PickerKeybinds) func(tui.KeyMsg) picker.Action {
	return func(key tui.KeyMsg) picker.Action {
		switch {
		case keybind.Matches(key, keybinds.Select.Keybind):
			return picker.ActionSelect
		case keybind.Matches(key, keybinds.Cancel.Keybind):
			return picker.ActionCancel
		}
		return picker.ActionNone
	}
}
