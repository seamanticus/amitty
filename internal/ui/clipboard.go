package ui

import (
	"context"
	"log/slog"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"golang.design/x/clipboard"
)

// WriteClipboard copies text to the clipboard.
func WriteClipboard(text string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if _, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(text)); err != nil {
			slog.Error("failed to write to clipboard", "err", err)
		}
		return nil
	})
}
