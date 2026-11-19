package captcha

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// evidence: witnesses
func TestFromError(t *testing.T) {
	challenged := func(status int, body string) error {
		return &httputil.HTTPError{Status: status, Body: []byte(body)}
	}
	for _, tt := range []struct {
		name string
		err  error
		want Challenge
		ok   bool
	}{
		{"full", challenged(400, `{"captcha_key":["captcha-required"],"captcha_service":"hcaptcha","captcha_sitekey":"sk","captcha_session_id":"sid","captcha_rqdata":"rd","captcha_rqtoken":"rt","should_serve_invisible":true}`),
			Challenge{sitekey: "sk", sessionID: "sid", rqdata: "rd", rqtoken: "rt", invisible: true}, true},
		{"sitekey only", challenged(400, `{"captcha_service":"hcaptcha","captcha_sitekey":"sk"}`), Challenge{sitekey: "sk"}, true},
		{"another service", challenged(400, `{"captcha_service":"recaptcha","captcha_sitekey":"sk"}`), Challenge{}, false},
		{"no sitekey", challenged(400, `{"captcha_service":"hcaptcha"}`), Challenge{}, false},
		{"another status", challenged(401, `{"captcha_service":"hcaptcha","captcha_sitekey":"sk"}`), Challenge{}, false},
		{"a plain 400", challenged(400, `{"code":50035,"message":"Invalid Form Body"}`), Challenge{}, false},
		{"not JSON", challenged(400, `<html>`), Challenge{}, false},
		{"empty body", challenged(400, ``), Challenge{}, false},
		{"not an HTTP error", errors.New("x"), Challenge{}, false},
		{"nil", nil, Challenge{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FromError(tt.err)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got %+v, %v; want %+v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// evidence: witnesses
func TestSolutionHeaders(t *testing.T) {
	for _, tt := range []struct {
		name     string
		solution Solution
		want     http.Header
	}{
		{"zero", Solution{}, http.Header{}},
		{"no key sends nothing", Solution{sessionID: "sid", rqtoken: "rt"}, http.Header{}},
		{"key only", Solution{key: "k"}, http.Header{"X-Captcha-Key": {"k"}}},
		{"all", Solution{key: "k", sessionID: "sid", rqtoken: "rt"}, http.Header{
			"X-Captcha-Key": {"k"}, "X-Captcha-Session-Id": {"sid"}, "X-Captcha-Rqtoken": {"rt"},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.solution.Headers()
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for name := range tt.want {
				if got.Get(name) != tt.want.Get(name) {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestServeRefusesNoSitekey(t *testing.T) {
	if page, err := Serve(t.Context(), Challenge{}); err == nil {
		page.Close()
		t.Fatal("served a challenge without a sitekey")
	}
}

// serve starts a page for challenge, closed when the test ends.
func serve(t *testing.T, challenge Challenge) *Page {
	t.Helper()
	page, err := Serve(t.Context(), challenge)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(page.Close)
	if !strings.HasPrefix(page.URL(), "http://127.0.0.1:") {
		t.Fatalf("URL = %q, want the literal loopback address", page.URL())
	}
	return page
}

// post sends body to the page's answer path with origin and, if set, host.
func post(t *testing.T, page *Page, origin, host, body string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, page.URL()+"/answer", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestPage(t *testing.T) {
	challenge := Challenge{sitekey: "sk</script><b>", sessionID: "sid", rqdata: "rd", rqtoken: "rt"}

	t.Run("shows the challenge, unable to end its script", func(t *testing.T) {
		page := serve(t, challenge)
		resp, err := http.Get(page.URL())
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		html := string(body)
		if resp.StatusCode != http.StatusOK || !strings.Contains(html, `"rqdata":"rd"`) || !strings.Contains(html, "sk") {
			t.Fatalf("status %d, page:\n%s", resp.StatusCode, html)
		}
		if strings.Count(html, "</script>") != strings.Count(pageHTML, "</script>") {
			t.Fatalf("the sitekey ended a script:\n%s", html)
		}
	})

	t.Run("only the page path is served", func(t *testing.T) {
		page := serve(t, challenge)
		resp, err := http.Get(page.origin + "/")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d at /", resp.StatusCode)
		}
	})

	t.Run("refuses answers from elsewhere, then takes one", func(t *testing.T) {
		page := serve(t, challenge)
		for _, tt := range []struct {
			name, origin, host, body string
			want                     int
		}{
			{"another origin", "https://evil.example", "", `{"key":"k"}`, http.StatusForbidden},
			{"no origin", "", "", `{"key":"k"}`, http.StatusForbidden},
			{"localhost", "http://localhost", "localhost", `{"key":"k"}`, http.StatusMisdirectedRequest},
			{"another host", page.origin, "rebound.example", `{"key":"k"}`, http.StatusMisdirectedRequest},
			{"no key", page.origin, "", `{}`, http.StatusBadRequest},
			{"not JSON", page.origin, "", `key`, http.StatusBadRequest},
			{"too large", page.origin, "", `{"key":"` + strings.Repeat("k", maxAnswerBytes) + `"}`, http.StatusBadRequest},
			{"the answer", page.origin, "", `{"key":"k"}`, http.StatusNoContent},
			{"a second answer", page.origin, "", `{"key":"k2"}`, http.StatusConflict},
		} {
			if got := post(t, page, tt.origin, tt.host, tt.body); got != tt.want {
				t.Fatalf("%s: status %d, want %d", tt.name, got, tt.want)
			}
		}
		solution, err := page.Wait(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if solution != (Solution{key: "k", sessionID: "sid", rqtoken: "rt"}) {
			t.Fatalf("solution = %+v", solution)
		}
		if _, err := http.Get(page.URL()); err == nil {
			t.Fatal("the page is still served after its answer")
		}
	})

	t.Run("an ended context stops the wait", func(t *testing.T) {
		page := serve(t, challenge)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := page.Wait(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("the page closes when the context it was served with ends", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		page, err := Serve(ctx, challenge)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(page.Close)
		cancel()
		if _, err := page.Wait(t.Context()); err == nil || !strings.Contains(err.Error(), "canceled") {
			t.Fatalf("err = %v, want canceled", err)
		}
	})

	t.Run("Close stops the wait and is safe twice", func(t *testing.T) {
		page := serve(t, challenge)
		page.Close()
		page.Close()
		if _, err := page.Wait(t.Context()); err == nil {
			t.Fatal("a closed page gave an answer")
		}
	})
}
