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

// Default is the name of the sound played when none is set.
const Default = "chime"

// Names returns the names of the shipped sounds, sorted.
func Names() []string {
	entries, err := shipped.ReadDir("sounds")
	if err != nil {
		panic("invariant: the embedded sounds dir is missing")
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.TrimSuffix(entry.Name(), ".wav"))
	}
	return names
}

// players are tried in order: ffplay plays any format, as mpv and vlc pull in ffmpeg; the others are the sound servers' own players, for the shipped WAV sounds.
func players() [][]string {
	return [][]string{
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
		{"pw-play"},
		{"paplay"},
		{"aplay", "-q"},
	}
}

// Path returns path, which must name an existing file, or for "" the shipped sound name ("" for Default) in dataDir/sounds, writing the shipped sounds missing there first.
// The file is checked here as players such as ffplay exit 0 on a missing one, which would hide the mistake.
func Path(path, name, dataDir string) (string, error) {
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
	if name == "" {
		name = Default
	}
	if !slices.Contains(Names(), name) {
		return "", fmt.Errorf("notification sound: no sound named %q", name)
	}
	dir := filepath.Join(dataDir, "sounds")
	if err := writeShipped(dir); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".wav"), nil
}

// writeShipped writes every shipped sound missing from dir, keeping files already there, which may be the user's own.
func writeShipped(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sounds dir: %w", err)
	}
	for _, name := range Names() {
		path := filepath.Join(dir, name+".wav")
		_, err := os.Stat(path)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("notification sound: %w", err)
		}
		data, err := shipped.ReadFile("sounds/" + name + ".wav")
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
