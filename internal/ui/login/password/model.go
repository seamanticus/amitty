package password

import (
	"errors"
	"fmt"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/button"
	"github.com/ayn2op/tview/column"
	"github.com/ayn2op/tview/help"
	"github.com/ayn2op/tview/keybind"
	"github.com/ayn2op/tview/layout"
	"github.com/ayn2op/tview/row"
	"github.com/ayn2op/tview/text"
	"github.com/ayn2op/tview/textinput"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/seamanticus/amitty/internal/captcha"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/ui/login/keys"
)

// focus is the input or button that receives keys.
type focus int

const (
	focusLogin focus = iota
	focusPassword
	focusButton
	focusCount
)

// Model is a login and a password input above a button. Tab and the arrow keys move the focus, Enter in an input moves to the next one, and Enter on the button or clicking it logs in. A login that needs a second factor replaces the form with a code form until it ends or Esc goes back.
type Model struct {
	// build is what the login requests identify as.
	build dhttp.Build
	// network is how they connect.
	network dhttp.Network

	login, password string
	// loginCursor and passwordCursor are the byte offsets of the cursor in login and password.
	loginCursor, passwordCursor int

	focus focus
	// buttonHovered reports whether the pointer is over the button.
	buttonHovered bool
	// submitting reports whether a login request is in flight, during which submits are ignored.
	submitting bool
	// retryIn is the seconds left before the one retry of a login whose connection failed, and retryReason why; 0 when no retry is pending.
	retryIn     int
	retryReason string

	// code is the second step; its zero value means the form is shown.
	code codeForm
	// fingerprint ties the logins of this form to their second step; fetched before the first login and kept, as Discord grants only 3 per IP per 2 minutes.
	fingerprint string

	// captchaTries counts the challenges answered for the login in flight.
	captchaTries int
	// captcha is the page waiting for an answer, or nil; captchaOpenErr is why no browser opened it.
	captcha        *captcha.Page
	captchaOpenErr string
	// solution is the last answer, sent with the login until Discord takes or challenges it again.
	solution captcha.Solution
}

// NewModel returns an empty form whose login requests identify as build and connect through network.
func NewModel(build dhttp.Build, network dhttp.Network) Model {
	return Model{build: build, network: network}
}

func (Model) Label() string {
	return "Password"
}

var _ tview.Model[Model] = Model{}

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	if m.coding() {
		return m.code.keybinds()
	}
	if m.captcha != nil {
		return []keybind.Keybind{keys.Back("cancel captcha")}
	}
	return []keybind.Keybind{keys.FocusNext(), keys.FocusPrevious()}
}

func (m Model) FullHelp() [][]keybind.Keybind { return [][]keybind.Keybind{m.ShortHelp()} }

func (Model) Init() tview.Cmd { return nil }

func (m Model) Update(msg tview.Msg) (Model, tview.Cmd) {
	switch msg := msg.(type) {
	case MFAMsg:
		// The password is not needed for the second step, so it is not kept.
		m.password, m.passwordCursor = "", 0
		m.submitting = false
		msg.ticket.fingerprint = m.fingerprint
		m.code = codeForm{ticket: msg.ticket, method: msg.ticket.first(), focus: codeFocusInput}
		return m, nil
	case FingerprintMsg:
		if !m.submitting {
			return m, nil
		}
		m.fingerprint = msg.fingerprint
		return m, submitLogin(m.build, m.network, m.fingerprint, m.solution, m.login, m.password, msg.final)
	case SMSSentMsg:
		m.submitting = false
		m.code.phone = msg.phone
		return m, nil
	case CountdownMsg:
		if !m.submitting {
			return m, nil
		}
		if msg.left > 0 {
			m.retryIn, m.retryReason = msg.left, msg.err.Error()
			return m, tick(msg)
		}
		m.retryIn, m.retryReason = 0, ""
		if m.coding() {
			return m, submitCode(m.build, m.network, m.code.ticket, m.code.method, m.code.text, true)
		}
		return m, m.submit(true)
	case CaptchaMsg, CaptchaPageMsg, CaptchaOpenFailedMsg, CaptchaSolvedMsg:
		return m.updateCaptcha(msg)
	case FailedMsg:
		m.submitting = false
		m.retryIn, m.retryReason = 0, ""
		m.captchaTries, m.captcha, m.captchaOpenErr, m.solution = 0, nil, "", captcha.Solution{}
		if _, expired := errors.AsType[expiredError](msg.Err); expired {
			m.code = codeForm{}
			m.focus = focusPassword
		} else if m.coding() {
			m.code.text, m.code.cursor = "", 0
		}
		return m, nil
	}
	if m.coding() {
		return m.updateCode(msg)
	}

	switch msg := msg.(type) {
	case loginMsg:
		m.login, m.loginCursor = msg.text, msg.cursor
	case passwordMsg:
		m.password, m.passwordCursor = msg.text, msg.cursor
	case focusMsg:
		m.focus = focus(msg)
	case hoverMsg:
		m.buttonHovered = bool(msg)
	case tview.KeyMsg:
		switch {
		case keybind.Matches(msg, keys.FocusNext()):
			m.focus = (m.focus + 1) % focusCount
		case keybind.Matches(msg, keys.FocusPrevious()):
			m.focus = (m.focus + focusCount - 1) % focusCount
		case m.captcha != nil && keybind.Matches(msg, keys.Back("")):
			// The wait then fails, which ends the login.
			return m, closeCaptcha(m.captcha)
		}
	case submitMsg:
		if m.submitting || m.login == "" || m.password == "" {
			return m, nil
		}
		m.submitting = true
		m.captchaTries, m.solution = 0, captcha.Solution{}
		return m, m.submit(false)
	}
	return m, nil
}

