package root

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/seamanticus/discordo/internal/config"
	"github.com/seamanticus/discordo/internal/http"
	"github.com/seamanticus/discordo/internal/ui/chat"
	"github.com/seamanticus/discordo/internal/ui/login/password"
	"github.com/seamanticus/discordo/internal/ui/login/qr"
	"github.com/seamanticus/discordo/internal/ui/login/token"
	"github.com/zalando/go-keyring"
)

func run(t *testing.T, command tview.Cmd) tview.Msg {
	t.Helper()
	task, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command = %#v, want a task", command)
	}
	return task()
}

func stored() (string, error) { return keyring.Get(keyringService, keyringUser) }

// evidence: witnesses
func TestLoadToken(t *testing.T) {
	for _, tt := range []struct {
		name, env  string
		keyringErr error
		saved      string
		want       tview.Msg
	}{
		{"the environment wins over the keyring", "env-token", nil, "saved", tokenMsg("env-token")},
		{"the environment is used when the keyring fails", "env-token", errors.New("locked"), "", tokenMsg("env-token")},
		{"the keyring is used without the environment", "", nil, "saved", tokenMsg("saved")},
		{"no token anywhere asks for a login", "", nil, "", loginMsg{}},
		{"a failing keyring asks for a login", "", errors.New("no secret service"), "", loginMsg{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			keyring.MockInit()
			if tt.saved != "" {
				if err := keyring.Set(keyringService, keyringUser, tt.saved); err != nil {
					t.Fatal(err)
				}
			}
			if tt.keyringErr != nil {
				keyring.MockInitWithError(tt.keyringErr)
			}
			t.Setenv(tokenEnvVarKey, tt.env)
			if got := run(t, loadToken()); got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestKeyring(t *testing.T) {
	t.Run("a stored login token opens the chat and is read back", func(t *testing.T) {
		keyring.MockInit()
		if msg := run(t, storeToken("abc")); msg != tokenMsg("abc") {
			t.Fatalf("storeToken sent %#v", msg)
		}
		if got := run(t, getToken()); got != tokenMsg("abc") {
			t.Fatalf("got %#v", got)
		}
	})
	t.Run("a keyring that fails to store still opens the chat", func(t *testing.T) {
		keyring.MockInitWithError(errors.New("locked"))
		if msg := run(t, storeToken("abc")); msg != tokenMsg("abc") {
			t.Fatalf("storeToken sent %#v", msg)
		}
	})
	t.Run("delete removes it", func(t *testing.T) {
		keyring.MockInit()
		run(t, storeToken("abc"))
		if msg := run(t, deleteToken()); msg != nil {
			t.Fatalf("deleteToken sent %#v", msg)
		}
		if _, err := stored(); !errors.Is(err, keyring.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("deleting fails quietly", func(t *testing.T) {
		for _, init := range []func(){keyring.MockInit, func() { keyring.MockInitWithError(errors.New("locked")) }} {
			init()
			if msg := run(t, deleteToken()); msg != nil {
				t.Fatalf("deleteToken sent %#v", msg)
			}
		}
	})
}

// runAll runs every task in command, batches flattened, and returns their messages.
func runAll(command tview.Cmd) []tview.Msg {
	switch c := command.(type) {
	case cmd.Batch:
		var all []tview.Msg
		for _, inner := range c {
			all = append(all, runAll(inner)...)
		}
		return all
	case cmd.Task:
		return []tview.Msg{c()}
	}
	return nil
}

func TestModelTokens(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a stored or environment token opens the chat without storing", func(t *testing.T) {
		keyring.MockInit()
		m, _ := NewModel(cfg, http.Build{}).Update(tokenMsg("t"))
		if m.screen != screenChat {
			t.Fatalf("screen = %d", m.screen)
		}
		if got, err := stored(); err == nil {
			t.Fatalf("stored %q, want nothing", got)
		}
	})

	t.Run("no token shows the login", func(t *testing.T) {
		if m, _ := NewModel(cfg, http.Build{}).Update(loginMsg{}); m.screen != screenLogin {
			t.Fatalf("screen = %d", m.screen)
		}
	})

	// Every login method stores its token before the chat opens with it.
	for _, tt := range []struct {
		name string
		msg  tview.Msg
	}{
		{"password login", password.TokenMsg("t")},
		{"QR login", qr.TokenMsg("t")},
		{"token login", token.TokenMsg("t")},
	} {
		t.Run(tt.name+" stores, then opens the chat", func(t *testing.T) {
			keyring.MockInit()
			m, command := NewModel(cfg, http.Build{}).Update(tt.msg)
			if m.screen == screenChat {
				t.Fatal("chat opened before the token was stored")
			}
			if msg := run(t, command); msg != tokenMsg("t") {
				t.Fatalf("store sent %#v, want the token on to the chat", msg)
			}
			if got, err := stored(); err != nil || got != "t" {
				t.Fatalf("stored = %q, %v", got, err)
			}
		})
	}

	t.Run("logout deletes the token and shows the login", func(t *testing.T) {
		keyring.MockInit()
		run(t, storeToken("t"))
		m, command := NewModel(cfg, http.Build{}).Update(chat.LogoutMsg{})
		runAll(command)
		if m.screen != screenLogin || m.dialog != nil {
			t.Fatalf("screen = %d, dialog = %+v", m.screen, m.dialog)
		}
		if _, err := stored(); !errors.Is(err, keyring.ErrNotFound) {
			t.Fatalf("token kept after logout: err = %v", err)
		}
	})

	t.Run("a rejected token logs out with the reason over the login", func(t *testing.T) {
		keyring.MockInit()
		m, _ := NewModel(cfg, http.Build{}).Update(chat.LogoutMsg{Err: errors.New("4004")})
		if m.screen != screenLogin || m.dialog == nil {
			t.Fatalf("screen = %d, dialog = %+v", m.screen, m.dialog)
		}
	})

	t.Run("a closed session quits", func(t *testing.T) {
		if _, command := NewModel(cfg, http.Build{}).Update(chat.StateClosedMsg{}); command != (cmd.Quit{}) {
			t.Fatalf("command = %#v", command)
		}
	})
}
