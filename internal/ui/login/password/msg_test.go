package password

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// reply answers with status and a JSON body.
func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// evidence: witnesses
func TestRequestLogin(t *testing.T) {
	const path = "/api/v9/auth/login"

	t.Run("posts the credentials as JSON to the login endpoint", func(t *testing.T) {
		var got struct{ method, path, contentType, body string }
		var auth bool
		client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body bytes.Buffer
			_, _ = body.ReadFrom(r.Body)
			got.method, got.path, got.contentType, got.body = r.Method, r.URL.Path, r.Header.Get("Content-Type"), body.String()
			_, auth = r.Header["Authorization"]
			reply(http.StatusOK, `{"token":"t"}`)(w, r)
		}))
		requestLogin(t.Context(), client, "user@example.com", `p"w`)

		if got.method != http.MethodPost || got.path != path {
			t.Errorf("request = %s %s, want POST %s", got.method, got.path, path)
		}
		if !strings.HasPrefix(got.contentType, "application/json") {
			t.Errorf("Content-Type = %q", got.contentType)
		}
		// Even an empty Authorization is one a browser never sends before login.
		if auth {
			t.Error("Authorization sent before login")
		}
		var body struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal([]byte(got.body), &body); err != nil {
			t.Fatalf("body %q: %v", got.body, err)
		}
		// The quote must survive encoding, or a password with one would log in as another.
		if body.Login != "user@example.com" || body.Password != `p"w` {
			t.Errorf("body = %+v", body)
		}
	})

	for _, tt := range []struct {
		name, body string
		want       any
		wantErr    string
	}{
		{"token", `{"token":"t"}`, TokenMsg("t"), ""},
		{"token wins over an mfa flag", `{"token":"t","mfa":true}`, TokenMsg("t"), ""},
		{"mfa starts the code step", `{"mfa":true,"ticket":"x","login_instance_id":"i","totp":true,"backup":true}`,
			MFAMsg{ticket{id: "x", instance: "i", allowed: [methodCount]bool{methodTOTP: true, methodBackup: true}}}, ""},
		{"mfa with only a security key", `{"mfa":true,"ticket":"x","webauthn":"{}"}`, nil, "security key"},
		{"mfa without a ticket", `{"mfa":true,"totp":true}`, nil, "two-factor ticket"},
		{"no token and no mfa", `{"user_id":"1"}`, nil, "did not include a token"},
		{"empty object", `{}`, nil, "did not include a token"},
		{"required actions only", `{"required_actions":["update_password"]}`, nil, "did not include a token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			msg := requestLogin(t.Context(), discordtest.Client(t, reply(http.StatusOK, tt.body)), "a", "b")
			if tt.wantErr == "" {
				if msg != tt.want {
					t.Fatalf("got %#v, want %#v", msg, tt.want)
				}
				return
			}
			failed, ok := msg.(FailedMsg)
			if !ok || failed.Err == nil || !strings.Contains(failed.Err.Error(), tt.wantErr) {
				t.Fatalf("got %#v, want FailedMsg containing %q", msg, tt.wantErr)
			}
		})
	}

	for _, tt := range []struct {
		name   string
		status int
		body   string
	}{
		{"invalid credentials", http.StatusBadRequest, `{"code":50035,"message":"Invalid Form Body","errors":{"login":{"_errors":[{"code":"INVALID_LOGIN"}]}}}`},
		// An hCaptcha challenge is answered instead (TestRequestLoginCaptcha); another service is not.
		{"captcha of another service", http.StatusBadRequest, `{"captcha_key":["captcha-required"],"captcha_sitekey":"k","captcha_service":"recaptcha"}`},
		{"unauthorized", http.StatusUnauthorized, `{"message":"401: Unauthorized","code":0}`},
		{"forbidden", http.StatusForbidden, `{"message":"forbidden"}`},
		{"not JSON", http.StatusOK, `<html>`},
		{"empty body", http.StatusOK, ``},
	} {
		t.Run(tt.name+" fails", func(t *testing.T) {
			msg := requestLogin(t.Context(), discordtest.Client(t, reply(tt.status, tt.body)), "a", "b")
			if failed, ok := msg.(FailedMsg); !ok || failed.Err == nil {
				t.Fatalf("got %#v, want FailedMsg", msg)
			}
		})
	}

	t.Run("an HTTP error keeps its status", func(t *testing.T) {
		msg := requestLogin(t.Context(), discordtest.Client(t, reply(http.StatusBadRequest, `{"message":"bad"}`)), "a", "b")
		httpErr, ok := errors.AsType[*httputil.HTTPError](msg.(FailedMsg).Err)
		if !ok || httpErr.Status != http.StatusBadRequest {
			t.Fatalf("got %#v, want an HTTPError with status 400", msg)
		}
	})

	t.Run("a 4xx is not retried", func(t *testing.T) {
		var calls atomic.Int32
		client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			reply(http.StatusBadRequest, `{}`)(w, r)
		}))
		requestLogin(t.Context(), client, "a", "b")
		if n := calls.Load(); n != 1 {
			t.Fatalf("credentials posted %d times, want 1", n)
		}
	})

	t.Run("a canceled context fails without a request", func(t *testing.T) {
		var calls atomic.Int32
		client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			reply(http.StatusOK, `{"token":"t"}`)(w, r)
		}))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		msg := requestLogin(ctx, client, "a", "b")
		failed, ok := msg.(FailedMsg)
		if !ok || !errors.Is(failed.Err, context.Canceled) {
			t.Fatalf("got %#v, want FailedMsg wrapping context.Canceled", msg)
		}
		if n := calls.Load(); n != 0 {
			t.Fatalf("%d requests after cancel, want 0", n)
		}
	})
}

