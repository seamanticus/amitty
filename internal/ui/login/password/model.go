package password

import (
	"fmt"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/button"
	"github.com/ayn2op/tview/column"
	"github.com/ayn2op/tview/layout"
	"github.com/ayn2op/tview/row"
	"github.com/ayn2op/tview/text"
	"github.com/ayn2op/tview/textinput"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	dhttp "github.com/seamanticus/discordo/internal/http"
)

// focus is the input or button that receives keys.
type focus int

const (
	focusLogin focus = iota
	focusPassword
	focusButton
	focusCount
)

// Model is a login and a password input above a button. Tab and the arrow keys move the focus, Enter in an input moves to the next one, and Enter on the button or clicking it logs in.
type Model struct {
	// build is what the login requests identify as.
	build dhttp.Build

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
}

// NewModel returns an empty form whose login requests identify as build.
func NewModel(build dhttp.Build) Model {
	return Model{build: build}
}

func (Model) Label() string {
	return "Password"
}

var _ tview.Model[Model] = Model{}

func (Model) Init() tview.Cmd { return nil }

func (m Model) Update(msg tview.Msg) (Model, tview.Cmd) {
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
		switch msg.Key() {
		case tcell.KeyTab, tcell.KeyDown:
			m.focus = (m.focus + 1) % focusCount
		case tcell.KeyBacktab, tcell.KeyUp:
			m.focus = (m.focus + focusCount - 1) % focusCount
		}
	case submitMsg:
		if m.submitting || m.login == "" || m.password == "" {
			return m, nil
		}
		m.submitting = true
		return m, submitLogin(m.build, m.login, m.password, false)
	case CountdownMsg:
		if !m.submitting {
			return m, nil
		}
		if msg.left > 0 {
			m.retryIn, m.retryReason = msg.left, msg.err.Error()
			return m, tick(msg)
		}
		m.retryIn, m.retryReason = 0, ""
		return m, submitLogin(m.build, m.login, m.password, true)
	case FailedMsg:
		m.submitting = false
		m.retryIn, m.retryReason = 0, ""
	}
	return m, nil
}

func (m Model) View() tview.Widget {
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
	buttonStyle := tcell.StyleDefault
	switch {
	case m.focus == focusButton:
		buttonStyle = buttonStyle.Reverse(true)
	case m.buttonHovered:
		buttonStyle = buttonStyle.Underline(true)
	}
	label := "Login"
	switch {
	case m.retryIn > 0:
		label = "Waiting..."
	case m.submitting:
		label = "Logging in..."
	}
	submit := button.New(text.New(label)).
		Style(buttonStyle).
		Width(layout.Fixed(len(label) + 4)).
		Focused(m.focus == focusButton).
		Hovered(m.buttonHovered).
		OnHover(func(hovered bool) tview.Msg { return hoverMsg(hovered) }).
		OnClick(submitMsg{})
	var warning tview.Widget
	if m.retryIn > 0 {
		warning = text.New(fmt.Sprintf("Connection failed, retrying once in %d s: %s", m.retryIn, m.retryReason)).
			Style(tcell.StyleDefault.Foreground(color.Yellow))
	}
	return column.New(
		row.New(text.New("Login").Width(labelWidth), login).Height(layout.Shrink),
		row.New(text.New("Password").Width(labelWidth), password).Height(layout.Shrink),
		submit,
		warning,
	).Spacing(1).AlignX(tview.HorizontalAlignmentCenter)
}
