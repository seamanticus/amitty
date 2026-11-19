package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// xdgOpenError describes one of xdg-open's documented exit codes.
func xdgOpenError(code int) (string, bool) {
	switch code {
	case 1:
		return "bad command line", true
	case 2:
		return "the file does not exist", true
	case 3:
		return "no application can open it", true
	case 4:
		return "the application failed", true
	}
	return "", false
}

// OpenExternal opens target, a path or URL, in the desktop's default application through xdg-open.
// It blocks until xdg-open exits, which is once the application starts unless no desktop handler exists.
func OpenExternal(ctx context.Context, target string) error {
	// No stdin, stdout or stderr: the child must not draw over the terminal UI.
	err := exec.CommandContext(ctx, "xdg-open", target).Run()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if reason, ok := xdgOpenError(exitErr.ExitCode()); ok {
			return fmt.Errorf("xdg-open: %s", reason)
		}
	}
	if err != nil {
		return fmt.Errorf("xdg-open: %w", err)
	}
	return nil
}
