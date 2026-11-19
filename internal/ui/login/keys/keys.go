// Package keys holds the keybinds of the login screen, shared by its tabs.
package keys

import "github.com/ayn2op/tview/keybind"

func PreviousTab() keybind.Keybind { return keybind.NewSingleKeybind("ctrl+h", "prev tab") }
func NextTab() keybind.Keybind     { return keybind.NewSingleKeybind("ctrl+l", "next tab") }

func FocusNext() keybind.Keybind { return keybind.New("tab", "down").WithHelp("tab", "next") }
func FocusPrevious() keybind.Keybind {
	return keybind.New("shift+tab", "up").WithHelp("shift+tab", "prev")
}

func OptionNext() keybind.Keybind     { return keybind.NewSingleKeybind("right", "next method") }
func OptionPrevious() keybind.Keybind { return keybind.NewSingleKeybind("left", "prev method") }

// Back leaves a step or cancels a pending login.
func Back(desc string) keybind.Keybind { return keybind.NewSingleKeybind("esc", desc) }
