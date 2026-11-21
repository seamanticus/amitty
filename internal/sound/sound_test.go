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
		if got, err := Path(mine, "pop", dir); got != mine || err != nil {
			t.Fatalf("Path = %q, %v", got, err)
		}
	})
	t.Run("set but missing", func(t *testing.T) {
		if _, err := Path(filepath.Join(dir, "missing.ogg"), "", dir); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("err = %v, want not exist", err)
		}
	})
	t.Run("set to a directory", func(t *testing.T) {
		if _, err := Path(dir, "", dir); err == nil {
			t.Fatal("no error")
		}
	})
	t.Run("missing or unsafe name", func(t *testing.T) {
		for _, name := range []string{"nope.wav", "../x.wav", "a/b.wav", ".."} {
			if _, err := Path("", name, dir); err == nil {
				t.Errorf("Path(%q): no error", name)
			}
		}
	})
	t.Run("shipped sounds are written once", func(t *testing.T) {
		if err := Install(dir); err != nil {
			t.Fatal(err)
		}
		got, err := Path("", "", dir)
		if err != nil || got != filepath.Join(Dir(dir), Default) {
			t.Fatalf("Path = %q, %v", got, err)
		}
		for _, name := range Shipped() {
			want, _ := shipped.ReadFile("sounds/" + name)
			if data, err := os.ReadFile(filepath.Join(Dir(dir), name)); err != nil || !bytes.Equal(data, want) {
				t.Fatalf("%s not written: %v", name, err)
			}
		}
		// A user's own file there is kept.
		if err := os.WriteFile(got, []byte("mine"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Install(dir); err != nil {
			t.Fatal(err)
		}
		if data, _ := os.ReadFile(got); string(data) != "mine" {
			t.Fatal("the file in place was replaced")
		}
	})
	t.Run("a shipped sound deleted since start is written again", func(t *testing.T) {
		fresh := t.TempDir()
		if err := Install(fresh); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(Dir(fresh), Default)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if got, err := Path("", "", fresh); got != path || err != nil {
			t.Fatalf("Path = %q, %v", got, err)
		}
	})
	t.Run("a file of the user's own is played by name", func(t *testing.T) {
		mine := filepath.Join(Dir(dir), "horn.mp3")
		if err := os.WriteFile(mine, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := Path("", "horn.mp3", dir); got != mine || err != nil {
			t.Fatalf("Path = %q, %v", got, err)
		}
	})
}

// evidence: witnesses
func TestShipped(t *testing.T) {
	names := Shipped()
	if len(names) < 12 || !slices.Contains(names, Default) {
		t.Fatalf("names = %q, want at least 12 with %q", names, Default)
	}
	for _, name := range names {
		data, err := shipped.ReadFile("sounds/" + name)
		if err != nil || len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
			t.Errorf("%s is not a WAV file: %v", name, err)
		}
	}
}

// evidence: witnesses
func TestNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"horn.mp3", "a.ogg", ".hidden", Default} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(slices.Values(append(Shipped(), "a.ogg", "horn.mp3")))
	if got := Names(dir); !slices.Equal(got, want) {
		t.Fatalf("Names = %q, want %q", got, want)
	}
	// A missing dir offers the shipped sounds alone.
	if got := Names(filepath.Join(dir, "missing")); !slices.Equal(got, Shipped()) {
		t.Fatalf("Names = %q, want the shipped sounds", got)
	}
}
