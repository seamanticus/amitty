package password

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/seamanticus/amitty/internal/discordtest"
	dhttp "github.com/seamanticus/amitty/internal/http"
)

// evidence: witnesses
func TestRequestFingerprint(t *testing.T) {
	t.Run("gets experiments without a fingerprint or token", func(t *testing.T) {
		var path string
		var sent, auth bool
		client := authClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path = r.Method + " " + r.URL.Path
			_, sent = r.Header["X-Fingerprint"]
			_, auth = r.Header["Authorization"]
			reply(http.StatusOK, `{"fingerprint":"1.x","assignments":[],"installation":"i.x"}`)(w, r)
		})), dhttp.Build{}, "")
		browser := dhttp.NewBrowser(dhttp.Network{})
		if msg := requestFingerprint(t.Context(), client, browser); msg != (FingerprintMsg{fingerprint: "1.x"}) {
			t.Fatalf("got %#v", msg)
		}
		if got := browser.Installation(); got != "i.x" {
			t.Errorf("installation = %q, want the one experiments gave", got)
		}
		if path != "GET /api/v9/experiments" {
			t.Errorf("request = %s", path)
		}
		// Discord only hands out a fingerprint to a request that has neither.
		if sent || auth {
			t.Errorf("X-Fingerprint sent = %v, Authorization sent = %v", sent, auth)
		}
	})

	for _, tt := range []struct {
		name   string
		status int
		body   string
	}{
		{"no fingerprint", http.StatusOK, `{"assignments":[]}`},
		{"empty fingerprint", http.StatusOK, `{"fingerprint":""}`},
		{"server error", http.StatusInternalServerError, `{}`},
		{"rate limited", http.StatusTooManyRequests, `{"retry_after":1}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := authClient(discordtest.Client(t, reply(tt.status, tt.body)), dhttp.Build{}, "")
			if msg := requestFingerprint(t.Context(), client, dhttp.NewBrowser(dhttp.Network{})); !isFailed(msg) {
				t.Fatalf("got %#v, want FailedMsg", msg)
			}
		})
	}

	t.Run("a closed server is retryable once", func(t *testing.T) {
		msg := requestFingerprint(t.Context(), discordtest.ClosedClient(t), dhttp.NewBrowser(dhttp.Network{}))
		if _, ok := countdownOr(msg, false).(CountdownMsg); !ok {
			t.Fatalf("got %#v, want CountdownMsg", msg)
		}
	})
}

// evidence: witnesses
func TestVisitLoginPage(t *testing.T) {
	var got []string
	client := authClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, auth := r.Header["Authorization"]
		got = append(got, fmt.Sprintf("%s %s fp=%s auth=%v len=%d", r.Method, r.URL.Path, r.Header.Get("X-Fingerprint"), auth, r.ContentLength))
		// A failed call must not stop the next.
		w.WriteHeader(http.StatusInternalServerError)
	})), dhttp.Build{}, "1.x")
	visitLoginPage(t.Context(), client)
	// The order, fingerprint and empty body of the web client's login page in a NetLog.
	want := []string{
		"GET /api/v9/auth/location-metadata fp=1.x auth=false len=0",
		"POST /api/v9/auth/conditional/start fp=1.x auth=false len=0",
	}
	if !slices.Equal(got, want) {
		t.Errorf("sent %q, want %q", got, want)
	}
}

func TestModelFingerprint(t *testing.T) {
	ready := func() Model { return Model{login: "u", password: "p"} }

	t.Run("the first submit fetches a fingerprint, then logs in with it", func(t *testing.T) {
		m, command := ready().Update(submitMsg{})
		if command == nil || !m.submitting || m.fingerprint != "" {
			t.Fatalf("command = %v, submitting = %v", command, m.submitting)
		}
		m, command = m.Update(FingerprintMsg{fingerprint: "1.x"})
		if command == nil || m.fingerprint != "1.x" || !m.submitting {
			t.Fatalf("fingerprint = %q, command = %v", m.fingerprint, command)
		}
	})

	t.Run("a fingerprint after the login ended is ignored", func(t *testing.T) {
		m, command := ready().Update(FingerprintMsg{fingerprint: "1.x"})
		if command != nil || m.fingerprint != "" {
			t.Fatalf("fingerprint = %q, command = %v", m.fingerprint, command)
		}
	})

	t.Run("a failed login keeps the fingerprint for the next submit", func(t *testing.T) {
		m, _ := ready().Update(submitMsg{})
		m, _ = m.Update(FingerprintMsg{fingerprint: "1.x"})
		m, _ = m.Update(FailedMsg{errors.New("wrong password")})
		if m.fingerprint != "1.x" {
			t.Fatalf("fingerprint = %q, want kept", m.fingerprint)
		}
	})

	t.Run("the second step repeats the login's fingerprint", func(t *testing.T) {
		m := ready()
		m.fingerprint = "1.x"
		m, _ = m.Update(submitMsg{})
		m, _ = m.Update(MFAMsg{ticket{id: "tk", allowed: [methodCount]bool{methodTOTP: true}}})
		if m.code.ticket.fingerprint != "1.x" {
			t.Fatalf("ticket fingerprint = %q", m.code.ticket.fingerprint)
		}
	})
}
