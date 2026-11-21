package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		name string
		want slog.Level
		ok   bool
	}{
		{"raw-debug", LevelRaw, true},
		{"debug", slog.LevelDebug, true},
		{"info", slog.LevelInfo, true},
		{"error", slog.LevelError, true},
		{"", 0, false},
		{"raw", 0, false},
	} {
		got, err := ParseLevel(test.name)
		if (err == nil) != test.ok || got != test.want {
			t.Errorf("ParseLevel(%q) = %v, %v", test.name, got, err)
		}
	}
}

func TestRedact(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct{ body, want string }{
		{`{"token":"a.b.c","user_settings":{}}`, `{"token":"<redacted>","user_settings":{}}`},
		{`{"ticket": "x\"y", "mfa":true}`, `{"ticket": "<redacted>", "mfa":true}`},
		{`{"analytics_token":"t","auth_token":"u"}`, `{"analytics_token":"<redacted>","auth_token":"<redacted>"}`},
		{`{"password":"p"}`, `{"password":"<redacted>"}`},
		{`{"token":null,"tokens":"kept","name":"token"}`, `{"token":null,"tokens":"kept","name":"token"}`},
		{``, ``},
	} {
		if got := string(redact([]byte(test.body))); got != test.want {
			t.Errorf("redact(%s) = %s, want %s", test.body, got, test.want)
		}
	}
}

func TestLoadRestrictsTheFile(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	path := filepath.Join(t.TempDir(), "logs.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := Load(path, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %v, want 0600", mode)
	}
}

func TestRaw(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, test := range []struct {
		level slog.Level
		want  bool
	}{{LevelRaw, true}, {slog.LevelDebug, false}} {
		path := filepath.Join(t.TempDir(), "logs.txt")
		file, err := Load(path, test.level)
		if err != nil {
			t.Fatal(err)
		}
		Raw(t.Context(), "api response", []byte(`{"id":"1"}`), "status", 200)
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		logged := strings.Contains(string(data), `level=RAW msg="api response" status=200 body="{\"id\":\"1\"}"`)
		if logged != test.want {
			t.Errorf("at %v the log is %q", test.level, data)
		}
	}
}
