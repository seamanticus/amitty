package logger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/seamanticus/amitty/internal/consts"
)

const fileName = "logs.txt"

// LevelRaw is below debug: it also logs the full body of every API response and gateway event, which hold personal data; credentials in them are blanked.
const LevelRaw = slog.LevelDebug - 4

// ParseLevel returns the level named by -log-level: "raw-debug", or one slog knows such as "debug" or "info".
func ParseLevel(name string) (slog.Level, error) {
	if name == "raw-debug" {
		return LevelRaw, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(name)); err != nil {
		return 0, fmt.Errorf("invalid log level %q: %w", name, err)
	}
	return level, nil
}

// secretField matches a JSON string field that holds a credential: a token of any kind, a login ticket or a password.
var secretField = regexp.MustCompile(`"(token|[a-z_]*_token|ticket|password)"(\s*:\s*)"(?:[^"\\]|\\.)*"`)

// redact blanks the value of every credential field in body.
func redact(body []byte) []byte {
	return secretField.ReplaceAll(body, []byte(`"$1"$2"<redacted>"`))
}

// Raw logs the full body of a response or event at LevelRaw, if the logger takes that level, with credentials blanked.
func Raw(ctx context.Context, msg string, body []byte, attrs ...any) {
	if !slog.Default().Enabled(ctx, LevelRaw) {
		return
	}
	slog.Log(ctx, LevelRaw, msg, append(attrs, "body", string(redact(body)))...)
}

func DefaultPath() string {
	return filepath.Join(consts.CacheDir(), fileName)
}

// Load opens the log file, readable by the user alone, and configures default logger.
func Load(path string, level slog.Level) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	// OpenFile keeps the mode of a file that already exists.
	if err := file.Chmod(0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("restrict log file: %w", err), file.Close())
	}

	opts := &slog.HandlerOptions{Level: level, ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		// slog names a level below debug "DEBUG-4".
		if attr.Key == slog.LevelKey && attr.Value.Any() == LevelRaw {
			attr.Value = slog.StringValue("RAW")
		}
		return attr
	}}
	handler := slog.NewTextHandler(file, opts)
	slog.SetDefault(slog.New(handler))
	return file, nil
}
