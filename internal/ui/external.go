package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-shellwords"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// commandArgs returns command split into words with every placeholder word replaced by arg, or with arg appended if no word holds placeholder.
// A placeholder inside a word, as in "--chooser-file={file}", is replaced within it.
func commandArgs(command, placeholder, arg string) ([]string, error) {
	words, err := shellwords.Parse(command)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", command, err)
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("parse %q: no program", command)
	}
	replaced := false
	for i, word := range words {
		if strings.Contains(word, placeholder) {
			words[i] = strings.ReplaceAll(word, placeholder, arg)
			replaced = true
		}
	}
	if !replaced {
		words = append(words, arg)
	}
	return words, nil
}

// Open returns the command that opens target, a downloaded file of type mediaType or a link when mediaType is config.OpenerURL, with its opener in openers, or with xdg-open when none is set.
// A terminal opener runs with amitty suspended; others start in the background. what names the action in an error dialog.
func Open(openers config.Openers, mediaType, target, what string) tui.Cmd {
	opener, ok := openers.For(mediaType)
	if !ok {
		return cmd.Task(func() tui.Msg {
			if err := OpenExternal(context.Background(), target); err != nil {
				return ErrorDialog(what, err)
			}
			return nil
		})
	}

	args, err := commandArgs(opener.Command, "{}", target)
	if err != nil {
		return cmd.Task(func() tui.Msg { return ErrorDialog(what, err) })
	}
	if opener.Terminal {
		return cmd.Suspend(func() tui.Msg {
			command := exec.Command(args[0], args[1:]...)
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				return ErrorDialog(what, fmt.Errorf("run %s: %w", args[0], err))
			}
			return nil
		})
	}
	return cmd.Task(func() tui.Msg {
		// No stdio, so the program cannot draw over the terminal UI. It outlives the task and is not waited for: its exit leaves a process entry until amitty exits.
		command := exec.Command(args[0], args[1:]...)
		if err := command.Start(); err != nil {
			return ErrorDialog(what, fmt.Errorf("run %s: %w", args[0], err))
		}
		_ = command.Process.Release()
		return nil
	})
}

// PickFiles returns the command that runs picker, suspending amitty, and passes the paths it chose to picked.
// The paths are read from the file a {file} word names, else from the picker's stdout, one per line; no path makes no message.
func PickFiles(picker string, picked func(paths []string) tui.Msg) tui.Cmd {
	return cmd.Suspend(func() tui.Msg {
		paths, err := runPicker(picker, consts.CacheDir())
		if err != nil {
			return ErrorDialog("pick files", err)
		}
		if len(paths) == 0 {
			return nil
		}
		return picked(paths)
	})
}

// runPicker runs picker, making the chooser file for a {file} word in dir.
func runPicker(picker, dir string) ([]string, error) {
	var chooser string
	if strings.Contains(picker, "{file}") {
		file, err := os.CreateTemp(dir, "picked-*")
		if err != nil {
			return nil, fmt.Errorf("create chooser file: %w", err)
		}
		chooser = file.Name()
		defer os.Remove(chooser) //nolint:errcheck
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("create chooser file: %w", err)
		}
	}

	words, err := shellwords.Parse(picker)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", picker, err)
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("parse %q: no program", picker)
	}
	for i, word := range words {
		words[i] = strings.ReplaceAll(word, "{file}", chooser)
	}

	command := exec.Command(words[0], words[1:]...)
	// The picker draws on the terminal through stderr and the tty, as fzf does when its stdout is captured.
	command.Stdin, command.Stderr = os.Stdin, os.Stderr
	var out []byte
	if chooser == "" {
		out, err = command.Output()
	} else {
		command.Stdout = os.Stdout
		if err = command.Run(); err == nil {
			out, err = os.ReadFile(chooser)
		}
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok && len(out) == 0 {
		// Pickers exit non-zero when nothing is chosen, as fzf does on esc.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("run %s: %w", words[0], err)
	}
	return pickedPaths(string(out)), nil
}

// pickedPaths returns the non-empty lines of out.
func pickedPaths(out string) []string {
	var paths []string
	for line := range strings.Lines(out) {
		if line = strings.TrimRight(line, "\r\n"); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}
