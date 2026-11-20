package password

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// challengeBody is a captcha response as Discord sends it.
const challengeBody = `{"captcha_key":["captcha-required"],"captcha_service":"hcaptcha","captcha_sitekey":"sk","captcha_session_id":"sid","captcha_rqtoken":"rt"}`

// testChallenge is challengeBody as a challenge.
func testChallenge(t *testing.T) captcha.Challenge {
	t.Helper()
	challenge, ok := captcha.FromError(&httputil.HTTPError{Status: http.StatusBadRequest, Body: []byte(challengeBody)})
	if !ok {
		t.Fatal("challengeBody is not a challenge")
	}
	return challenge
}

// testPage serves testChallenge, closed when the test ends.
func testPage(t *testing.T) *captcha.Page {
	t.Helper()
	page, err := captcha.Serve(t.Context(), testChallenge(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(page.Close)
	return page
}

// solve answers page with key as the browser does and returns the solution.
func solve(t *testing.T, page *captcha.Page, key string) captcha.Solution {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, page.URL()+"/answer", strings.NewReader(`{"key":"`+key+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(page.URL())
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", u.Scheme+"://"+u.Host)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("answer status %d", resp.StatusCode)
	}
	solution, err := page.Wait(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return solution
}

func TestRequestLoginCaptcha(t *testing.T) {
	t.Run("a challenge asks for a captcha", func(t *testing.T) {
		client := discordtest.Client(t, reply(http.StatusBadRequest, challengeBody))
		if msg := requestLogin(t.Context(), client, "a", "b"); msg != (ChallengedMsg{testChallenge(t)}) {
			t.Fatalf("got %#v", msg)
		}
	})

	t.Run("the answer goes with the login", func(t *testing.T) {
		var got http.Header
		client := withSolution(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Clone()
			reply(http.StatusOK, `{"token":"t"}`)(w, r)
		})), solve(t, testPage(t), "key"))
		if msg := requestLogin(t.Context(), client, "a", "b"); msg != TokenMsg("t") {
			t.Fatalf("got %#v", msg)
		}
		if got.Get("X-Captcha-Key") != "key" || got.Get("X-Captcha-Session-Id") != "sid" || got.Get("X-Captcha-Rqtoken") != "rt" {
			t.Fatalf("headers = %v", got)
		}
	})

	t.Run("no answer sends no captcha headers", func(t *testing.T) {
		present := true
		client := withSolution(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, present = r.Header["X-Captcha-Key"]
			reply(http.StatusOK, `{"token":"t"}`)(w, r)
		})), captcha.Solution{})
		requestLogin(t.Context(), client, "a", "b")
		if present {
			t.Fatal("X-Captcha-Key sent without an answer")
		}
	})
}

// challenged submits m and serves the challenge its login got, closed when the test ends.
func challenged(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = m.Update(submitMsg{})
	m, command := m.Update(ChallengedMsg{testChallenge(t)})
	served, ok := command.(cmd.Task)().(CaptchaMsg)
	if !ok {
		t.Fatalf("serve = %#v", served)
	}
	m, _ = m.Update(served)
	t.Cleanup(func() {
		if closing, ok := m.captcha.Cancel().(cmd.Task); ok {
			closing()
		}
	})
	return m
}

func TestModelCaptcha(t *testing.T) {
	// evidence: witnesses
	form := Model{login: "u", password: "p", fingerprint: "f"}

	t.Run("a challenge shows the page and Esc cancels it", func(t *testing.T) {
		m := challenged(t, form)
		if help := m.ShortHelp(); len(help) != 1 || help[0].Help().Desc != "cancel captcha" {
			t.Fatalf("help = %v", help)
		}
		if drawn := discordtest.Draw(t, m.View(), 200, 12); !strings.Contains(drawn, "try 1 of") {
			t.Fatalf("drawn:\n%s", drawn)
		}
		if _, command := m.Update(key(tcell.KeyEsc, "")); command == nil {
			t.Fatal("Esc did nothing")
		}
	})

	t.Run("a challenge while idle is dropped", func(t *testing.T) {
		if m, command := form.Update(ChallengedMsg{testChallenge(t)}); command != nil || m.captcha.Waiting() {
			t.Fatalf("waiting = %v, command = %v", m.captcha.Waiting(), command)
		}
	})

	t.Run("an answer logs in again", func(t *testing.T) {
		m, _ := form.Update(submitMsg{})
		if _, command := m.Update(CaptchaDoneMsg{Key: m.attempt}); command == nil {
			t.Fatal("no login")
		}
	})

	t.Run("the end of an earlier login's captcha is dropped", func(t *testing.T) {
		m, _ := form.Update(submitMsg{})
		if _, command := m.Update(CaptchaDoneMsg{Key: m.attempt - 1}); command != nil {
			t.Fatalf("command = %v", command)
		}
	})

	t.Run("a refused captcha fails the login and points to QR login", func(t *testing.T) {
		m := challenged(t, form)
		for range captcha.MaxTries - 1 {
			m, _ = m.Update(ChallengedMsg{testChallenge(t)})
		}
		_, command := m.Update(ChallengedMsg{testChallenge(t)})
		var done CaptchaDoneMsg
		for _, step := range command.(cmd.Batch) {
			if task, ok := step.(cmd.Task); ok {
				if msg, ok := task().(CaptchaDoneMsg); ok {
					done = msg
				}
			}
		}
		_, command = m.Update(done)
		if failed, ok := command.(cmd.Task)().(FailedMsg); !ok || !strings.Contains(failed.Err.Error(), "did not accept") || !strings.Contains(failed.Err.Error(), "QR login") {
			t.Fatalf("failed = %#v", failed)
		}
	})

	t.Run("a failure closes the page", func(t *testing.T) {
		m := challenged(t, form)
		m, command := m.Update(FailedMsg{errors.New("x")})
		if m.captcha.Waiting() || command == nil {
			t.Fatalf("waiting = %v, command = %v", m.captcha.Waiting(), command)
		}
	})

	t.Run("a new submit names a new login", func(t *testing.T) {
		m, _ := form.Update(submitMsg{})
		m.submitting = false
		if next, _ := m.Update(submitMsg{}); next.attempt != m.attempt+1 {
			t.Fatalf("attempt = %d, want %d", next.attempt, m.attempt+1)
		}
	})
}
