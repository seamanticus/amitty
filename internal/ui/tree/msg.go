package tree

import (
	"fmt"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/tree"
	"github.com/seamanticus/amitty/internal/ui"
)

// YankID copies the ID of node to the clipboard.
func YankID(node *tree.Node) tview.Cmd {
	if node == nil {
		return nil
	}

	id, ok := node.Reference().(fmt.Stringer)
	if !ok {
		return nil
	}
	return ui.WriteClipboard(id.String())
}
