package password

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
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
		if msg := requestLogin(t.Context(), client, "a", "b"); msg != (CaptchaMsg{testChallenge(t)}) {
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

func TestModelCaptcha(t *testing.T) {
	submitting := func() Model {
		return Model{login: "u", password: "p", submitting: true, fingerprint: "f"}
	}

	t.Run("a challenge serves a page, up to the last try", func(t *testing.T) {
		m := submitting()
		for try := 1; try <= captcha.MaxTries; try++ {
			var command tview.Cmd
			m, command = m.Update(CaptchaMsg{testChallenge(t)})
			if m.captchaTries != try || command == nil {
				t.Fatalf("try %d: tries = %d, command = %v", try, m.captchaTries, command)
			}
		}
		_, command := m.Update(CaptchaMsg{testChallenge(t)})
		if failed, ok := command.(cmd.Task)().(FailedMsg); !ok || !strings.Contains(failed.Err.Error(), "did not accept") {
			t.Fatalf("after %d tries: %#v", captcha.MaxTries, command)
		}
	})

	t.Run("a challenge while idle is dropped", func(t *testing.T) {
		if m, command := (Model{}).Update(CaptchaMsg{testChallenge(t)}); command != nil || m.captchaTries != 0 {
			t.Fatalf("tries = %d, command = %v", m.captchaTries, command)
		}
	})

	t.Run("a page opens in the browser and is waited on", func(t *testing.T) {
		page := testPage(t)
		m, command := submitting().Update(CaptchaPageMsg{page})
		if batch, ok := command.(cmd.Batch); !ok || len(batch) != 2 || m.captcha != page {
			t.Fatalf("captcha = %v, command = %#v", m.captcha, command)
		}
		if help := m.ShortHelp(); len(help) != 1 || help[0].Help().Desc != "cancel captcha" {
			t.Fatalf("help = %v", help)
		}
	})

	t.Run("a page for an ended login is closed", func(t *testing.T) {
		page := testPage(t)
		m, command := (Model{}).Update(CaptchaPageMsg{page})
		if m.captcha != nil || command == nil {
			t.Fatalf("captcha = %v, command = %v", m.captcha, command)
		}
		command.(cmd.Task)()
		if _, err := page.Wait(t.Context()); err == nil {
			t.Fatal("page still open")
		}
	})

	t.Run("Esc closes the page, failing the login", func(t *testing.T) {
		m := submitting()
		m.captcha = testPage(t)
		_, command := m.Update(key(tcell.KeyEsc, ""))
		if command == nil {
			t.Fatal("Esc did nothing")
		}
		command.(cmd.Task)()
		if msg, ok := waitCaptcha(m.captcha).(cmd.Task)().(FailedMsg); !ok || !strings.Contains(msg.Err.Error(), "canceled") {
			t.Fatalf("wait = %#v", msg)
		}
	})

	t.Run("no browser shows the URL to open by hand", func(t *testing.T) {
		m := submitting()
		m.captcha, m.captchaTries = testPage(t), 1
		m, _ = m.Update(CaptchaOpenFailedMsg{errors.New("no handler")})
		drawn := discordtest.Draw(t, m.View(), 200, 12)
		if !strings.Contains(drawn, "no handler") || !strings.Contains(drawn, m.captcha.URL()) || !strings.Contains(drawn, "try 1 of") {
			t.Fatalf("drawn:\n%s", drawn)
		}
	})

	t.Run("an answer logs in again with it", func(t *testing.T) {
		m := submitting()
		m.captcha = testPage(t)
		solution := solve(t, m.captcha, "key")
		m, command := m.Update(CaptchaSolvedMsg{solution})
		if m.captcha != nil || m.solution != solution || command == nil {
			t.Fatalf("captcha = %v, solution = %+v, command = %v", m.captcha, m.solution, command)
		}
	})

	t.Run("a failure forgets the captcha", func(t *testing.T) {
		m := submitting()
		m.captcha, m.captchaTries, m.captchaOpenErr = testPage(t), 2, "x"
		m, _ = m.Update(FailedMsg{errors.New("x")})
		if m.captcha != nil || m.captchaTries != 0 || m.captchaOpenErr != "" || m.solution != (captcha.Solution{}) {
			t.Fatalf("model = %+v", m)
		}
	})

	t.Run("a new submit starts with no answer", func(t *testing.T) {
		m := Model{login: "u", password: "p", captchaTries: 3}
		if m, _ = m.Update(submitMsg{}); m.captchaTries != 0 {
			t.Fatalf("tries = %d", m.captchaTries)
		}
	})
}
