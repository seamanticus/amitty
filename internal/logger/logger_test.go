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