// evidence: witnesses
func TestAuthClient(t *testing.T) {
	for _, fingerprint := range []string{"", "1.x"} {
		var referer, platform, got string
		var sent bool
		client := discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			referer, platform = r.Header.Get("Referer"), r.Header.Get("Sec-Ch-Ua-Platform")
			_, sent = r.Header["X-Fingerprint"]
			got = r.Header.Get("X-Fingerprint")
			_, _ = w.Write([]byte(`{"token":"t"}`))
		}))
		requestLogin(t.Context(), authClient(client, dhttp.Build{}, fingerprint), "a", "b")
		if referer != "https://discord.com/login" {
			t.Errorf("Referer = %q, want the login page", referer)
		}
		if platform == "" {
			t.Error("browser headers were not sent")
		}
		// An empty header would still count as a fingerprint, so none is sent.
		if got != fingerprint || sent != (fingerprint != "") {
			t.Errorf("X-Fingerprint = %q (sent %v), want %q", got, sent, fingerprint)
		}
	}
}

func TestSubmitLogin(t *testing.T) {
	for _, final := range []bool{false, true} {
		if _, ok := submitLogin(dhttp.Build{}, dhttp.NewBrowser(dhttp.Network{}), "f", captcha.Solution{}, "a", "b", final).(cmd.Task); !ok {
			t.Fatal("submitLogin does not run as a task, so it would block Update")
		}
	}
}

// A refused connection is the failure the one retry is for.
func TestRefusedLoginRetryable(t *testing.T) {
	msg := requestLogin(t.Context(), discordtest.ClosedClient(t), "a", "b")
	if failed, ok := msg.(FailedMsg); !ok || !dhttp.Retryable(failed.Err) {
		t.Fatalf("got %#v, want a retryable failure", msg)
	}
}

// The bodies are those Discord answered in a raw-debug log and a NetLog.
// evidence: witnesses
func TestLoginRefusal(t *testing.T) {
	for _, tt := range []struct {
		name     string
		body     string
		location bool
	}{
		{"new location", `{"message":"Invalid Form Body","code":50035,"errors":{"login":{"_errors":[{"code":"ACCOUNT_LOGIN_VERIFICATION_EMAIL","message":"New login location detected, please check your e-mail."}]}}}`, true},
		{"wrong password", `{"message":"Invalid Form Body","code":50035,"errors":{"login":{"_errors":[{"code":"INVALID_LOGIN","message":"Login or password is invalid."}]},"password":{"_errors":[{"code":"INVALID_LOGIN","message":"Login or password is invalid."}]}}}`, false},
		{"no field errors", `{"message":"Invalid Form Body","code":50035}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := discordtest.Client(t, reply(http.StatusBadRequest, tt.body))
			failed, ok := requestLogin(t.Context(), client, "a", "b").(FailedMsg)
			if !ok || isVerifyLocation(failed.Err) != tt.location {
				t.Fatalf("got %#v, want the new-location error = %v", failed, tt.location)
			}
			// Discord's own answer stays reachable, and a refusal is never retried.
			if _, ok := errors.AsType[*httputil.HTTPError](failed.Err); !ok || dhttp.Retryable(failed.Err) {
				t.Errorf("%v: HTTP error kept = %v, retryable = %v", failed.Err, ok, dhttp.Retryable(failed.Err))
			}
			if tt.location && !strings.Contains(failed.Err.Error(), "emailed you a link") {
				t.Errorf("message %q does not say to click the emailed link", failed.Err)
			}
		})
	}
}

func TestAuthClientPostsOnce(t *testing.T) {
	for _, code := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusTooManyRequests} {
		var calls atomic.Int32
		client := authClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(code)
		})), dhttp.Build{}, "")
		requestLogin(t.Context(), client, "a", "b")
		if n := calls.Load(); n != 1 {
			t.Fatalf("status %d: credentials posted %d times, want 1", code, n)
		}
	}
}