// submit logs in with the form, first fetching the fingerprint if there is none yet.
func (m Model) submit(final bool) tview.Cmd {
	if m.fingerprint == "" {
		return fetchFingerprint(m.build, m.network, final)
	}
	return submitLogin(m.build, m.network, m.fingerprint, m.solution, m.login, m.password, final)
}

// coding reports whether the code form is shown.
func (m Model) coding() bool { return m.code.ticket.id != "" }

func (m Model) View() tview.Widget {
	if m.coding() {
		return m.viewCode()
	}
	labelWidth := layout.Fixed(len("Password") + 1)
	login := textinput.New(m.login, m.loginCursor).
		Focused(m.focus == focusLogin).
		OnInput(func(text string, cursor int) tview.Msg { return loginMsg{text, cursor} }).
		OnSubmit(focusMsg(focusPassword))
	password := textinput.New(m.password, m.passwordCursor).
		Mask("*").
		Focused(m.focus == focusPassword).
		OnInput(func(text string, cursor int) tview.Msg { return passwordMsg{text, cursor} }).
		OnSubmit(focusMsg(focusButton))
	label := "Login"
	switch {
	case m.captcha != nil:
		label = "Captcha..."
	case m.retryIn > 0:
		label = "Waiting..."
	case m.submitting:
		label = "Logging in..."
	}
	submit := button.New(text.New(label)).
		Style(buttonStyle(m.focus == focusButton, m.buttonHovered)).
		Width(layout.Fixed(len(label) + 4)).
		Focused(m.focus == focusButton).
		Hovered(m.buttonHovered).
		OnHover(func(hovered bool) tview.Msg { return hoverMsg(hovered) }).
		OnClick(submitMsg{})
	var warning tview.Widget
	switch {
	case m.captcha != nil:
		warning = text.New(m.captcha.Prompt(m.captchaTries, m.captchaOpenErr)).Style(tcell.StyleDefault.Foreground(color.Yellow))
	case m.retryIn > 0:
		warning = retryWarning(m.retryIn, m.retryReason)
	}
	return column.New(
		row.New(text.New("Login").Width(labelWidth), login).Height(layout.Shrink),
		row.New(text.New("Password").Width(labelWidth), password).Height(layout.Shrink),
		submit,
		warning,
	).Spacing(1).AlignX(tview.HorizontalAlignmentCenter)
}

// buttonStyle is reversed when focused, else underlined when hovered.
func buttonStyle(focused, hovered bool) tcell.Style {
	switch {
	case focused:
		return tcell.StyleDefault.Reverse(true)
	case hovered:
		return tcell.StyleDefault.Underline(true)
	}
	return tcell.StyleDefault
}

// retryWarning says that the one retry starts in left seconds and why.
func retryWarning(left int, reason string) tview.Widget {
	return text.New(fmt.Sprintf("Connection failed, retrying once in %d s: %s", left, reason)).
		Style(tcell.StyleDefault.Foreground(color.Yellow))
}
