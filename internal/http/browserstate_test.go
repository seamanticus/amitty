package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// evidence: witnesses
func TestOpenBrowserKeepsState(t *testing.T) {
	var cookies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookies = append(cookies, r.Header.Get("Cookie"))
		http.SetCookie(w, &http.Cookie{Name: "__dcfduid", Value: "d", Path: "/", MaxAge: 3600})
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "s", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "gone", Value: "g", Path: "/", Expires: time.Unix(1, 0)})
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	path := filepath.Join(t.TempDir(), "credentials", BrowserFileName)

	first := OpenBrowser(Network{}, path)
	if err := first.Client("").FastRequest(http.MethodGet, server.URL+"/api/v9/experiments"); err != nil {
		t.Fatal(err)
	}
	first.SetInstallation("i.x")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("browser file: %v, %v; want it 0600", info, err)
	}

	// The next run sends what the first was given, the expired cookie left out.
	second := OpenBrowser(Network{}, path)
	if got := second.Installation(); got != "i.x" {
		t.Errorf("installation %q, want i.x", got)
	}
	if err := second.Client("").FastRequest(http.MethodGet, server.URL+"/api/v9/experiments"); err != nil {
		t.Fatal(err)
	}
	if cookies[0] != "" || cookies[1] != "__dcfduid=d; session=s" && cookies[1] != "session=s; __dcfduid=d" {
		t.Errorf("cookies sent %q, want none, then those kept", cookies)
	}
}

// evidence: witnesses
func TestBrowserStateExpiry(t *testing.T) {
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name   string
		cookie fhttp.Cookie
		want   bool
	}{
		{"session", fhttp.Cookie{}, true},
		{"later", fhttp.Cookie{Expires: now.Add(time.Second)}, true},
		{"now", fhttp.Cookie{Expires: now}, false},
		{"earlier", fhttp.Cookie{Expires: now.Add(-time.Second)}, false},
		{"deleted", fhttp.Cookie{MaxAge: -1}, false},
	} {
		if got := live(&tt.cookie, now); got != tt.want {
			t.Errorf("%s: live = %v, want %v", tt.name, got, tt.want)
		}
	}

	b := NewBrowser(Network{})
	data := `{"installation":"i.x","cookies":[{"host":"discord.com","name":"a","value":"1","path":"/"},{"host":"discord.com","name":"b","value":"2","path":"/","expires":"2026-11-21T00:00:00Z"}]}`
	if err := b.restore([]byte(data), now); err != nil {
		t.Fatal(err)
	}
	got := b.state(now)
	if got.Installation != "i.x" || len(got.Cookies) != 1 || got.Cookies[0].Name != "a" || got.Cookies[0].Host != "discord.com" {
		t.Errorf("restored %+v, want installation i.x and cookie a alone", got)
	}
}

// evidence: witnesses
func TestOpenBrowserUnreadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), BrowserFileName)
	if b := OpenBrowser(Network{}, path); b.Installation() != "" {
		t.Errorf("a missing file gave installation %q", b.Installation())
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := OpenBrowser(Network{}, path)
	if b.Installation() != "" {
		t.Errorf("a broken file gave installation %q", b.Installation())
	}
	// A broken file is replaced on the first change.
	b.SetInstallation("i.x")
	if again := OpenBrowser(Network{}, path); again.Installation() != "i.x" {
		t.Errorf("installation after replacing %q, want i.x", again.Installation())
	}
}
