package token

import (
	"strings"
	"testing"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/discordo/internal/discordtest"
)

var area = tview.Rectangle{Width: 40, Height: 5}

// send delivers msg the way the application does: through the view, then Update.
func send(m *Model, msg tview.Msg) (tview.Msg, tview.Cmd) {
	var command tview.Cmd
	if msg = m.View().Handle(msg, area); msg != nil {
		*m, command = m.Update(msg)
	}
	return msg, command
}

func key(k tcell.Key, str string) tview.KeyMsg { return tcell.NewEventKey(k, str, tcell.ModNone) }

func TestModelUpdate(t *testing.T) {
	t.Run("typing fills the token and Enter moves to the button", func(t *testing.T) {
		var m Model
		for _, r := range "abc" {
			send(&m, key(tcell.KeyRune, string(r)))
		}
		send(&m, key(tcell.KeyEnter, ""))
		if m.token != "abc" || !m.buttonFocused {
			t.Fatalf("token = %q, button focused = %v", m.token, m.buttonFocused)
		}
	})

	t.Run("Enter on the button submits the token as typed", func(t *testing.T) {
		m := Model{token: " tok ", buttonFocused: true}
		got, command := send(&m, key(tcell.KeyEnter, ""))
		if got != (submitMsg{}) {
			t.Fatalf("got %v, want submitMsg", got)
		}
		task, ok := command.(cmd.Task)
		if !ok {
			t.Fatalf("command = %#v, want a task", command)
		}
		if msg := task(); msg != TokenMsg(" tok ") {
			t.Fatalf("task returned %#v", msg)
		}
	})

	t.Run("an empty token is not submitted", func(t *testing.T) {
		if _, command := (Model{}).Update(submitMsg{}); command != nil {
			t.Fatal("empty token submitted")
		}
	})

	// proof: exhaustive
	t.Run("every navigation key toggles between input and button", func(t *testing.T) {
		for _, k := range []tcell.Key{tcell.KeyTab, tcell.KeyDown, tcell.KeyBacktab, tcell.KeyUp} {
			var m Model
			m, _ = m.Update(key(k, ""))
			if !m.buttonFocused {
				t.Fatalf("key %v did not focus the button", k)
			}
			m, _ = m.Update(key(k, ""))
			if m.buttonFocused {
				t.Fatalf("key %v did not focus the input again", k)
			}
		}
	})
}

func TestModelView(t *testing.T) {
	m := Model{token: "secret-token"}
	drawn := discordtest.Draw(t, m.View(), area.Width, area.Height)
	if strings.Contains(drawn, "secret-token") {
		t.Fatalf("token drawn in clear:\n%s", drawn)
	}
	if !strings.Contains(drawn, strings.Repeat("*", len("secret-token"))) {
		t.Fatalf("mask missing:\n%s", drawn)
	}
}
