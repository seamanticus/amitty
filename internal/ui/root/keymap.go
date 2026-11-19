package root

import (
	"github.com/ayn2op/tview/help"
	"github.com/ayn2op/tview/keybind"
	"github.com/seamanticus/discordo/internal/ui/chat"
	"github.com/seamanticus/discordo/internal/ui/login"
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
		m.cfg.Keybinds.ToggleHelp.Keybind,
		m.cfg.Keybinds.ToggleFullHelp.Keybind,
		m.cfg.Keybinds.Suspend.Keybind,
		m.cfg.Keybinds.Quit.Keybind,
	}
}

func (m Model) activeKeyMap() help.KeyMap {
	return m.screen.Match(
		func(l login.Model) help.KeyMap { return l },
		func(c chat.Model) help.KeyMap { return c },
	)
}
