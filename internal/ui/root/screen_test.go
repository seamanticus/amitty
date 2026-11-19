package root

import (
	"testing"

	"github.com/seamanticus/discordo/internal/ui/chat"
	"github.com/seamanticus/discordo/internal/ui/login"
)

// proof: exhaustive
func TestScreenMatch(t *testing.T) {
	which := func(s screen) string {
		return s.Match(func(login.Model) string { return "login" }, func(chat.Model) string { return "chat" })
	}
	for _, tt := range []struct {
		screen screen
		want   string
	}{
		{screen{}, "login"},
		{loginScreen(login.Model{}), "login"},
		{chatScreen(chat.Model{}), "chat"},
	} {
		if got := which(tt.screen); got != tt.want {
			t.Errorf("Match = %s, want %s", got, tt.want)
		}
	}
}
