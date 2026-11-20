package root

import (
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/ui/chat"
	"github.com/seamanticus/amitty/internal/ui/login"
)

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	return append(m.activeKeyMap().ShortHelp(), m.globalKeybinds()...)
}

func (m Model) FullHelp() [][]keybind.Keybind {
	return append(m.activeKeyMap().FullHelp(), m.globalKeybinds())
}

// globalKeybinds apply on every screen.
func (m Model) globalKeybinds() []keybind.Keybind {
	return []keybind.Keybind{
		m.cfg.Keybinds.Settings.Keybind,
		m.cfg.Keybinds.ToggleHelp.Keybind,
		m.cfg.Keybinds.ToggleFullHelp.Keybind,
		m.cfg.Keybinds.Suspend.Keybind,
		m.cfg.Keybinds.Quit.Keybind,
	}
}

func (m Model) activeKeyMap() help.KeyMap {
	// The panel takes every key while it is open.
	if m.settings != nil {
		return *m.settings
	}
	return m.screen.Match(
		func(l login.Model) help.KeyMap { return l },
		func(c chat.Model) help.KeyMap { return c },
	)
}
