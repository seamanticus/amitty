// Package sound plays the notification sound through a player program, as Go has no audio output of its own without cgo.
package sound

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
)

//go:generate go run genjingle.go

// jingle is the default notification sound, written to the data dir for players that read only files.
//
//go:embed notification.wav
var jingle []byte

// jingleName is the default sound's file in the sounds dir.
const jingleName = "notification.wav"

// players are tried in order: ffplay plays any format, as mpv and vlc pull in ffmpeg; the others are the sound servers' own players, for the default jingle.
func players() [][]string {
	return [][]string{
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
		{"pw-play"},
		{"paplay"},
		{"aplay", "-q"},
	}
}

// Path returns path, which must name an existing file, or for "" the default jingle in dataDir/sounds, writing it there if it is missing.
// The file is checked here as players such as ffplay exit 0 on a missing one, which would hide the mistake.
func Path(path, dataDir string) (string, error) {
	if path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("notification sound: %w", err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("notification sound: %s is a directory", path)
		}
		return path, nil
	}
	dir := filepath.Join(dataDir, "sounds")
	path = filepath.Join(dir, jingleName)
	if _, err := os.Stat(path); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return path, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create sounds dir: %w", err)
	}
	if err := os.WriteFile(path, jingle, 0o600); err != nil {
		return "", fmt.Errorf("write default sound: %w", err)
	}
	return path, nil
}

// Command returns the program and arguments that play path: command with {} replaced by it (or it appended), or for "" the first player found on $PATH.
func Command(command, path string, lookPath func(string) (string, error)) ([]string, error) {
	if command != "" {
		words, err := shellwords.Parse(command)
		if err != nil || len(words) == 0 {
			return nil, fmt.Errorf("parse sound command %q: %w", command, orNoProgram(err))
		}
		replaced := false
		for i, word := range words {
			if strings.Contains(word, "{}") {
				words[i], replaced = strings.ReplaceAll(word, "{}", path), true
			}
		}
		if !replaced {
			words = append(words, path)
		}
		return words, nil
	}
	for _, player := range players() {
		if _, err := lookPath(player[0]); err == nil {
			return append(player, path), nil
		}
	}
	return nil, errNoPlayer
}

var errNoPlayer = errors.New("no sound player found: install ffmpeg (ffplay), pipewire (pw-play), pulseaudio-utils (paplay) or alsa-utils (aplay), or set notifications.sound.command")

func orNoProgram(err error) error {
	if err != nil {
		return err
	}
	return errors.New("no program")
}

// Play runs args to the end, with no input or output, as a player writes to the terminal amitty draws on.
func Play(ctx context.Context, args []string) error {
	player := exec.CommandContext(ctx, args[0], args[1:]...)
	if err := player.Run(); err != nil {
		return fmt.Errorf("play sound with %s: %w", args[0], err)
	}
	return nil
}
