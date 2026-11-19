// Package tree holds what amitty's trees share.
package tree

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/tui/viewport"
	"github.com/seamanticus/amitty/internal/ui"
)

// ScrollMsg scrolls the tree to ScrollState, before passing on Msg if the tree sent one.
type ScrollMsg struct {
	ScrollState viewport.ScrollState
	Msg         tui.Msg
}

// New returns a boxed tree of the nodes under root in a viewport.
func New(
	root, current *tree.Node,
	scrollState viewport.ScrollState,
	cfg *config.Config,
	theme config.TreeThemeConfig,
	graphics bool,
	keybinds config.TreeKeybinds,
	focused bool,
	onChange, onSelect func(*tree.Node) tui.Msg,
) box.Widget {
	set := cfg.UI.Border.NormalSet.BorderSet
	if focused {
		set = cfg.UI.Border.ActiveSet.BorderSet
	}
	t := tree.New(root, current).
		TopLevel(1).
		Markers(tree.Markers{
			Expanded:  cfg.Sidebar.Markers.Expanded,
			Collapsed: cfg.Sidebar.Markers.Collapsed,
			Leaf:      cfg.Sidebar.Markers.Leaf,
		}).
		Graphics(graphics).
		GraphicsSet(set).
		GraphicsStyle(tcell.StyleDefault.Foreground(tcell.GetColor(theme.GraphicsColor))).
		Keybind(treeKeybind(keybinds)).
		Focused(focused).
		OnChange(onChange).
		OnSelect(onSelect)
	v := viewport.New(t, scrollState).
		Target(t.Target).
		ContentWidth(t.RowsWidth).
		OnChildMsg(func(s *viewport.ScrollState, msg tui.Msg) tui.Msg { return ScrollMsg{ScrollState: *s, Msg: msg} }).
		Axes(layout.Axes{Width: true, Height: true}).
		Keybind(scrollKeybind(keybinds)).
		Focused(focused).
		OnChange(func(s *viewport.ScrollState) tui.Msg { return ScrollMsg{ScrollState: *s} })
	return ui.Box(v, cfg, focused)
}
