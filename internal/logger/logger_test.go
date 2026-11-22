package logger

import (
	"encoding/json/jsontext"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
		{`{"captcha_key":"k","captcha_rqtoken":"r","captcha_sitekey":"s"}`, `{"captcha_key":"<redacted>","captcha_rqtoken":"<redacted>","captcha_sitekey":"s"}`},
		{`{"op":"nonce_proof","encrypted_nonce":"n","proof":"p","fingerprint":"f","encoded_public_key":"k"}`, `{"op":"nonce_proof","encrypted_nonce":"<redacted>","proof":"<redacted>","fingerprint":"<redacted>","encoded_public_key":"<redacted>"}`},
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
		Raw(t.Context(), "api exchange", []byte(`{"id":"1","token":"t"}`), "status", 200)
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		logged := strings.Contains(string(data), `"level":"RAW","msg":"api exchange","status":200,"body":{"id":"1","token":"<redacted>"}}`)
		if logged != test.want {
			t.Errorf("at %v the log is %q", test.level, data)
		}
	}
}

func TestBody(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		body []byte
		want any
	}{
		{nil, ""},
		{[]byte(`{"password":"p"}`), jsontext.Value(`{"password":"<redacted>"}`)},
		{[]byte(`not json "ticket":"x"`), `not json "ticket":"<redacted>"`},
		{[]byte{0xff, 0xfe, 0}, "<3 bytes>"},
	} {
		got := Body(test.body).Any()
		if fmt.Sprint(got) != fmt.Sprint(test.want) || fmt.Sprintf("%T", got) != fmt.Sprintf("%T", test.want) {
			t.Errorf("Body(%q) = %T %v, want %T %v", test.body, got, got, test.want, test.want)
		}
	}
}

func TestHeaders(t *testing.T) {
	header := map[string][]string{
		"Header-Order:":      {"x"},
		"Content-Type":       {"application/json"},
		"Authorization":      {"secret"},
		"X-Super-Properties": {"e30="},
		"Accept":             {"*/*"},
		"Cookie":             {""},
	}
	var got []string
	for _, attr := range Headers(header, []string{"authorization", "x-super-properties", "absent"}).Group() {
		got = append(got, attr.Key+"="+attr.Value.String())
	}
	want := []string{"authorization=<redacted>", "x-super-properties=e30=", "accept=*/*", "content-type=application/json", "cookie="}
	if !slices.Equal(got, want) {
		t.Errorf("Headers = %q, want %q", got, want)
	}
}
