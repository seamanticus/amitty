package login

import (
	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/help"
	"github.com/ayn2op/tview/keybind"
	"github.com/ayn2op/tview/tabs"
	"github.com/seamanticus/amitty/internal/ui/login/keys"
)

func tabsKeybind(key tview.KeyMsg) tabs.Action {
	switch {
	case keybind.Matches(key, keys.PreviousTab()):
		return tabs.ActionPrevious
	case keybind.Matches(key, keys.NextTab()):
		return tabs.ActionNext
	}
	return tabs.ActionNone
}

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	return append(m.navHelp(), m.activeKeyMap().ShortHelp()...)
}

func (m Model) FullHelp() [][]keybind.Keybind {
	var full [][]keybind.Keybind
	if nav := m.navHelp(); len(nav) > 0 {
		full = append(full, nav)
	}
	return append(full, m.activeKeyMap().FullHelp()...)
}

// navHelp returns the tab keybinds that switch to another tab from the active one.
func (m Model) navHelp() []keybind.Keybind {
	var nav []keybind.Keybind
	if m.active > 0 {
		nav = append(nav, keys.PreviousTab())
	}
	if m.active < tabCount-1 {
		nav = append(nav, keys.NextTab())
	}
	return nav
}

// activeKeyMap returns the keybinds of the active tab.
func (m Model) activeKeyMap() help.KeyMap {
	switch m.active {
	case tabPassword:
		return m.password
	case tabQR:
		return m.qr
	case tabToken:
		return m.token
	}
	panic("invariant: unknown login tab")
}
