package ui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

func TestCommandArgs(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		name, command string
		want          []string
	}{
		{"appended", "mpv --fs", []string{"mpv", "--fs", "/a b"}},
		{"replaced", "mpv {} --fs", []string{"mpv", "/a b", "--fs"}},
		{"inside a word", "imv --file={}", []string{"imv", "--file=/a b"}},
		{"quoted", `sh -c "echo hi" {}`, []string{"sh", "-c", "echo hi", "/a b"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := commandArgs(test.command, "{}", "/a b")
			if err != nil || !slices.Equal(got, test.want) {
				t.Fatalf("commandArgs = %q, %v, want %q", got, err, test.want)
			}
		})
	}
	for _, command := range []string{"", "   ", `mpv "unclosed`} {
		if _, err := commandArgs(command, "{}", "x"); err == nil {
			t.Errorf("commandArgs(%q) has no error", command)
		}
	}
}

func TestOpenersFor(t *testing.T) {
	openers := config.Openers{
		"video/*":    {Command: "mpv"},
		"video/webm": {Command: "vlc"},
		"image/png":  {Command: ""},
		"url":        {Command: "firefox"},
	}
	// evidence: witnesses
	for _, test := range []struct {
		key, want string
		ok        bool
	}{
		{"video/mp4", "mpv", true},
		{"video/webm", "vlc", true},
		{"video/webm; codecs=vp9", "vlc", true},
		{"image/png", "", false},
		{"audio/ogg", "", false},
		{"", "", false},
		{config.OpenerURL, "firefox", true},
	} {
		if got, ok := openers.For(test.key); ok != test.ok || got.Command != test.want {
			t.Errorf("For(%q) = %q, %v, want %q, %v", test.key, got.Command, ok, test.want, test.ok)
		}
	}
}

func TestPickedPaths(t *testing.T) {
	if got, want := pickedPaths("/a\n\n/b c\r\n/d"), []string{"/a", "/b c", "/d"}; !slices.Equal(got, want) {
		t.Fatalf("pickedPaths = %q, want %q", got, want)
	}
	if got := pickedPaths(""); got != nil {
		t.Fatalf("pickedPaths(\"\") = %q", got)
	}
}

func TestOpenTerminal(t *testing.T) {
	openers := config.Openers{"text/plain": {Command: "true", Terminal: true}, "image/*": {Command: "true"}}
	if _, ok := Open(openers, "text/plain", "/f", "open").(cmd.Suspend); !ok {
		t.Error("a terminal opener does not suspend amitty")
	}
	if _, ok := Open(openers, "image/png", "/f", "open").(cmd.Task); !ok {
		t.Error("a background opener is not a task")
	}
}

func TestRunPicker(t *testing.T) {
	dir, cache := t.TempDir(), t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	t.Run("from stdout", func(t *testing.T) {
		got, err := runPicker("printf '"+a+`\n`+b+`\n'`, cache)
		if err != nil || !slices.Equal(got, []string{a, b}) {
			t.Fatalf("runPicker = %q, %v", got, err)
		}
	})
	t.Run("from the chooser file", func(t *testing.T) {
		got, err := runPicker(`sh -c "echo `+a+` > $0" {file}`, cache)
		if err != nil || !slices.Equal(got, []string{a}) {
			t.Fatalf("runPicker = %q, %v", got, err)
		}
	})
	t.Run("nothing chosen", func(t *testing.T) {
		if got, err := runPicker("false", cache); err != nil || got != nil {
			t.Fatalf("runPicker = %q, %v, want nothing", got, err)
		}
	})
	t.Run("no program", func(t *testing.T) {
		if _, err := runPicker("amitty-no-such-picker", cache); err == nil {
			t.Fatal("no error")
		}
	})
	if entries, _ := os.ReadDir(cache); len(entries) > 0 {
		t.Fatalf("chooser files left: %v", entries)
	}
}
