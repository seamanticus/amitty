package memberstree

import (
	"slices"

	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/tree"
	uitree "github.com/seamanticus/amitty/internal/ui/tree"
)

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	keybinds := m.cfg.Keybinds.MembersTree
	short := uitree.SelectHelp(m.root, m.current, keybinds)
	if isGroup(m.current) {
		desc := "expand"
		if m.current.Expanded() {
			desc = "collapse"
		}

		short = append(short, keybinds.SelectCurrent.WithHelp(keybinds.SelectCurrent.Help().Key, desc))
	}

	if m.parent(m.current) != nil {
		short = append(short, keybinds.CollapseParentNode.Keybind)
	}

	return short
}

// treeKeybinds returns the keys that move through the tree's groups, each only while it does something.
func (m Model) treeKeybinds() []keybind.Keybind {
	keybinds := m.cfg.Keybinds.MembersTree
	var group []keybind.Keybind
	if isGroup(m.current) {
		group = append(group, keybinds.SelectCurrent.Keybind)
	}
	if m.parent(m.current) != nil {
		group = append(group, keybinds.MoveToParentNode.Keybind, keybinds.CollapseParentNode.Keybind)
	}
	if slices.ContainsFunc(m.root.Children(), (*tree.Node).Expanded) {
		group = append(group, keybinds.CollapseAll.Keybind)
	}
	return group
}

func (m Model) FullHelp() [][]keybind.Keybind {
	keybinds := m.cfg.Keybinds.MembersTree
	return [][]keybind.Keybind{
		{keybinds.SelectUp.Keybind, keybinds.SelectDown.Keybind, keybinds.SelectTop.Keybind, keybinds.SelectBottom.Keybind},
		{keybinds.ScrollUp.Keybind, keybinds.ScrollDown.Keybind, keybinds.ScrollTop.Keybind, keybinds.ScrollBottom.Keybind, keybinds.ScrollLeft.Keybind, keybinds.ScrollRight.Keybind},
		m.treeKeybinds(),
		{keybinds.YankID.Keybind},
	}
}
