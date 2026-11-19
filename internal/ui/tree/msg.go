package tree

import (
	"fmt"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/ui"
)

// YankID copies the ID of node to the clipboard.
func YankID(node *tree.Node) tui.Cmd {
	if node == nil {
		return nil
	}

	id, ok := node.Reference().(fmt.Stringer)
	if !ok {
		return nil
	}
	return ui.WriteClipboard(id.String())
}
