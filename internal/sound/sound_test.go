package sound

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCommand(t *testing.T) {
	// on returns a lookPath that finds only names.
	on := func(names ...string) func(string) (string, error) {
		return func(name string) (string, error) {
			if slices.Contains(names, name) {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		}
	}
	// evidence: witnesses
	for _, test := range []struct {
		name, command string
		found         []string
		want          []string
		ok            bool
	}{
		{"ffplay first", "", []string{"aplay", "ffplay", "pw-play"}, []string{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "a.wav"}, true},
		{"pw-play next", "", []string{"paplay", "pw-play"}, []string{"pw-play", "a.wav"}, true},
		{"aplay last", "", []string{"aplay"}, []string{"aplay", "-q", "a.wav"}, true},
		{"none", "", nil, nil, false},
		{"placeholder", "mpv --no-video {}", nil, []string{"mpv", "--no-video", "a.wav"}, true},
		{"appended", "mpv --no-video", nil, []string{"mpv", "--no-video", "a.wav"}, true},
		{"blank has no program", "  ", nil, nil, false},
		{"unbalanced quote", `mpv "x`, nil, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Command(test.command, "a.wav", on(test.found...))
			if (err == nil) != test.ok || !slices.Equal(got, test.want) {
				t.Fatalf("Command = %q, %v; want %q, ok %v", got, err, test.want, test.ok)
			}
		})
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	t.Run("set", func(t *testing.T) {
		mine := filepath.Join(dir, "x.ogg")
		if err := os.WriteFile(mine, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := Path(mine, dir); got != mine || err != nil {
			t.Fatalf("Path = %q, %v", got, err)
		}
	})
	t.Run("set but missing", func(t *testing.T) {
		if _, err := Path(filepath.Join(dir, "missing.ogg"), dir); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("err = %v, want not exist", err)
		}
	})
	t.Run("set to a directory", func(t *testing.T) {
		if _, err := Path(dir, dir); err == nil {
			t.Fatal("no error")
		}
	})
	t.Run("default is written once", func(t *testing.T) {
		got, err := Path("", dir)
		if err != nil || got != filepath.Join(dir, "sounds", "notification.wav") {
			t.Fatalf("Path = %q, %v", got, err)
		}
		if data, err := os.ReadFile(got); err != nil || !bytes.Equal(data, jingle) {
			t.Fatalf("default sound not written: %v", err)
		}
		// A user's own file there is kept.
		if err := os.WriteFile(got, []byte("mine"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Path("", dir); err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(got); string(data) != "mine" {
			t.Fatal("the file in place was replaced")
		}
	})
	t.Run("jingle is a wav", func(t *testing.T) {
		if len(jingle) < 44 || string(jingle[:4]) != "RIFF" || string(jingle[8:12]) != "WAVE" {
			t.Fatal("not a WAV file")
		}
	})
}
