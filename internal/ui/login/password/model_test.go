package password

import (
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/discordtest"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
)

var area = tui.Rectangle{Width: 80, Height: 15}

// send delivers msg the way the application does: through the view, then Update.
func send(m *Model, msg tui.Msg) (tui.Msg, tui.Cmd) {
	var command tui.Cmd
	if msg = m.View().Handle(msg, area); msg != nil {
		*m, command = m.Update(msg)
	}
	return msg, command
}

func key(k tcell.Key, str string) tui.KeyMsg { return tcell.NewEventKey(k, str, tcell.ModNone) }

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

func TestModelCountdown(t *testing.T) {
	start := func() Model {
		m := Model{login: "u", password: "p"}
		m, _ = m.Update(submitMsg{})
		return m
	}
	reason := errors.New("connection refused")

	t.Run("each second shows the warning and ticks, and a second submit is still ignored", func(t *testing.T) {
		m := start()
		for left := dhttp.RetryDelay; left > 0; left-- {
			var command tui.Cmd
			m, command = m.Update(CountdownMsg{left: left, err: reason})
			if m.retryIn != left || command == nil {
				t.Fatalf("left %d: retryIn = %d, command = %v", left, m.retryIn, command)
			}
			if _, again := m.Update(submitMsg{}); again != nil {
				t.Fatalf("left %d: a submit during the countdown started a request", left)
			}
		}
	})

	t.Run("at zero it retries and clears the warning", func(t *testing.T) {
		m, command := start().Update(CountdownMsg{left: 0, err: reason})
		if command == nil || m.retryIn != 0 || !m.submitting {
			t.Fatalf("retryIn = %d, submitting = %v, command = %v", m.retryIn, m.submitting, command)
		}
	})

	t.Run("a countdown with no login in flight is dropped", func(t *testing.T) {
		if _, command := (Model{}).Update(CountdownMsg{left: 3, err: reason}); command != nil {
			t.Fatal("a stray countdown started work")
		}
	})

	t.Run("the warning and the countdown share one line", func(t *testing.T) {
		m, _ := start().Update(CountdownMsg{left: 4, err: reason})
		drawn := discordtest.Draw(t, m.View(), 120, 11)
		var found bool
		for line := range strings.Lines(drawn) {
			if strings.Contains(line, "retrying once in 4 s") && strings.Contains(line, "connection refused") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no line with the warning and the countdown:\n%s", drawn)
		}
	})

	t.Run("a failure ends the countdown", func(t *testing.T) {
		m, _ := start().Update(CountdownMsg{left: 2, err: reason})
		m, _ = m.Update(FailedMsg{reason})
		if m.retryIn != 0 || m.submitting {
			t.Fatalf("retryIn = %d, submitting = %v", m.retryIn, m.submitting)
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
