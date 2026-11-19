package password

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/seamanticus/discordo/internal/discordtest"
	dhttp "github.com/seamanticus/discordo/internal/http"
)

// proof: exhaustive
func TestMethod(t *testing.T) {
	seen := map[string]bool{}
	for m := range methodCount {
		if m.label() == "" || m.path() == "" || seen[m.path()] {
			t.Fatalf("method %d: label %q, path %q", m, m.label(), m.path())
		}
		seen[m.path()] = true
	}
}

// proof: exhaustive
func TestTicketNext(t *testing.T) {
	for set := range 1 << methodCount {
		var tk ticket
		for m := range methodCount {
			tk.allowed[m] = set&(1<<m) != 0
		}
		for m := range methodCount {
			for _, step := range []method{1, methodCount - 1} {
				got := tk.next(m, step)
				if set != 0 && !tk.allowed[got] {
					t.Fatalf("set %03b: next(%d, %d) = %d, not allowed", set, m, step, got)
				}
				if set == 0 && got != m {
					t.Fatalf("empty set: next(%d, %d) = %d, want %d", m, step, got, m)
				}
			}
			// Forward then backward returns to an allowed start.
			if tk.allowed[m] && tk.next(tk.next(m, 1), methodCount-1) != m {
				t.Fatalf("set %03b: next is not undone from %d", set, m)
			}
		}
		if set != 0 && !tk.allowed[tk.first()] {
			t.Fatalf("set %03b: first() = %d, not allowed", set, tk.first())
		}
	}
}

// evidence: witnesses
func TestNormalizeCode(t *testing.T) {
	for in, want := range map[string]string{"": "", "123456": "123456", "abcd-efgh": "abcdefgh", " 123 456 ": "123456", "--": ""} {
		if got := normalizeCode(in); got != want {
			t.Errorf("normalizeCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// evidence: witnesses
func TestRequestCode(t *testing.T) {
	tk := ticket{id: "tk", instance: "li", allowed: [methodCount]bool{true, true, true}}

	for m := range methodCount {
		t.Run("posts to the "+m.path()+" endpoint", func(t *testing.T) {
			var path string
			var body struct {
				Code            string `json:"code"`
				Ticket          string `json:"ticket"`
				LoginInstanceID string `json:"login_instance_id"`
			}
			client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var b bytes.Buffer
				_, _ = b.ReadFrom(r.Body)
				path = r.Method + " " + r.URL.Path
				_ = json.Unmarshal(b.Bytes(), &body)
				reply(http.StatusOK, `{"token":"t"}`)(w, r)
			}))
			if msg := requestCode(t.Context(), client, tk, m, "abcd-efgh"); msg != TokenMsg("t") {
				t.Fatalf("got %#v", msg)
			}
			if want := "POST /api/v9/auth/mfa/" + m.path(); path != want {
				t.Errorf("request = %s, want %s", path, want)
			}
			if body.Code != "abcdefgh" || body.Ticket != "tk" || body.LoginInstanceID != "li" {
				t.Errorf("body = %+v", body)
			}
		})
	}

	for _, tt := range []struct {
		name        string
		status      int
		body        string
		wantExpired bool
	}{
		{"wrong code", http.StatusBadRequest, `{"code":60008,"message":"Invalid two-factor code"}`, false},
		{"invalid ticket", http.StatusBadRequest, `{"code":60006,"message":"Invalid two-factor auth ticket"}`, true},
		{"invalid session", http.StatusBadRequest, `{"code":60009,"message":"Invalid session"}`, true},
		{"rate limited", http.StatusTooManyRequests, `{"message":"You are being rate limited.","retry_after":1}`, false},
		{"no token", http.StatusOK, `{}`, false},
		{"null", http.StatusOK, `null`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			client := authClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				reply(tt.status, tt.body)(w, r)
			})), dhttp.Build{}, "")
			failed, ok := requestCode(t.Context(), client, tk, methodTOTP, "123456").(FailedMsg)
			if !ok || failed.Err == nil {
				t.Fatalf("got %#v, want FailedMsg", failed)
			}
			if _, expired := errors.AsType[expiredError](failed.Err); expired != tt.wantExpired {
				t.Fatalf("expired = %v, want %v: %v", expired, tt.wantExpired, failed.Err)
			}
			if n := calls.Load(); n != 1 {
				t.Fatalf("code posted %d times, want 1", n)
			}
		})
	}

	t.Run("a closed server is retryable once", func(t *testing.T) {
		msg := countdownOr(requestCode(t.Context(), discordtest.ClosedClient(t), tk, methodTOTP, "1"), false)
		if _, ok := msg.(CountdownMsg); !ok {
			t.Fatalf("got %#v, want CountdownMsg", msg)
		}
		if msg := countdownOr(requestCode(t.Context(), discordtest.ClosedClient(t), tk, methodTOTP, "1"), true); !isFailed(msg) {
			t.Fatalf("final attempt: got %#v, want FailedMsg", msg)
		}
	})
}

// evidence: witnesses
func TestRequestSMS(t *testing.T) {
	tk := ticket{id: "tk", allowed: [methodCount]bool{methodSMS: true}}

	t.Run("sends the ticket and reports the phone", func(t *testing.T) {
		var path string
		var body struct {
			Ticket string `json:"ticket"`
		}
		client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var b bytes.Buffer
			_, _ = b.ReadFrom(r.Body)
			path = r.Method + " " + r.URL.Path
			_ = json.Unmarshal(b.Bytes(), &body)
			reply(http.StatusOK, `{"phone":"+*******1234"}`)(w, r)
		}))
		if msg := requestSMS(t.Context(), client, tk); msg != (SMSSentMsg{"+*******1234"}) {
			t.Fatalf("got %#v", msg)
		}
		if path != "POST /api/v9/auth/mfa/sms/send" || body.Ticket != "tk" {
			t.Errorf("request = %s, body = %+v", path, body)
		}
	})

	t.Run("an invalid ticket expires the step", func(t *testing.T) {
		client := discordtest.Client(t, reply(http.StatusBadRequest, `{"code":60006}`))
		failed, ok := requestSMS(t.Context(), client, tk).(FailedMsg)
		if _, expired := errors.AsType[expiredError](failed.Err); !ok || !expired {
			t.Fatalf("got %#v, want expired", failed)
		}
	})

	t.Run("is sent once", func(t *testing.T) {
		var calls atomic.Int32
		client := authClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusBadGateway)
		})), dhttp.Build{}, "")
		if msg := requestSMS(t.Context(), client, tk); !isFailed(msg) {
			t.Fatalf("got %#v, want FailedMsg", msg)
		}
		if n := calls.Load(); n != 1 {
			t.Fatalf("SMS requested %d times, want 1", n)
		}
	})
}

func isFailed(msg any) bool {
	failed, ok := msg.(FailedMsg)
	return ok && failed.Err != nil
}
