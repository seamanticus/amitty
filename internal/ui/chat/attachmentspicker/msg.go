package attachmentspicker

import "github.com/seamanticus/amitty/internal/tui"

type SelectedMsg struct {
	Action tui.Cmd
}

type CancelMsg struct{}
