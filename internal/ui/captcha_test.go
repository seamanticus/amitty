package ui

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// testChallenge is a captcha response as Discord sends it, as a challenge.
func testChallenge(t *testing.T) captcha.Challenge {
	t.Helper()
	body := `{"captcha_key":["captcha-required"],"captcha_service":"hcaptcha","captcha_sitekey":"sk","captcha_session_id":"sid","captcha_rqtoken":"rt"}`
	challenge, ok := captcha.FromError(&httputil.HTTPError{Status: http.StatusBadRequest, Body: []byte(body)})
	if !ok {
		t.Fatal("not a challenge")
	}
	return challenge
}

// run runs command, which must be a task, and returns its message.
func run(t *testing.T, command tui.Cmd) tui.Msg {
	t.Helper()
	task, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command = %#v, want a task", command)
	}
	return task()
}

// served steps c from a challenge of request key to a page waiting for an answer, closed when the test ends. It returns the wait for the answer.
func served(t *testing.T, c Captcha[int], key int) (Captcha[int], tui.Cmd) {
	t.Helper()
	c, command := c.Update(Challenged(key, testChallenge(t)))
	msg, ok := run(t, command).(CaptchaMsg[int])
	if !ok || msg.page == nil {
		t.Fatalf("serve = %#v", msg)
	}
	t.Cleanup(msg.page.Close)
	c, command = c.Update(msg)
	batch, ok := command.(cmd.Batch)
	if !ok || len(batch) != 2 || !c.Waiting() {
		t.Fatalf("waiting = %v, command = %#v", c.Waiting(), command)
	}
	return c, batch[1]
}

// answer posts key to the page as the browser does.
func answer(t *testing.T, page *captcha.Page, key string) {
	t.Helper()
	u, err := url.Parse(page.URL())
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, page.URL()+"/answer", strings.NewReader(`{"key":"`+key+`"}`))
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
}

func TestCaptcha(t *testing.T) {
	// evidence: witnesses
	t.Run("an answer ends the captcha with the solution", func(t *testing.T) {
		c, _ := Captcha[int]{}.Start(1)
		c, wait := served(t, c, 1)
		if prompt := c.Prompt(); !strings.Contains(prompt, "try 1 of") {
			t.Fatalf("prompt = %q", prompt)
		}
		answer(t, c.page, "key")
		c, command := c.Update(run(t, wait).(CaptchaMsg[int]))
		if done := run(t, command); done != (CaptchaDoneMsg[int]{Key: 1}) {
			t.Fatalf("done = %#v", done)
		}
		if c.Waiting() || c.Solution() == (captcha.Solution{}) {
			t.Fatalf("waiting = %v, solution = %+v", c.Waiting(), c.Solution())
		}
		if c, _ = c.Start(2); c.Solution() != (captcha.Solution{}) {
			t.Fatal("Start kept the answer of the earlier request")
		}
	})

	t.Run("challenges past the last try end the captcha refused and keep no page", func(t *testing.T) {
		c, _ := Captcha[int]{}.Start(1)
		for range captcha.MaxTries {
			c, _ = served(t, c, 1)
			c.page = nil
		}
		c, command := c.Update(Challenged(1, testChallenge(t)))
		var done tui.Msg
		for _, step := range command.(cmd.Batch) {
			if step != nil {
				done = run(t, step)
			}
		}
		if msg, ok := done.(CaptchaDoneMsg[int]); !ok || msg.Key != 1 || !IsCaptchaRefused(msg.Err) {
			t.Fatalf("done = %#v", done)
		}
		if c.active || c.tries != 0 {
			t.Fatalf("captcha = %+v", c)
		}
	})

	t.Run("a failed browser shows the URL to open by hand", func(t *testing.T) {
		c, _ := Captcha[int]{}.Start(1)
		c, _ = served(t, c, 1)
		c, _ = c.Update(CaptchaMsg[int]{key: 1, step: captchaOpenFailed, page: c.page, err: errors.New("no handler")})
		if prompt := c.Prompt(); !strings.Contains(prompt, "no handler") || !strings.Contains(prompt, c.page.URL()) {
			t.Fatalf("prompt = %q", prompt)
		}
	})

	t.Run("cancel ends the captcha with an error", func(t *testing.T) {
		c, _ := Captcha[int]{}.Start(1)
		c, wait := served(t, c, 1)
		run(t, c.Cancel())
		c, command := c.Update(run(t, wait).(CaptchaMsg[int]))
		if done, ok := run(t, command).(CaptchaDoneMsg[int]); !ok || done.Key != 1 || done.Err == nil || IsCaptchaRefused(done.Err) {
			t.Fatalf("done = %#v", done)
		}
		if c.Waiting() || c.active {
			t.Fatalf("captcha = %+v", c)
		}
	})

	t.Run("steps of another or no request are dropped and their page closed", func(t *testing.T) {
		other, _ := Captcha[int]{}.Start(2)
		other, _ = served(t, other, 2)
		page := other.page
		for _, c := range []Captcha[int]{{}, other} {
			c, _ = c.Start(1)
			if c.Waiting() {
				t.Fatal("Start kept the earlier page")
			}
			got, command := c.Update(CaptchaMsg[int]{key: 2, step: captchaServed, page: page})
			if got.Waiting() || command == nil {
				t.Fatalf("waiting = %v, command = %v", got.Waiting(), command)
			}
		}
		if got, command := (Captcha[int]{}).Update(Challenged(1, testChallenge(t))); got.tries != 0 || command != nil {
			t.Fatalf("captcha = %+v, command = %v", got, command)
		}
	})

	t.Run("start forgets the tries of the earlier request", func(t *testing.T) {
		c := Captcha[int]{active: true, key: 1, tries: 2}
		if c, _ = c.Start(2); c.tries != 0 || c.key != 2 || !c.active {
			t.Fatalf("captcha = %+v", c)
		}
	})
}
