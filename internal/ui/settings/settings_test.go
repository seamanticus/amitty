package settings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// panel returns the panel for a config file holding content.
func panel(t *testing.T, content string) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return New(cfg, path), path
}

func option(t *testing.T, m Model, path string) config.Option {
	t.Helper()
	for _, o := range config.Options(m.cfg) {
		if o.Path == path {
			return o
		}
	}
	t.Fatalf("no option %s", path)
	return config.Option{}
}

// run runs the task command returns and passes its message to the panel, returning the panel and the message.
func run(t *testing.T, m Model, command tui.Cmd) (Model, tui.Msg) {
	t.Helper()
	task, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command = %#v, want a task", command)
	}
	msg := task()
	if own, ok := msg.(Msg); ok {
		m, command = m.Update(own)
		if command != nil {
			_, msg = run(t, m, command)
		}
	}
	return m, msg
}

func TestToggle(t *testing.T) {
	m, path := panel(t, "# mine\nmouse = true\n")
	m, command := m.Update(Msg{pickedMsg{option(t, m, "mouse")}})
	m, msg := run(t, m, command)
	saved, ok := msg.(SavedMsg)
	if !ok || saved.Path != "mouse" || !saved.Restart || saved.Config.Mouse {
		t.Fatalf("message = %#v, want mouse saved off, on restart", msg)
	}
	if !strings.Contains(m.status, "after a restart") {
		t.Fatalf("status = %q", m.status)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "# mine\nmouse = false\n" {
		t.Fatalf("file = %q", data)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want the file's own", info.Mode().Perm())
	}
}

func TestChoice(t *testing.T) {
	t.Run("enter steps to the next value", func(t *testing.T) {
		m, path := panel(t, "")
		// circle is the last of square, rounded, circle, so the next wraps around.
		m, command := m.Update(Msg{pickedMsg{option(t, m, "ui.messages_list.avatars.shape")}})
		if m.step != stepList {
			t.Fatalf("step = %d, want the list", m.step)
		}
		if _, msg := run(t, m, command); msg == nil {
			t.Fatal("not saved")
		}
		if data, _ := os.ReadFile(path); !strings.Contains(string(data), `shape = "square"`) {
			t.Fatalf("file = %q", data)
		}
	})
	t.Run("profiles are listed by name and previewed", func(t *testing.T) {
		m, path := panel(t, "")
		profile := option(t, m, "theme_profile")
		names := profile.Choices[1:]
		if profile.Choices[0] != `""` || !slices.IsSortedFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }) {
			t.Fatalf("choices = %v, want none then sorted names", profile.Choices[:4])
		}
		m, _ = m.Update(Msg{pickedMsg{profile}})
		if m.step != stepChoice {
			t.Fatalf("step = %d, want the list of profiles", m.step)
		}
		task, ok := m.preview(`"Nord"`).(cmd.Task)
		if preview, isPreview := task().(PreviewMsg); !ok || !isPreview || preview.Config.ThemeProfile != "Nord" {
			t.Fatal("no preview of Nord")
		}
		if data, _ := os.ReadFile(path); len(data) != 0 {
			t.Fatalf("a preview wrote the file: %q", data)
		}
		// Leaving after browsing shows the file's own config again.
		m.previewed = `"Nord"`
		m, command := m.Update(Msg{backMsg{}})
		if _, msg := run(t, m, command); msg == nil || msg.(PreviewMsg).Config.ThemeProfile != "" {
			t.Fatalf("message = %#v, want the file's config back", msg)
		}
	})
}

func TestText(t *testing.T) {
	m, path := panel(t, "")
	m, _ = m.Update(Msg{pickedMsg{option(t, m, "keybinds.quit")}})
	if m.step != stepText || m.text != `"ctrl+c"` {
		t.Fatalf("step %d with %q, want the value to type over", m.step, m.text)
	}
	t.Run("a bad value is refused and the file kept", func(t *testing.T) {
		m, _ := m.Update(Msg{inputMsg{"ctrl+q", 6}})
		m, command := m.Update(Msg{submitMsg{}})
		m, _ = run(t, m, command)
		if m.status == "" || m.step != stepText {
			t.Fatalf("status %q at step %d, want an error while still typing", m.status, m.step)
		}
		if data, _ := os.ReadFile(path); len(data) != 0 {
			t.Fatalf("file = %q", data)
		}
	})
	t.Run("a good value is saved", func(t *testing.T) {
		m, _ := m.Update(Msg{inputMsg{`"ctrl+q"`, 8}})
		m, command := m.Update(Msg{submitMsg{}})
		m, _ = run(t, m, command)
		if m.step != stepList {
			t.Fatalf("step = %d, want the list", m.step)
		}
		if data, _ := os.ReadFile(path); string(data) != "\n[keybinds]\nquit = \"ctrl+q\"\n" {
			t.Fatalf("file = %q", data)
		}
	})
	t.Run("esc goes back, then closes", func(t *testing.T) {
		m, _ := m.Update(Msg{backMsg{}})
		if m.step != stepList {
			t.Fatal("did not go back")
		}
		if _, msg := run(t, m, func() tui.Cmd { _, c := m.Update(Msg{backMsg{}}); return c }()); msg != (ClosedMsg{}) {
			t.Fatalf("message = %#v, want closed", msg)
		}
	})
}

func TestFileOnly(t *testing.T) {
	m, path := panel(t, "")
	m, command := m.Update(Msg{pickedMsg{option(t, m, "openers")}})
	if command != nil || !strings.Contains(m.status, path) {
		t.Fatalf("status %q, command %v, want the file to edit by hand", m.status, command)
	}
}
