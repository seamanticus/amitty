package login

import (
	"log/slog"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/ui"
)

// selectTabMsg switches to the tab at an index.
type selectTabMsg int

func showErrorDialog(err error) tui.Cmd {
	slog.Error("failed to login", "err", err)
	message := err.Error()
	return ui.ShowDialog(message,
		ui.DialogButton{Label: "Copy", Cmd: ui.WriteClipboard(message), KeepOpen: true},
		ui.DialogButton{Label: "Close"},
	)
}
