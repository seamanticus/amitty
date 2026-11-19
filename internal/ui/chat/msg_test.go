package chat

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ayn2op/arikawa/v3/utils/ws"
	"github.com/ayn2op/tview/cmd"
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
	task, ok := closeState(nil, next).(cmd.Task)
	if !ok {
		t.Fatal("closeState is not a task")
	}
	if got := task(); got != next {
		t.Fatalf("without a session it sent %#v, want %#v", got, next)
	}
}
