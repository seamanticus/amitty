package ui

import (
	"log/slog"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

type DialogButton struct {
	Label    string
	Cmd      tui.Cmd
	KeepOpen bool
}

type DialogMsg struct {
	Text    string
	Buttons []DialogButton
}

func ShowDialog(text string, buttons ...DialogButton) tui.Cmd {
	return cmd.Task(func() tui.Msg { return DialogMsg{Text: text, Buttons: buttons} })
}

// ErrorDialog logs that what failed with err and returns the dialog that says so.
func ErrorDialog(what string, err error) DialogMsg {
	slog.Error("failed to "+what, "err", err)
	return DialogMsg{Text: "Failed to " + what + ": " + err.Error(), Buttons: []DialogButton{{Label: "OK"}}}
}
