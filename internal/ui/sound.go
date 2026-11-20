package ui

import (
	"context"
	"log/slog"
	"os/exec"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/sound"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// PlaySound returns the command that plays the notification sound cfg sets, logging why it could not.
func PlaySound(cfg config.NotificationSound) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		dataDir, err := consts.DataDir()
		if err != nil {
			slog.Error("failed to find the notification sound", "err", err)
			return nil
		}
		path, err := sound.Path(consts.ExpandHome(cfg.Path), string(cfg.Name), dataDir)
		if err != nil {
			slog.Error("failed to find the notification sound", "err", err)
			return nil
		}
		args, err := sound.Command(cfg.Command, path, exec.LookPath)
		if err == nil {
			slog.Debug("playing the notification sound", "args", args)
			err = sound.Play(context.Background(), args)
		}
		if err != nil {
			slog.Error("failed to play the notification sound", "path", path, "err", err)
		}
		return nil
	})
}
