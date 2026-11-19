package root

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tokenstore"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/chat"
	"github.com/seamanticus/amitty/internal/ui/login/password"
	"github.com/seamanticus/amitty/internal/ui/login/qr"
	"github.com/seamanticus/amitty/internal/ui/login/token"
)

func run(t *testing.T, command tui.Cmd) tui.Msg {
	t.Helper()
	task, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command = %#v, want a task", command)
	}
	return task()
}

// runAll runs every task in command, batches flattened, and returns their messages.
func runAll(command tui.Cmd) []tui.Msg {
	switch c := command.(type) {
	case cmd.Batch:
		var all []tui.Msg
		for _, inner := range c {
			all = append(all, runAll(inner)...)
		}
		return all
	case cmd.Task:
		return []tui.Msg{c()}
	}
	return nil
}

// testStore is a store in a temporary directory whose commands keep the token as is, so tests need no gpg key.
func testStore(t *testing.T) tokenstore.Store {
	t.Helper()
	store, err := tokenstore.New(filepath.Join(t.TempDir(), "state", tokenstore.FileName), "cat", "cat")
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// failingStore is a store whose commands always fail.
func failingStore(t *testing.T) tokenstore.Store {
	t.Helper()
	store, err := tokenstore.New(filepath.Join(t.TempDir(), tokenstore.FileName), "false", "false")
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func stored(t *testing.T, store tokenstore.Store) (string, bool) {
	t.Helper()
	token, ok, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return token, ok
}

func newModel(t *testing.T, store tokenstore.Store, token string) Model {
	t.Helper()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return NewModel(cfg, http.Build{}, http.Network{}, store, token)
}

func TestNewModelScreen(t *testing.T) {
	t.Run("a stored token opens the chat without storing again", func(t *testing.T) {
		store := testStore(t)
		if m := newModel(t, store, "t"); !m.screen.isChat() {
			t.Fatal("login shown with a token")
		}
		if got, ok := stored(t, store); ok {
			t.Fatalf("stored %q, want nothing", got)
		}
	})
	t.Run("no stored token shows the login", func(t *testing.T) {
		if newModel(t, testStore(t), "").screen.isChat() {
			t.Fatal("chat shown without a token")
		}
	})
	t.Run("the zero screen is the login", func(t *testing.T) {
		if (screen{}).isChat() {
			t.Fatal("zero screen is the chat")
		}
	})
}

func TestModelTokens(t *testing.T) {
	// Every login method stores its token before the chat opens with it.
	for _, tt := range []struct {
		name string
		msg  tui.Msg
	}{
		{"password login", password.TokenMsg("t")},
		{"QR login", qr.TokenMsg("t")},
		{"token login", token.TokenMsg("t")},
	} {
		t.Run(tt.name+" stores, then opens the chat", func(t *testing.T) {
			store := testStore(t)
			m, command := newModel(t, store, "").Update(tt.msg)
			if m.screen.isChat() {
				t.Fatal("chat opened before the token was stored")
			}
			msg := run(t, command)
			if got, ok := stored(t, store); !ok || got != "t" {
				t.Fatalf("stored = %q, %v", got, ok)
			}
			if m, _ = m.Update(msg); !m.screen.isChat() || m.dialog != nil {
				t.Fatalf("chat = %v, dialog = %+v", m.screen.isChat(), m.dialog)
			}
		})
	}

	t.Run("a failed store still opens the chat, with the reason", func(t *testing.T) {
		m, command := newModel(t, failingStore(t), "").Update(password.TokenMsg("t"))
		if m, _ = m.Update(run(t, command)); !m.screen.isChat() || m.dialog == nil {
			t.Fatalf("chat = %v, dialog = %+v", m.screen.isChat(), m.dialog)
		}
	})

	t.Run("a refreshed token replaces the stored one", func(t *testing.T) {
		store := testStore(t)
		run(t, storeToken(store, "old"))
		_, command := newModel(t, store, "old").Update(chat.TokenRefreshedMsg("new"))
		if msg := run(t, command); msg != nil {
			t.Fatalf("refresh sent %#v", msg)
		}
		if got, _ := stored(t, store); got != "new" {
			t.Fatalf("stored %q, want the refreshed token", got)
		}
	})

	t.Run("a refresh that fails to store says so and keeps the old token", func(t *testing.T) {
		store := testStore(t)
		run(t, storeToken(store, "old"))
		broken, err := tokenstore.New(store.Path(), "false", "cat")
		if err != nil {
			t.Fatal(err)
		}
		_, command := newModel(t, broken, "old").Update(chat.TokenRefreshedMsg("new"))
		if _, ok := run(t, command).(ui.DialogMsg); !ok {
			t.Fatal("no dialog for the failed store")
		}
		if got, _ := stored(t, store); got != "old" {
			t.Fatalf("stored %q, want the old token kept", got)
		}
	})

	// Logging out and a rejected token (4004) both delete the token and show the login.
	for _, tt := range []struct {
		name       string
		err        error
		wantDialog bool
	}{
		{"logout", nil, false},
		{"rejected token", errors.New("4004"), true},
	} {
		t.Run(tt.name+" deletes the token", func(t *testing.T) {
			store := testStore(t)
			run(t, storeToken(store, "t"))
			m, command := newModel(t, store, "t").Update(chat.LogoutMsg{Err: tt.err})
			runAll(command)
			if m.screen.isChat() || (m.dialog != nil) != tt.wantDialog {
				t.Fatalf("chat = %v, dialog = %+v", m.screen.isChat(), m.dialog)
			}
			if _, err := os.Stat(store.Path()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("token file kept: %v", err)
			}
		})
	}

	t.Run("a closed session quits", func(t *testing.T) {
		if _, command := newModel(t, testStore(t), "").Update(chat.StateClosedMsg{}); command != (cmd.Quit{}) {
			t.Fatalf("command = %#v", command)
		}
	})
}

func TestDeleteToken(t *testing.T) {
	t.Run("a missing token deletes quietly", func(t *testing.T) {
		if msg := run(t, deleteToken(testStore(t))); msg != nil {
			t.Fatalf("got %#v", msg)
		}
	})
	t.Run("a failed delete says so", func(t *testing.T) {
		// A directory in place of the file, with an entry, cannot be removed by os.Remove.
		path := filepath.Join(t.TempDir(), tokenstore.FileName)
		if err := os.MkdirAll(filepath.Join(path, "entry"), 0o700); err != nil {
			t.Fatal(err)
		}
		store, err := tokenstore.New(path, "cat", "cat")
		if err != nil {
			t.Fatal(err)
		}
		dialog, ok := run(t, deleteToken(store)).(ui.DialogMsg)
		if !ok || !strings.Contains(dialog.Text, "delete") {
			t.Fatalf("got %#v", dialog)
		}
	})
}
