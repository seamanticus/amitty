package consts

import (
	"path/filepath"
	"testing"
)

// evidence: witnesses
func TestStateDir(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for _, tt := range []struct{ name, xdg, want string }{
		{"XDG_STATE_HOME set", "/x/state", "/x/state/amitty"},
		{"unset", "", "/home/u/.local/state/amitty"},
		// The spec says a relative path is invalid and to be ignored.
		{"relative", "state", "/home/u/.local/state/amitty"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", tt.xdg)
			got, err := StateDir()
			if err != nil || got != filepath.FromSlash(tt.want) {
				t.Fatalf("StateDir() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
