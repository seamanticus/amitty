package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// evidence: witnesses
func TestIsAuthenticationFailed(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{
		{"4004", &ws.CloseEvent{Code: 4004}, true},
		{"4004 wrapped", fmt.Errorf("open: %w", &ws.CloseEvent{Code: 4004}), true},
		{"4004 joined", errors.Join(errors.New("x"), &ws.CloseEvent{Code: 4004}), true},
		// Only a rejected token logs out: these must keep the stored one.
		{"other fatal code", &ws.CloseEvent{Code: 4014}, false},
		{"rate limited", &ws.CloseEvent{Code: 4008}, false},
		{"no close code", &ws.CloseEvent{Code: -1}, false},
		{"normal closure", &ws.CloseEvent{Code: 1000}, false},
		{"deadline", context.DeadlineExceeded, false},
		{"canceled", context.Canceled, false},
		{"plain error saying 4004", errors.New("4004"), false},
		{"nil", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAuthenticationFailed(tt.err); got != tt.want {
				t.Fatalf("isAuthenticationFailed(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	m := Model{token: "old"}
	for _, same := range []string{"", "old"} {
		if command := m.refreshToken(same); command != nil {
			t.Fatalf("token %q asked to be stored", same)
		}
	}
	task, ok := m.refreshToken("new").(cmd.Task)
	if !ok || task() != TokenRefreshedMsg("new") || m.token != "new" {
		t.Fatalf("ok = %v, token = %q", ok, m.token)
	}
	if m.refreshToken("new") != nil {
		t.Fatal("the same refresh asked to be stored twice")
	}
}

func TestCloseState(t *testing.T) {
	next := LogoutMsg{Err: errors.New("x")}
	stopped := false
	task, ok := closeState(nil, func() { stopped = true }, next).(cmd.Task)
	if !ok {
		t.Fatal("closeState is not a task")
	}
	if got := task(); got != next || !stopped {
		t.Fatalf("without a session it sent %#v and stopped = %v, want %#v and the events stopped", got, stopped, next)
	}
}

// evidence: witnesses
func TestLogOut(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"ended", http.StatusNoContent, false},
		{"refused", http.StatusUnauthorized, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var sent string
			client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				sent = r.Method + " " + r.URL.Path + " " + string(body)
				w.WriteHeader(tt.status)
			}))
			stopped := false
			task, ok := logOut(nil, func() { stopped = true }, client).(cmd.Task)
			if !ok {
				t.Fatal("logOut is not a task")
			}
			msg, ok := task().(LogoutMsg)
			if !ok || !stopped || (msg.Err != nil) != tt.wantErr || tt.wantErr && msg.What == "" {
				t.Fatalf("got %#v, stopped = %v", msg, stopped)
			}
			// A logout Discord did not confirm says how to end the session by hand.
			if tt.wantErr && !strings.Contains(msg.Err.Error(), "Devices") {
				t.Errorf("error %q does not say where to end the session", msg.Err)
			}
			// The body the web client's logout sends, 38 bytes in a NetLog.
			if want := `POST /api/v9/auth/logout {"provider":null,"voip_provider":null}`; sent != want {
				t.Errorf("sent %q, want %q", sent, want)
			}
		})
	}
}
