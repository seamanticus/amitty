package qr

import (
	"fmt"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/ui"
)

// captchaMsg says Discord asked for challenge before it redeems ticket.
type captchaMsg struct {
	ticket    string
	challenge captcha.Challenge
}

// The captcha steps of a ticket exchange, named by the ticket.
type (
	captchaStepMsg = ui.CaptchaMsg[string]
	captchaDoneMsg = ui.CaptchaDoneMsg[string]
)

// updateCaptcha steps the captcha of the ticket exchange in flight; messages of a login that ended are dropped.
func (m Model) updateCaptcha(msg tui.Msg) (Model, tui.Cmd) {
	var command tui.Cmd
	switch msg := msg.(type) {
	case captchaMsg:
		if m.ended {
			return m, nil
		}
		m.status = "Captcha required..."
		m.captcha, command = m.captcha.Update(ui.Challenged(msg.ticket, msg.challenge))
		return m, command
	case captchaStepMsg:
		m.captcha, command = m.captcha.Update(msg)
		return m, command
	case captchaDoneMsg:
		if m.ended {
			return m, nil
		}
		if err := msg.Err; err != nil {
			if ui.IsCaptchaRefused(err) {
				return m.end(fmt.Sprintf("%v; scan a new code later", err))
			}
			return m.end(err.Error())
		}
		m.status = "Authenticating..."
		return m, exchangeTicket(m.build, m.network, m.fingerprint, m.captcha.Solution(), m.privateKey, msg.Key, false)
	}
	return m, nil
}
