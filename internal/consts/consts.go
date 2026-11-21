package consts

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const Name = "amitty"

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

// ExpandHome returns path with a leading "~" or "~/" replaced by the home directory, as a shell would; "~user" and other paths are kept.
func ExpandHome(path string) string {
	rest, ok := strings.CutPrefix(path, "~")
	if !ok || rest != "" && rest[0] != '/' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + rest
}

// DownloadDir returns where downloads go: configured, with a leading ~ for the home directory, else $XDG_DOWNLOAD_DIR from the environment or user-dirs.dirs, else ~/Downloads, else the working directory.
func DownloadDir(configured string) string {
	if configured != "" {
		return ExpandHome(configured)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	if dir := os.Getenv("XDG_DOWNLOAD_DIR"); filepath.IsAbs(dir) {
		return dir
	}
	if home == "" {
		return "."
	}
	if config, err := os.UserConfigDir(); err == nil {
		if content, err := os.ReadFile(filepath.Join(config, "user-dirs.dirs")); err == nil {
			if dir, ok := userDirsDownload(string(content), home); ok {
				return dir
			}
		}
	}
	return filepath.Join(home, "Downloads")
}

// userDirsDownload returns the XDG_DOWNLOAD_DIR that a user-dirs.dirs file sets, whose values are "$HOME/..." or absolute.
func userDirsDownload(content, home string) (string, bool) {
	for line := range strings.Lines(content) {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "XDG_DOWNLOAD_DIR=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		if rest, ok := strings.CutPrefix(value, "$HOME"); ok {
			value = home + rest
		}
		if filepath.IsAbs(value) {
			return filepath.Clean(value), true
		}
	}
	return "", false
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
