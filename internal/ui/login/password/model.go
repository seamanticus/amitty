package password

import (
	"errors"
	"fmt"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/login/form"
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
	// browser holds the cookies and installation every request shares, and how they connect.
	browser *dhttp.Browser

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

	// attempt numbers the submits, naming the login the captcha answers for.
	attempt attempt
	captcha ui.Captcha[attempt]
}

// NewModel returns an empty form whose login requests identify as build and go through browser.
func NewModel(build dhttp.Build, browser *dhttp.Browser) Model {
	return Model{build: build, browser: browser}
}

func (Model) Label() string {
	return "Password"
}

// Title names the step shown: the credentials form, or the code that follows it.
func (m Model) Title() string {
	if m.coding() {
		return "Two-Factor Authentication"
	}
	return "Credentials"
}

var _ tui.Model[Model] = Model{}

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	if m.coding() {
		return m.code.keybinds()
	}
	if m.captcha.Waiting() {
		return []keybind.Keybind{keys.Back("cancel captcha")}
	}
	return []keybind.Keybind{keys.Submit(), keys.FocusNext(), keys.FocusPrevious()}
}

func (m Model) FullHelp() [][]keybind.Keybind { return [][]keybind.Keybind{m.ShortHelp()} }

func (Model) Init() tui.Cmd { return nil }

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
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
		return m, submitLogin(m.build, m.browser, m.fingerprint, m.captcha.Solution(), m.login, m.password, msg.final)
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
			return m, submitCode(m.build, m.browser, m.code.ticket, m.code.method, m.code.text, true)
		}
		return m, m.submit(true)
	case ChallengedMsg:
		if !m.submitting {
			return m, nil
		}
		var command tui.Cmd
		m.captcha, command = m.captcha.Update(ui.Challenged(m.attempt, msg.challenge))
		return m, command
	case CaptchaMsg:
		var command tui.Cmd
		m.captcha, command = m.captcha.Update(msg)
		return m, command
	case CaptchaDoneMsg:
		return m.captchaDone(msg)
	case FailedMsg:
		m.submitting = false
		m.retryIn, m.retryReason = 0, ""
		var closing tui.Cmd
		m.captcha, closing = m.captcha.Stop()
		if _, expired := errors.AsType[expiredError](msg.Err); expired {
			m.code = codeForm{}
			m.focus = focusPassword
		} else if m.coding() {
			m.code.text, m.code.cursor = "", 0
		}
		return m, closing
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
	case tui.KeyMsg:
		switch {
		case keybind.Matches(msg, keys.FocusNext()):
			m.focus = (m.focus + 1) % focusCount
		case keybind.Matches(msg, keys.FocusPrevious()):
			m.focus = (m.focus + focusCount - 1) % focusCount
		case m.captcha.Waiting() && keybind.Matches(msg, keys.Back("")):
			// The wait then fails, which ends the login.
			return m, m.captcha.Cancel()
		}
	case submitMsg:
		if m.submitting || m.login == "" || m.password == "" {
			return m, nil
		}
		m.submitting = true
		m.attempt++
		var closing tui.Cmd
		m.captcha, closing = m.captcha.Start(m.attempt)
		return m, cmd.Batch{closing, m.submit(false)}
	}
	return m, nil
}

// submit logs in with the form, first fetching the fingerprint if there is none yet.
func (m Model) submit(final bool) tui.Cmd {
	if m.fingerprint == "" {
		return fetchFingerprint(m.build, m.browser, final)
	}
	return submitLogin(m.build, m.browser, m.fingerprint, m.captcha.Solution(), m.login, m.password, final)
}

// coding reports whether the code form is shown.
func (m Model) coding() bool { return m.code.ticket.id != "" }

func (m Model) View() tui.Widget {
	if m.coding() {
		return m.viewCode()
	}
	labelWidth := form.LabelWidth(loginLabel, passwordLabel)
	login := textinput.New(m.login, m.loginCursor).
		Focused(m.focus == focusLogin).
		OnInput(func(text string, cursor int) tui.Msg { return loginMsg{text, cursor} }).
		OnSubmit(focusMsg(focusPassword))
	password := textinput.New(m.password, m.passwordCursor).
		Mask("*").
		Focused(m.focus == focusPassword).
		OnInput(func(text string, cursor int) tui.Msg { return passwordMsg{text, cursor} }).
		OnSubmit(focusMsg(focusButton))
	label := "Log in"
	switch {
	case m.captcha.Waiting():
		label = "Captcha..."
	case m.retryIn > 0:
		label = "Waiting..."
	case m.submitting:
		label = "Logging in..."
	}
	submit := form.Button(label, m.focus == focusButton, m.buttonHovered).
		OnHover(func(hovered bool) tui.Msg { return hoverMsg(hovered) }).
		OnClick(submitMsg{})
	var warning tui.Widget
	switch {
	case m.captcha.Waiting():
		warning = text.New(m.captcha.Prompt()).Style(tcell.StyleDefault.Foreground(color.Yellow))
	case m.retryIn > 0:
		warning = retryWarning(m.retryIn, m.retryReason)
	}
	return column.New(
		form.Gap(2),
		form.Field(loginLabel, labelWidth, form.FieldWidth, login),
		form.Gap(1),
		form.Field(passwordLabel, labelWidth, form.FieldWidth, password),
		form.Gap(1),
		submit,
		warningGap(warning),
		warning,
	).AlignX(tui.HorizontalAlignmentCenter)
}

// warningGap is the empty row above warning, or nil when there is none.
func warningGap(warning tui.Widget) tui.Widget {
	if warning == nil {
		return nil
	}
	return form.Gap(1)
}

// The labels of the inputs; Discord takes a phone number in place of the email.
const (
	loginLabel    = "Email or phone number"
	passwordLabel = "Password"
)

// retryWarning says that the one retry starts in left seconds and why.
func retryWarning(left int, reason string) tui.Widget {
	return text.New(fmt.Sprintf("Connection failed, retrying once in %d s: %s", left, reason)).
		Style(tcell.StyleDefault.Foreground(color.Yellow))
}
