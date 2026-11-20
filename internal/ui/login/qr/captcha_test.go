package qr

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
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

// exchanging is a login redeeming ticket tk.
func exchanging() Model {
	m := NewModel(dhttp.Build{}, dhttp.Network{})
	m, _ = m.Update(pendingLoginMsg{ticket: "tk"})
	return m
}

// challenged is exchanging with the page of a challenge waiting, closed when the test ends.
func challenged(t *testing.T) Model {
	t.Helper()
	m, command := exchanging().Update(captchaMsg{ticket: "tk", challenge: testChallenge(t)})
	served, ok := command.(cmd.Task)().(captchaStepMsg)
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
	t.Run("a challenge shows the page in the status", func(t *testing.T) {
		m := challenged(t)
		if m.ended {
			t.Fatal("ended")
		}
		if drawn := discordtest.Draw(t, m.View(), 200, 12); !strings.Contains(drawn, "try 1 of") {
			t.Fatalf("drawn:\n%s", drawn)
		}
	})

	t.Run("an answer redeems the ticket again", func(t *testing.T) {
		m, command := exchanging().Update(captchaDoneMsg{Key: "tk"})
		if m.ended || m.status != "Authenticating..." || command == nil {
			t.Fatalf("ended = %v, status = %q, command = %v", m.ended, m.status, command)
		}
	})

	t.Run("a refused captcha ends the login", func(t *testing.T) {
		m := exchanging()
		// Each challenge is a try; the pages are never served.
		for range captcha.MaxTries {
			m, _ = m.Update(captchaMsg{ticket: "tk", challenge: testChallenge(t)})
		}
		m, command := m.Update(captchaMsg{ticket: "tk", challenge: testChallenge(t)})
		for _, step := range command.(cmd.Batch) {
			if task, ok := step.(cmd.Task); ok {
				if done, ok := task().(captchaDoneMsg); ok {
					m, _ = m.Update(done)
				}
			}
		}
		if !m.ended || !strings.Contains(m.status, "did not accept") || !strings.Contains(m.status, "scan a new code") {
			t.Fatalf("ended = %v, status = %q", m.ended, m.status)
		}
	})

	t.Run("Esc ends the login and closes the page", func(t *testing.T) {
		m := challenged(t)
		m, command := m.Update(tui.KeyMsg(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone)))
		if !m.ended || m.captcha.Waiting() || command == nil {
			t.Fatalf("ended = %v, waiting = %v, command = %v", m.ended, m.captcha.Waiting(), command)
		}
	})

	t.Run("an ended login drops captcha messages", func(t *testing.T) {
		m, _ := exchanging().end("Canceled")
		for _, msg := range []tui.Msg{captchaMsg{ticket: "tk", challenge: testChallenge(t)}, captchaDoneMsg{Key: "tk"}} {
			if got, command := m.Update(msg); command != nil || got.status != "Canceled" {
				t.Fatalf("%T: status = %q, command = %v", msg, got.status, command)
			}
		}
	})
}
