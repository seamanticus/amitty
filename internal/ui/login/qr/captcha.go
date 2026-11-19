package qr

import (
	"context"
	"fmt"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
)

// captchaMsg says Discord asked for challenge before it redeems ticket.
type captchaMsg struct {
	ticket    string
	challenge captcha.Challenge
}

// captchaPageMsg is the page that shows the challenge of ticket, ready for the browser.
type captchaPageMsg struct {
	ticket string
	page   *captcha.Page
}

// captchaOpenFailedMsg says no browser opened page, which the user can still open by hand.
type captchaOpenFailedMsg struct {
	page *captcha.Page
	err  error
}

// captchaSolvedMsg is the answer to redeem ticket again with.
type captchaSolvedMsg struct {
	ticket   string
	solution captcha.Solution
}

func serveCaptcha(ticket string, challenge captcha.Challenge) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		page, err := captcha.Serve(context.Background(), challenge)
		if err != nil {
			return err
		}
		return captchaPageMsg{ticket: ticket, page: page}
	})
}

// openCaptcha opens page in the browser. It can block until a browser it started exits, so it runs apart from waitCaptcha.
func openCaptcha(page *captcha.Page) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if err := ui.OpenExternal(context.Background(), page.URL()); err != nil {
			return captchaOpenFailedMsg{page: page, err: err}
		}
		return nil
	})
}

// waitCaptcha waits for the answer to page, which closes itself after captcha.AnswerTimeout.
func waitCaptcha(ticket string, page *captcha.Page) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		solution, err := page.Wait(context.Background())
		if err != nil {
			return err
		}
		return captchaSolvedMsg{ticket: ticket, solution: solution}
	})
}

func closeCaptcha(page *captcha.Page) tui.Cmd {
	if page == nil {
		return nil
	}
	return cmd.Task(func() tui.Msg {
		page.Close()
		return nil
	})
}

// updateCaptcha steps the captcha of the ticket exchange in flight; messages of a login that ended are dropped.
func (m Model) updateCaptcha(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case captchaMsg:
		if m.ended {
			return m, nil
		}
		if m.captchaTries >= captcha.MaxTries {
			return m.end(fmt.Sprintf("Discord did not accept the captcha after %d tries; scan a new code later", captcha.MaxTries))
		}
		m.captchaTries++
		m.status = "Captcha required..."
		return m, serveCaptcha(msg.ticket, msg.challenge)
	case captchaPageMsg:
		if m.ended {
			return m, closeCaptcha(msg.page)
		}
		m.captcha, m.captchaOpenErr = msg.page, ""
		m.status = msg.page.Prompt(m.captchaTries, "")
		return m, cmd.Batch{openCaptcha(msg.page), waitCaptcha(msg.ticket, msg.page)}
	case captchaOpenFailedMsg:
		if msg.page == m.captcha && m.captcha != nil {
			m.captchaOpenErr = msg.err.Error()
			m.status = m.captcha.Prompt(m.captchaTries, m.captchaOpenErr)
		}
		return m, nil
	case captchaSolvedMsg:
		if m.ended {
			return m, nil
		}
		m.captcha, m.captchaOpenErr = nil, ""
		m.solution = msg.solution
		m.status = "Authenticating..."
		return m, exchangeTicket(m.build, m.network, m.fingerprint, m.solution, m.privateKey, msg.ticket, false)
	}
	return m, nil
}
