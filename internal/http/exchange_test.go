package http

import (
	"encoding/json/v2"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/seamanticus/amitty/internal/logger"
)

// rawLog logs at LevelRaw to a file for the test, and returns its records once the test's requests are done.
func rawLog(t *testing.T) func() []map[string]any {
	t.Helper()
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	path := filepath.Join(t.TempDir(), "logs.txt")
	file, err := logger.Load(path, logger.LevelRaw)
	if err != nil {
		t.Fatal(err)
	}
	return func() []map[string]any {
		t.Helper()
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		var records []map[string]any
		for line := range strings.Lines(string(data)) {
			var record map[string]any
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				t.Fatalf("log line %q: %v", line, err)
			}
			if record["msg"] == "api exchange" {
				records = append(records, record)
			}
		}
		return records
	}
}

func TestLogURL(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct{ in, want string }{
		{"https://discord.com/api/v9/users/@me/channels", "https://discord.com/api/v9/users/@me/channels"},
		{"https://discord.com/api/v9/channels/1/messages?limit=50&before=2", "https://discord.com/api/v9/channels/1/messages?limit=50&before=2"},
		{"https://up.example/u/a.png?upload_id=u&X-Goog-Signature=s", "https://up.example/u/a.png?upload_id=u&X-Goog-Signature=s"},
		{"https://h/x?token=t&a=1", "https://h/x?a=1&token=%3Credacted%3E"},
		{"http://user:pass@h/", "http://user:xxxxx@h/"},
	} {
		u, err := url.Parse(test.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := logURL(u); got != test.want {
			t.Errorf("logURL(%s) = %s, want %s", test.in, got, test.want)
		}
	}
}

func TestExchangeLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Set-Cookie", "__dcfduid=x")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"captcha_key":["captcha-required"],"captcha_rqtoken":"r"}`)
	}))
	t.Cleanup(server.Close)
	client := Network{}.TLSClient()
	send := func(keepBody bool) {
		req, err := fhttp.NewRequestWithContext(t.Context(), fhttp.MethodPost, server.URL+"/api/v9/auth/login", strings.NewReader(`{"login":"a","password":"p"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "secret")
		req.Header.Set("Content-Type", "application/json")
		orderHeaders(req.Header, []string{"content-type", "authorization"})
		resp, err := do(client, req, keepBody)
		if err != nil {
			t.Fatal(err)
		}
		// Closed unread, as FastRequest does.
		_ = resp.Body.Close()
		_ = resp.Body.Close()
	}

	records := rawLog(t)
	send(true)
	send(false)
	req, err := fhttp.NewRequestWithContext(t.Context(), fhttp.MethodGet, "http://127.0.0.1:1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := do(client, req, true); err == nil {
		t.Fatal("a request to a closed port passed")
	}
	got := records()
	if len(got) != 3 {
		t.Fatalf("%d records, want 3: %v", len(got), got)
	}

	encode := func(v any) string {
		b, err := json.Marshal(v, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for i, want := range []struct {
		request  []string
		response string
	}{
		{
			[]string{`{"body":{"login":"a","password":"<redacted>"},`, `"authorization":"<redacted>"`, `"content-length":"28"`},
			`{"body":{"captcha_key":["captcha-required"],"captcha_rqtoken":"r"},`,
		},
		{[]string{`{"body":"<28 bytes>",`}, `"set-cookie":"__dcfduid=x"`},
	} {
		record := got[i]
		if record["status"] != 400.0 || record["method"] != "POST" {
			t.Errorf("record %d: %v", i, record)
		}
		// The transport's own content-length shows the headers are the wire's, not only those amitty set.
		request := encode(record["request"])
		for j, part := range want.request {
			if j == 0 && !strings.HasPrefix(request, part) || !strings.Contains(request, part) {
				t.Errorf("record %d request %s, want %s in it", i, request, part)
			}
		}
		if response := encode(record["response"]); !strings.Contains(response, want.response) {
			t.Errorf("record %d response %s, want %s", i, response, want.response)
		}
	}
	if failed := got[2]; failed["error"] == nil || failed["status"] != nil || failed["response"] != nil {
		t.Errorf("failed request logged as %v", failed)
	}
}
