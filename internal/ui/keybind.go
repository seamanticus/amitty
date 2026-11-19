package ui

import (
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/list"
	"github.com/seamanticus/amitty/internal/tui/viewport"
)

// SelectionKeybind binds keybinds to moving a list's selection.
func SelectionKeybind(keybinds config.SelectionKeybinds) func(tui.KeyMsg) list.Action {
	return func(key tui.KeyMsg) list.Action {
		switch {
		case keybind.Matches(key, keybinds.SelectUp.Keybind):
			return list.ActionSelectUp
		case keybind.Matches(key, keybinds.SelectDown.Keybind):
			return list.ActionSelectDown
		case keybind.Matches(key, keybinds.SelectTop.Keybind):
			return list.ActionSelectTop
		case keybind.Matches(key, keybinds.SelectBottom.Keybind):
			return list.ActionSelectBottom
		}
		return list.ActionNone
	}
}

// ScrollKeybind binds keybinds to scrolling a viewport.
func ScrollKeybind(keybinds config.ScrollKeybinds) func(tui.KeyMsg) viewport.Action {
	return func(key tui.KeyMsg) viewport.Action {
		switch {
		case keybind.Matches(key, keybinds.ScrollUp.Keybind):
			return viewport.ActionUp
		case keybind.Matches(key, keybinds.ScrollDown.Keybind):
			return viewport.ActionDown
		case keybind.Matches(key, keybinds.ScrollTop.Keybind):
			return viewport.ActionTop
		case keybind.Matches(key, keybinds.ScrollBottom.Keybind):
			return viewport.ActionBottom
		}
		return viewport.ActionNone
	}
}
