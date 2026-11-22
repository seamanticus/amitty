package logger

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seamanticus/amitty/internal/consts"
)

const fileName = "logs.txt"

// LevelRaw is below debug: it also logs every API exchange and gateway frame in full, which hold personal data; credentials in them are blanked.
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

// secretField matches a JSON string field that holds a credential or a one-time login value: a token of any kind, a login ticket, a password, a captcha answer, or a QR login's keys and fingerprint.
var secretField = regexp.MustCompile(`"(token|[a-z_]*_token|ticket|password|captcha_key|captcha_rqtoken|captcha_rqdata|fingerprint|encrypted_[a-z_]*|proof|encoded_public_key)"(\s*:\s*)"(?:[^"\\]|\\.)*"`)

const redacted = "<redacted>"

// redact blanks the value of every credential field in body.
func redact(body []byte) []byte {
	return secretField.ReplaceAll(body, []byte(`"$1"$2"`+redacted+`"`))
}

// Raw logs body in full at LevelRaw, if the logger takes that level, with credentials blanked.
func Raw(ctx context.Context, msg string, body []byte, attrs ...any) {
	if !slog.Default().Enabled(ctx, LevelRaw) {
		return
	}
	slog.Log(ctx, LevelRaw, msg, append(attrs, slog.Any("body", Body(body)))...)
}

// Body returns body with credentials blanked, as JSON in the log when it is JSON, as a string when it is other text, else as its size.
func Body(body []byte) slog.Value {
	switch {
	case len(body) == 0:
		return slog.StringValue("")
	case jsontext.Value(body).IsValid():
		return slog.AnyValue(jsontext.Value(redact(body)))
	case utf8.Valid(body):
		return slog.StringValue(string(redact(body)))
	}
	return slog.StringValue(fmt.Sprintf("<%d bytes>", len(body)))
}

// secretHeader reports whether the header named name, in lower case, holds a credential.
func secretHeader(name string) bool {
	switch name {
	case "authorization", "cookie", "set-cookie", "x-captcha-key", "x-captcha-rqtoken", "x-fingerprint":
		return true
	}
	return false
}

// Headers returns header as a log group with names in lower case: those in order first, in that order, then the rest sorted. Credentials are blanked, their names kept.
func Headers(header map[string][]string, order []string) slog.Value {
	byName := make(map[string][]string, len(header))
	for name, values := range header {
		// fhttp keeps its ordering keys, which end in ':', among the headers; HTTP/2 pseudo-headers start with one.
		if strings.HasSuffix(name, ":") {
			continue
		}
		lower := strings.ToLower(name)
		byName[lower] = append(byName[lower], values...)
	}
	attrs := make([]slog.Attr, 0, len(byName))
	add := func(name string) {
		values, ok := byName[name]
		if !ok {
			return
		}
		delete(byName, name)
		value := strings.Join(values, ", ")
		if value != "" && secretHeader(name) {
			value = redacted
		}
		attrs = append(attrs, slog.String(name, value))
	}
	for _, name := range order {
		add(strings.ToLower(name))
	}
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		add(name)
	}
	return slog.GroupValue(attrs...)
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
	handler := slog.NewJSONHandler(file, opts)
	slog.SetDefault(slog.New(handler))
	return file, nil
}
