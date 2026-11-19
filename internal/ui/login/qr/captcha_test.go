package qr

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
	dhttp "github.com/seamanticus/amitty/internal/http"
)

// challengeBody is a captcha response as Discord sends it.
const challengeBody = `{"captcha_key":["captcha-required"],"captcha_service":"hcaptcha","captcha_sitekey":"sk","captcha_rqtoken":"rt"}`

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

// answer posts key to page as the browser does.
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

func TestRedeemTicketCaptcha(t *testing.T) {
	client := ticketClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(challengeBody))
	})), dhttp.Build{}, "fp")
	if msg := redeemTicket(t.Context(), client, testKey(t), "tk"); msg != (captchaMsg{ticket: "tk", challenge: testChallenge(t)}) {
		t.Fatalf("got %#v", msg)
	}
}

func TestModelCaptcha(t *testing.T) {
	exchanging := func() Model {
		m := NewModel(dhttp.Build{}, dhttp.Network{})
		m.status = "Authenticating..."
		return m
	}

	t.Run("a challenge serves a page, up to the last try", func(t *testing.T) {
		m := exchanging()
		for try := 1; try <= captcha.MaxTries; try++ {
			var command tview.Cmd
			m, command = m.Update(captchaMsg{ticket: "tk", challenge: testChallenge(t)})
			if m.captchaTries != try || command == nil || m.ended {
				t.Fatalf("try %d: tries = %d, ended = %v, command = %v", try, m.captchaTries, m.ended, command)
			}
		}
		if m, _ = m.Update(captchaMsg{ticket: "tk", challenge: testChallenge(t)}); !m.ended || !strings.Contains(m.status, "did not accept") {
			t.Fatalf("ended = %v, status = %q", m.ended, m.status)
		}
	})

	t.Run("the page is waited on and the answer redeems the ticket again", func(t *testing.T) {
		page := testPage(t)
		m, command := exchanging().Update(captchaPageMsg{ticket: "tk", page: page})
		batch, ok := command.(cmd.Batch)
		if !ok || len(batch) != 2 || m.captcha != page || !strings.Contains(m.status, "browser") {
			t.Fatalf("captcha = %v, status = %q, command = %#v", m.captcha, m.status, command)
		}
		answer(t, page, "key")
		solved, ok := batch[1].(cmd.Task)().(captchaSolvedMsg)
		if !ok || solved.ticket != "tk" {
			t.Fatalf("wait = %#v", solved)
		}
		m, command = m.Update(solved)
		if m.captcha != nil || m.solution != solved.solution || command == nil {
			t.Fatalf("captcha = %v, solution = %+v, command = %v", m.captcha, m.solution, command)
		}
	})

	t.Run("no browser shows the URL to open by hand", func(t *testing.T) {
		m := exchanging()
		m.captcha, m.captchaTries = testPage(t), 1
		m, _ = m.Update(captchaOpenFailedMsg{page: m.captcha, err: errors.New("no handler")})
		if !strings.Contains(m.status, "no handler") || !strings.Contains(m.status, m.captcha.URL()) {
			t.Fatalf("status = %q", m.status)
		}
	})

	t.Run("Esc ends the login and closes the page", func(t *testing.T) {
		m := exchanging()
		page := testPage(t)
		m.captcha = page
		m, command := m.Update(tview.KeyMsg(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone)))
		if !m.ended || m.captcha != nil {
			t.Fatalf("ended = %v, captcha = %v", m.ended, m.captcha)
		}
		for _, c := range command.(cmd.Batch) {
			if task, ok := c.(cmd.Task); ok {
				task()
			}
		}
		if _, err := page.Wait(t.Context()); err == nil {
			t.Fatal("page still open")
		}
	})

	t.Run("an ended login drops captcha messages and closes a late page", func(t *testing.T) {
		m, _ := exchanging().end("Canceled")
		for _, msg := range []tview.Msg{captchaMsg{ticket: "tk", challenge: testChallenge(t)}, captchaSolvedMsg{ticket: "tk"}} {
			if got, command := m.Update(msg); command != nil || got.status != "Canceled" {
				t.Fatalf("%T: status = %q, command = %v", msg, got.status, command)
			}
		}
		if got, command := m.Update(captchaPageMsg{ticket: "tk", page: testPage(t)}); got.captcha != nil || command == nil {
			t.Fatalf("captcha = %v, command = %v", got.captcha, command)
		}
	})
}
