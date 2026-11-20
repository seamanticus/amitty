package config

import (
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

var _ help.KeyMap = PickerKeybinds{}

func (k PickerKeybinds) ShortHelp() []keybind.Keybind {
	return []keybind.Keybind{k.SelectUp.Keybind, k.SelectDown.Keybind, k.Select.Keybind, k.Cancel.Keybind}
}

func (k PickerKeybinds) FullHelp() [][]keybind.Keybind {
	return [][]keybind.Keybind{
		{k.SelectUp.Keybind, k.SelectDown.Keybind, k.SelectTop.Keybind, k.SelectBottom.Keybind},
		{k.Select.Keybind, k.Cancel.Keybind},
	}
}

var _ help.KeyMap = PathPromptKeybinds{}

func (k PathPromptKeybinds) ShortHelp() []keybind.Keybind {
	return []keybind.Keybind{k.SelectUp.Keybind, k.SelectDown.Keybind, k.Complete.Keybind, k.Submit.Keybind, k.Cancel.Keybind}
}

func (k PathPromptKeybinds) FullHelp() [][]keybind.Keybind {
	return [][]keybind.Keybind{k.ShortHelp()}
}

var _ help.KeyMap = ProfileKeybinds{}

func (k ProfileKeybinds) ShortHelp() []keybind.Keybind {
	return []keybind.Keybind{k.YankID.Keybind, k.Close.Keybind}
}

func (k ProfileKeybinds) FullHelp() [][]keybind.Keybind {
	return [][]keybind.Keybind{k.ShortHelp()}
}
