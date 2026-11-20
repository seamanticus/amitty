package consts

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const Name = "amitty"

// StateDir returns $XDG_STATE_HOME/amitty, or ~/.local/state/amitty when that is unset or relative, as the XDG base directory spec says.
func StateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, Name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("state dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", Name), nil
}

// DataDir returns $XDG_DATA_HOME/amitty, or ~/.local/share/amitty when that is unset or relative, as the XDG base directory spec says.
func DataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, Name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("data dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", Name), nil
}

// CacheDir returns the cache directory, creating it on first use.
var CacheDir = sync.OnceValue(func() string {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		userCacheDir = os.TempDir()
		slog.Warn("failed to get user cache dir; falling back to temp dir", "err", err, "path", userCacheDir)
	}

	path := filepath.Join(userCacheDir, Name)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		slog.Error("failed to create cache dir", "err", err, "path", path)
	}

	return path
})
