package password

import (
	"context"
	"fmt"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/ui"
)

// CaptchaMsg says Discord asked for challenge before it takes the login.
type CaptchaMsg struct{ challenge captcha.Challenge }

// CaptchaPageMsg is the page that shows the challenge, ready for the browser.
type CaptchaPageMsg struct{ page *captcha.Page }

// CaptchaOpenFailedMsg says no browser opened the page, which the user can still open by hand.
type CaptchaOpenFailedMsg struct{ err error }

// CaptchaSolvedMsg is the answer to send the login again with.
type CaptchaSolvedMsg struct{ solution captcha.Solution }

// serveCaptcha puts challenge on a page.
func serveCaptcha(challenge captcha.Challenge) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		page, err := captcha.Serve(context.Background(), challenge)
		if err != nil {
			return FailedMsg{err}
		}
		return CaptchaPageMsg{page}
	})
}

// openCaptcha opens url in the browser. It can block until a browser it started exits, so it runs apart from waitCaptcha.
func openCaptcha(url string) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		if err := ui.OpenExternal(context.Background(), url); err != nil {
			return CaptchaOpenFailedMsg{err}
		}
		return nil
	})
}

// waitCaptcha waits for the answer to page, which closes itself after captcha.AnswerTimeout.
func waitCaptcha(page *captcha.Page) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		solution, err := page.Wait(context.Background())
		if err != nil {
			return FailedMsg{err}
		}
		return CaptchaSolvedMsg{solution}
	})
}

// closeCaptcha closes page, which ends its waitCaptcha with a failure.
func closeCaptcha(page *captcha.Page) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		page.Close()
		return nil
	})
}

// updateCaptcha steps the captcha of the login in flight; messages of a login that ended are dropped.
func (m Model) updateCaptcha(msg tview.Msg) (Model, tview.Cmd) {
	switch msg := msg.(type) {
	case CaptchaMsg:
		if !m.submitting {
			return m, nil
		}
		if m.captchaTries >= captcha.MaxTries {
			err := fmt.Errorf("Discord did not accept the captcha after %d tries; try again later or use QR login", captcha.MaxTries)
			return m, cmd.Task(func() tview.Msg { return FailedMsg{err} })
		}
		m.captchaTries++
		return m, serveCaptcha(msg.challenge)
	case CaptchaPageMsg:
		if !m.submitting {
			return m, closeCaptcha(msg.page)
		}
		m.captcha, m.captchaOpenErr = msg.page, ""
		return m, cmd.Batch{openCaptcha(msg.page.URL()), waitCaptcha(msg.page)}
	case CaptchaOpenFailedMsg:
		if m.captcha != nil {
			m.captchaOpenErr = msg.err.Error()
		}
		return m, nil
	case CaptchaSolvedMsg:
		if !m.submitting {
			return m, nil
		}
		m.captcha, m.captchaOpenErr = nil, ""
		m.solution = msg.solution
		return m, m.submit(false)
	}
	return m, nil
}
