package password

import (
	"fmt"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
)

// attempt numbers the logins of a form, so the captcha steps of an earlier one are dropped.
type attempt uint64

// ChallengedMsg says Discord asked for challenge before it takes the login.
type ChallengedMsg struct{ challenge captcha.Challenge }

// CaptchaMsg and CaptchaDoneMsg are the captcha steps of a login.
type (
	CaptchaMsg     = ui.CaptchaMsg[attempt]
	CaptchaDoneMsg = ui.CaptchaDoneMsg[attempt]
)

// captchaDone sends the login again with the answer, or fails it.
func (m Model) captchaDone(msg CaptchaDoneMsg) (Model, tui.Cmd) {
	if !m.submitting || msg.Key != m.attempt {
		return m, nil
	}
	if err := msg.Err; err != nil {
		if ui.IsCaptchaRefused(err) {
			err = fmt.Errorf("%w; try again later or use QR login", err)
		}
		return m, cmd.Task(func() tui.Msg { return FailedMsg{err} })
	}
	return m, m.submit(false)
}
