package password

import (
	"errors"
	"strings"
	"testing"

	"github.com/ayn2op/tview"
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/discordo/internal/discordtest"
)

var area = tview.Rectangle{Width: 40, Height: 7}

// send delivers msg the way the application does: through the view, then Update.
func send(m *Model, msg tview.Msg) (tview.Msg, tview.Cmd) {
	var command tview.Cmd
	if msg = m.View().Handle(msg, area); msg != nil {
		*m, command = m.Update(msg)
	}
	return msg, command
}

func key(k tcell.Key, str string) tview.KeyMsg { return tcell.NewEventKey(k, str, tcell.ModNone) }

func typeText(m *Model, s string) {
	for _, r := range s {
		send(m, key(tcell.KeyRune, string(r)))
	}
}

func TestModelUpdate(t *testing.T) {
	var m Model
	send(&m, key(tcell.KeyRune, "a"))
	send(&m, key(tcell.KeyEnter, ""))
	send(&m, key(tcell.KeyRune, "b"))
	t.Run("typing fills the focused input and Enter moves on", func(t *testing.T) {
		if m.login != "a" || m.password != "b" {
			t.Fatalf("values = %q, %q", m.login, m.password)
		}
	})
	t.Run("enter on the button submits", func(t *testing.T) {
		send(&m, key(tcell.KeyTab, ""))
		if got, _ := send(&m, key(tcell.KeyEnter, "")); got != (submitMsg{}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("a submit in flight blocks another until it fails", func(t *testing.T) {
		if _, command := m.Update(submitMsg{}); command != nil {
			t.Fatal("second submit started a request")
		}
		m, _ = m.Update(FailedMsg{})
		if _, command := m.Update(submitMsg{}); command == nil {
			t.Fatal("submit after a failure started no request")
		}
	})
}

// evidence: witnesses
func TestModelSubmit(t *testing.T) {
	for _, tt := range []struct {
		name            string
		login, password string
		wantRequest     bool
	}{
		{"both empty", "", "", false},
		{"login empty", "", "p", false},
		{"password empty", "u", "", false},
		{"both set", "u", "p", true},
		{"whitespace counts as set", " ", " ", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{login: tt.login, password: tt.password}
			m, command := m.Update(submitMsg{})
			if (command != nil) != tt.wantRequest || m.submitting != tt.wantRequest {
				t.Fatalf("request = %v, submitting = %v, want %v", command != nil, m.submitting, tt.wantRequest)
			}
		})
	}

	t.Run("a failure keeps what was typed", func(t *testing.T) {
		m := Model{login: "u", password: "p", submitting: true}
		m, _ = m.Update(FailedMsg{errors.New("x")})
		if m.submitting || m.login != "u" || m.password != "p" {
			t.Fatalf("model = %+v", m)
		}
	})

	t.Run("a stray failure while idle changes nothing", func(t *testing.T) {
		m := Model{login: "u"}
		if got, _ := m.Update(FailedMsg{}); got != m {
			t.Fatalf("got %+v, want %+v", got, m)
		}
	})
}

// proof: exhaustive
func TestModelFocus(t *testing.T) {
	t.Run("Tab and Down cycle forward through every field", func(t *testing.T) {
		for _, k := range []tcell.Key{tcell.KeyTab, tcell.KeyDown} {
			var m Model
			for want := range focusCount {
				if m.focus != (want)%focusCount {
					t.Fatalf("key %v step %d: focus = %d", k, want, m.focus)
				}
				m, _ = m.Update(key(k, ""))
			}
			if m.focus != focusLogin {
				t.Fatalf("key %v did not wrap to the login, focus = %d", k, m.focus)
			}
		}
	})
	t.Run("Backtab and Up cycle backward through every field", func(t *testing.T) {
		for _, k := range []tcell.Key{tcell.KeyBacktab, tcell.KeyUp} {
			var m Model
			for step := range focusCount {
				m, _ = m.Update(key(k, ""))
				if want := (focusCount - 1 - step) % focusCount; m.focus != want {
					t.Fatalf("key %v step %d: focus = %d, want %d", k, step, m.focus, want)
				}
			}
		}
	})
	t.Run("Enter in the password moves to the button without submitting", func(t *testing.T) {
		m := Model{login: "u", password: "p", focus: focusPassword}
		if got, _ := send(&m, key(tcell.KeyEnter, "")); got == (submitMsg{}) || m.focus != focusButton {
			t.Fatalf("got %v, focus = %d", got, m.focus)
		}
	})
	t.Run("typing on the button changes no field", func(t *testing.T) {
		m := Model{focus: focusButton}
		typeText(&m, "x")
		if m.login != "" || m.password != "" {
			t.Fatalf("login = %q, password = %q", m.login, m.password)
		}
	})
}

func TestModelView(t *testing.T) {
	t.Run("the password is masked", func(t *testing.T) {
		m := Model{login: "user", password: "hunter2"}
		drawn := discordtest.Draw(t, m.View(), area.Width, area.Height)
		if strings.Contains(drawn, "hunter2") {
			t.Fatalf("password drawn in clear:\n%s", drawn)
		}
		if !strings.Contains(drawn, "user") || !strings.Contains(drawn, "*******") {
			t.Fatalf("login or mask missing:\n%s", drawn)
		}
	})
	t.Run("the button says a login is in flight", func(t *testing.T) {
		for _, submitting := range []bool{false, true} {
			drawn := discordtest.Draw(t, Model{submitting: submitting}.View(), area.Width, area.Height)
			if got := strings.Contains(drawn, "Logging in..."); got != submitting {
				t.Fatalf("submitting = %v, \"Logging in...\" drawn = %v:\n%s", submitting, got, drawn)
			}
		}
	})
}
