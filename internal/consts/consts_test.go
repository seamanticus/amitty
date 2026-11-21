package consts

import (
	"os"
	"path/filepath"
	"testing"
)

// evidence: witnesses
func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	for _, tt := range []struct{ path, want string }{
		{"~", "/home/u"},
		{"~/a.wav", "/home/u/a.wav"},
		{"~v/a.wav", "~v/a.wav"},
		{"/x/~/a", "/x/~/a"},
		{"rel", "rel"},
		{"", ""},
	} {
		if got := ExpandHome(tt.path); got != tt.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// evidence: witnesses
func TestDownloadDir(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	for _, tt := range []struct{ name, configured, env, userDirs, want string }{
		{"configured", "/x/dl", "/env", "", "/x/dl"},
		{"configured from home", "~/dl", "", "", "/home/u/dl"},
		{"configured ~user is literal", "~v/dl", "", "", "~v/dl"},
		{"environment", "", "/env", `XDG_DOWNLOAD_DIR="/file"`, "/env"},
		{"relative environment is ignored", "", "env", "", "/home/u/Downloads"},
		{"user-dirs.dirs", "", "", "# c\nXDG_DESKTOP_DIR=\"$HOME/Desktop\"\nXDG_DOWNLOAD_DIR=\"$HOME/Fetched\"\n", "/home/u/Fetched"},
		{"user-dirs.dirs absolute", "", "", `XDG_DOWNLOAD_DIR="/data/dl"`, "/data/dl"},
		{"user-dirs.dirs relative is ignored", "", "", `XDG_DOWNLOAD_DIR="dl"`, "/home/u/Downloads"},
		{"fallback", "", "", "", "/home/u/Downloads"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_DOWNLOAD_DIR", tt.env)
			if err := os.WriteFile(filepath.Join(config, "user-dirs.dirs"), []byte(tt.userDirs), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := DownloadDir(tt.configured); got != filepath.FromSlash(tt.want) {
				t.Fatalf("DownloadDir(%q) = %q; want %q", tt.configured, got, tt.want)
			}
		})
	}
}
