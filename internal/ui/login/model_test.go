package login

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/login/password"
)

func newModel(t *testing.T) Model {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(cfg, http.Build{}, http.Network{})
}

// dialogs runs the tasks in command, one level of batches deep, and returns the dialogs they show.
func dialogs(command tui.Cmd) []ui.DialogMsg {
	var commands []tui.Cmd
	if batch, ok := command.(cmd.Batch); ok {
		commands = batch
	} else {
		commands = []tui.Cmd{command}
	}
	var shown []ui.DialogMsg
	for _, c := range commands {
		if task, ok := c.(cmd.Task); ok {
			if dialog, ok := task().(ui.DialogMsg); ok {
				shown = append(shown, dialog)
			}
		}
	}
	return shown
}

func TestModelUpdate(t *testing.T) {
	// proof: exhaustive
	t.Run("selecting a tab activates it", func(t *testing.T) {
		m := newModel(t)
		for tab := range tabCount {
			if m, _ = m.Update(selectTabMsg(tab)); m.active != tab {
				t.Fatalf("active = %d, want %d", m.active, tab)
			}
		}
	})

	t.Run("only the QR tab starts work when selected", func(t *testing.T) {
		m := newModel(t)
		for tab := range tabCount {
			_, command := m.Update(selectTabMsg(tab))
			if (command != nil) != (tab == tabQR) {
				t.Fatalf("tab %d: command = %v", tab, command)
			}
		}
	})

	// proof: exhaustive
	t.Run("a password failure shows once and unlocks the form from any tab", func(t *testing.T) {
		for tab := range tabCount {
			m := newModel(t)
			if !hasTask(fillAndSubmit(&m)) {
				t.Fatal("the first submit started no request")
			}
			if hasTask(press(&m, key(tcell.KeyEnter))) {
				t.Fatal("a second submit started a request while the first is in flight")
			}

			m.active = tab
			m, command := m.Update(password.FailedMsg{Err: errors.New("bad password")})
			if shown := dialogs(command); len(shown) != 1 || shown[0].Text != "bad password" {
				t.Fatalf("tab %d: dialogs = %+v, want one with the error", tab, shown)
			}

			m.active = tabPassword
			if !hasTask(press(&m, key(tcell.KeyEnter))) {
				t.Fatalf("tab %d: the form is still locked", tab)
			}
		}
	})

	t.Run("an error shows a dialog with Copy and Close", func(t *testing.T) {
		m := newModel(t)
		m.active = tabQR
		_, command := m.Update(errors.New("gateway closed"))
		shown := dialogs(command)
		if len(shown) != 1 || shown[0].Text != "gateway closed" || len(shown[0].Buttons) != 2 {
			t.Fatalf("dialogs = %+v", shown)
		}
		if b := shown[0].Buttons; b[0].Label != "Copy" || !b[0].KeepOpen || b[1].Label != "Close" {
			t.Fatalf("buttons = %+v", b)
		}
	})

	t.Run("leaving the QR tab ends its login, whose late errors show nothing", func(t *testing.T) {
		m := newModel(t)
		m, _ = m.Update(selectTabMsg(tabQR))
		m, command := m.Update(selectTabMsg(tabPassword))
		if !hasTask(command) {
			t.Fatal("leaving ran nothing to close the connection")
		}
		if _, command := m.Update(errors.New("remote auth gateway: session closed (close 1000)")); len(dialogs(command)) != 0 {
			t.Fatalf("late error shown: %+v", dialogs(command))
		}
	})
}

var area = tui.Rectangle{Width: 60, Height: 15}

func key(k tcell.Key) tui.KeyMsg { return tcell.NewEventKey(k, "", tcell.ModNone) }

// press delivers msgs the way the application does, through the view then Update, and returns the last command.
func press(m *Model, msgs ...tui.Msg) tui.Cmd {
	var command tui.Cmd
	for _, msg := range msgs {
		if msg = m.View().Handle(msg, area); msg != nil {
			*m, command = m.Update(msg)
		}
	}
	return command
}

// fillAndSubmit types a login and password on the password tab and presses the button, returning the submit's command.
func fillAndSubmit(m *Model) tui.Cmd {
	m.active = tabPassword
	return press(m,
		tcell.NewEventKey(tcell.KeyRune, "u", tcell.ModNone), key(tcell.KeyEnter),
		tcell.NewEventKey(tcell.KeyRune, "p", tcell.ModNone), key(tcell.KeyEnter),
		key(tcell.KeyEnter),
	)
}

// hasTask reports whether command, or a command in its batch, is a task: work that would reach the network. It does not run it.
func hasTask(command tui.Cmd) bool {
	if batch, ok := command.(cmd.Batch); ok {
		return slices.ContainsFunc(batch, hasTask)
	}
	_, ok := command.(cmd.Task)
	return ok
}
