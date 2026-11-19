package pathprompt

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

func TestSplit(t *testing.T) {
	for _, test := range []struct{ text, dir, prefix string }{
		{"", "", ""},
		{"a.txt", "", "a.txt"},
		{"/", "/", ""},
		{"/home/u/do", "/home/u/", "do"},
		{"~/", "~/", ""},
	} {
		if dir, prefix := split(test.text); dir != test.dir || prefix != test.prefix {
			t.Errorf("split(%q) = %q, %q, want %q, %q", test.text, dir, prefix, test.dir, test.prefix)
		}
	}
}

func TestMatches(t *testing.T) {
	names := []string{".hidden", "a.txt", "ab/", "b"}
	for _, test := range []struct {
		prefix string
		want   []string
	}{
		{"", []string{"a.txt", "ab/", "b"}},
		{"a", []string{"a.txt", "ab/"}},
		{".", []string{".hidden"}},
		{"z", nil},
	} {
		if got := matches(names, test.prefix); !slices.Equal(got, test.want) {
			t.Errorf("matches(%q) = %q, want %q", test.prefix, got, test.want)
		}
	}
}

func TestComplete(t *testing.T) {
	names := []string{"docs/", "download.png", "music/"}
	for _, test := range []struct{ text, want string }{
		{"/x/m", "/x/music/"},
		{"/x/do", "/x/do"},
		{"/x/dow", "/x/download.png"},
		{"/x/z", "/x/z"},
		{"/x/", "/x/"},
	} {
		if got := complete(test.text, names); got != test.want {
			t.Errorf("complete(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestExpand(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for _, test := range []struct {
		text, want string
		ok         bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"~", "/home/u", true},
		{"~/a", "/home/u/a", true},
		{"~user/a", "~user/a", true},
		{"/abs", "/abs", true},
		{"rel", "rel", true},
	} {
		if got, ok := expand(test.text); got != test.want || ok != test.ok {
			t.Errorf("expand(%q) = %q, %v, want %q, %v", test.text, got, ok, test.want, test.ok)
		}
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if got, want := list(dir+"/"), []string{"file", "link/", "sub/"}; !slices.Equal(got, want) {
		t.Errorf("list = %q, want %q", got, want)
	}
	if got := list(filepath.Join(dir, "missing") + "/"); got != nil {
		t.Errorf("list of a missing directory = %q, want nil", got)
	}
}

// run returns the message command makes, which must be a task.
func run(t *testing.T, command tui.Cmd) tui.Msg {
	t.Helper()
	task, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command %T is not a task", command)
	}
	return task()
}

func TestModel(t *testing.T) {
	cfg := config.Default()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var submitted string
	show := ShowMsg{Title: "Attach", Path: dir + "/", Submit: func(path string) tui.Cmd {
		submitted = path
		return nil
	}}

	m, command := New(&cfg, show)
	m, _ = m.Update(run(t, command))
	if !slices.Equal(m.names, []string{"photo.png"}) {
		t.Fatalf("names = %q, want the directory listed", m.names)
	}

	t.Run("typing in the same directory does not list it again", func(t *testing.T) {
		if _, command := m.Update(inputMsg{text: dir + "/p", cursor: len(dir) + 2}); command != nil {
			t.Error("listed again")
		}
	})
	t.Run("a stale listing is dropped", func(t *testing.T) {
		moved, _ := m.Update(inputMsg{text: "/", cursor: 1})
		if moved, _ = moved.Update(listedMsg{dir: dir + "/", names: []string{"x"}}); moved.names != nil {
			t.Errorf("names = %q, want the stale listing dropped", moved.names)
		}
	})
	t.Run("tab completes the name", func(t *testing.T) {
		typed, _ := m.Update(inputMsg{text: dir + "/p", cursor: len(dir) + 2})
		completed, _ := typed.Update(tui.KeyMsg(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone)))
		if want := dir + "/photo.png"; completed.text != want || completed.cursor != len(want) {
			t.Errorf("text, cursor = %q, %d, want %q at its end", completed.text, completed.cursor, want)
		}
	})
	t.Run("submit passes the path on", func(t *testing.T) {
		typed, _ := m.Update(inputMsg{text: dir + "/photo.png"})
		_, command := typed.Update(submitMsg{})
		if _, ok := run(t, command).(SubmittedMsg); !ok || submitted != dir+"/photo.png" {
			t.Errorf("submitted %q, want the typed path", submitted)
		}
	})
	t.Run("a blank path is not submitted", func(t *testing.T) {
		blank, _ := m.Update(inputMsg{text: " "})
		if _, command := blank.Update(submitMsg{}); command != nil {
			t.Error("blank path submitted")
		}
	})
	t.Run("esc cancels", func(t *testing.T) {
		_, command := m.Update(tui.KeyMsg(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone)))
		if _, ok := run(t, command).(CancelMsg); !ok {
			t.Error("esc did not cancel")
		}
	})
}
