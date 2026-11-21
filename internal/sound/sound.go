// Package sound plays the notification sound through a player program, as Go has no audio output of its own without cgo.
package sound

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mattn/go-shellwords"
)

//go:generate go run gensounds.go

// shipped are the sounds amitty ships, written to the data dir for players that read only files.
//
//go:embed sounds/*.wav
var shipped embed.FS

// Default is the file name of the sound played when none is set.
const Default = "chime.wav"

// Shipped returns the file names of the sounds amitty ships, sorted.
func Shipped() []string {
	entries, err := shipped.ReadDir("sounds")
	if err != nil {
		panic("invariant: the embedded sounds dir is missing")
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// Names returns the file names a sound can be picked from, sorted: the shipped ones and every other file in dir, the user's own in any format. Hidden files and directories are left out.
func Names(dir string) []string {
	names := Shipped()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if name := entry.Name(); !strings.HasPrefix(name, ".") && !isDir(dir, entry) && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// isDir reports whether entry, in dir, is a directory or a link to one.
func isDir(dir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(dir, entry.Name()))
	return err == nil && info.IsDir()
}

// IsName reports whether name is a bare file name, which a sound in the sounds dir is picked by.
func IsName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}

// Dir returns the sounds dir in dataDir.
func Dir(dataDir string) string { return filepath.Join(dataDir, "sounds") }

// players are tried in order: ffplay plays any format, as mpv and vlc pull in ffmpeg; the others are the sound servers' own players, for the shipped WAV sounds.
func players() [][]string {
	return [][]string{
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
		{"pw-play"},
		{"paplay"},
		{"aplay", "-q"},
	}
}

// Path returns path, or for "" the file name ("" for Default) in the sounds dir of dataDir, writing the shipped sounds again if that one is missing. The file must exist.
// The file is checked here as players such as ffplay exit 0 on a missing one, which would hide the mistake.
func Path(path, name, dataDir string) (string, error) {
	if path == "" {
		if name == "" {
			name = Default
		}
		if !IsName(name) {
			return "", fmt.Errorf("notification sound: %q is not a file name", name)
		}
		path = filepath.Join(Dir(dataDir), name)
		// Install ran at start; this restores a shipped sound deleted since.
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) && slices.Contains(Shipped(), name) {
			if err := Install(dataDir); err != nil {
				return "", err
			}
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("notification sound: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("notification sound: %s is a directory", path)
	}
	return path, nil
}

// Install writes every shipped sound missing from the sounds dir of dataDir, keeping files already there, which may be the user's own.
func Install(dataDir string) error {
	dir := Dir(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sounds dir: %w", err)
	}
	for _, name := range Shipped() {
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("notification sound: %w", err)
		}
		data, err := shipped.ReadFile("sounds/" + name)
		if err != nil {
			panic("invariant: a listed sound is not embedded")
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write sound %s: %w", name, err)
		}
	}
	return nil
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
