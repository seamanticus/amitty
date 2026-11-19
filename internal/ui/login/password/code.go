package password

import (
	"strings"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/button"
	"github.com/ayn2op/tview/column"
	"github.com/ayn2op/tview/keybind"
	"github.com/ayn2op/tview/layout"
	"github.com/ayn2op/tview/row"
	"github.com/ayn2op/tview/text"
	"github.com/ayn2op/tview/textinput"
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/discordo/internal/ui/login/keys"
)

// codeFocus is the control of the code form that receives keys.
type codeFocus uint8

const (
	codeFocusMethod codeFocus = iota
	codeFocusInput
	codeFocusVerify
	codeFocusSMS
	codeFocusCount
)

// codeForm is the second step of a login: a method picker, a code input, a verify button and, for SMS, a send button.
type codeForm struct {
	ticket ticket
	method method
	text   string
	// cursor is the byte offset of the cursor in text.
	cursor int
	focus  codeFocus
	// phone is the redacted number the last SMS code went to; empty before one is sent.
	phone string
}

type (
	codeMsg      inputMsg
	codeFocusMsg codeFocus
	verifyMsg    struct{}
	sendSMSMsg   struct{}
)

// focusable reports whether f can take the focus, as the send button exists only for SMS.
func (c codeForm) focusable(f codeFocus) bool {
	return f != codeFocusSMS || c.method == methodSMS
}

// moveFocus returns the next focusable control step places after the focused one, wrapping around. step is 1 for forward and codeFocusCount-1 for backward.
func (c codeForm) moveFocus(step codeFocus) codeFocus {
	f := c.focus
	for range codeFocusCount {
		if f = (f + step) % codeFocusCount; c.focusable(f) {
			return f
		}
	}
	return c.focus
}

// keybinds are those of the code form, the method keys only while the picker has the focus.
func (c codeForm) keybinds() []keybind.Keybind {
	binds := []keybind.Keybind{keys.FocusNext(), keys.FocusPrevious()}
	if c.focus == codeFocusMethod {
		binds = append(binds, keys.OptionPrevious(), keys.OptionNext())
	}
	return append(binds, keys.Back("back"))
}

func (m Model) updateCode(msg tview.Msg) (Model, tview.Cmd) {
	switch msg := msg.(type) {
	case codeMsg:
		m.code.text, m.code.cursor = msg.text, msg.cursor
	case codeFocusMsg:
		m.code.focus = codeFocus(msg)
	case tview.KeyMsg:
		switch {
		case keybind.Matches(msg, keys.FocusNext()):
			m.code.focus = m.code.moveFocus(1)
		case keybind.Matches(msg, keys.FocusPrevious()):
			m.code.focus = m.code.moveFocus(codeFocusCount - 1)
		case m.code.focus == codeFocusMethod && keybind.Matches(msg, keys.OptionNext()):
			m.code.method = m.code.ticket.next(m.code.method, 1)
		case m.code.focus == codeFocusMethod && keybind.Matches(msg, keys.OptionPrevious()):
			m.code.method = m.code.ticket.next(m.code.method, methodCount-1)
		case keybind.Matches(msg, keys.Back("")):
			// A request in flight finishes first, so its answer cannot land on the form.
			if !m.submitting {
				m.code = codeForm{}
				m.focus = focusPassword
			}
		}
	case verifyMsg:
		if m.submitting || normalizeCode(m.code.text) == "" {
			return m, nil
		}
		m.submitting = true
		return m, submitCode(m.build, m.code.ticket, m.code.method, m.code.text, false)
	case sendSMSMsg:
		if m.submitting || m.code.method != methodSMS {
			return m, nil
		}
		m.submitting = true
		return m, sendSMS(m.build, m.code.ticket)
	}
	return m, nil
}

func (m Model) viewCode() tview.Widget {
	c := m.code
	methods := row.New().Spacing(2)
	for method := range methodCount {
		if !c.ticket.allowed[method] {
			continue
		}
		style := tcell.StyleDefault
		if method == c.method {
			style = style.Bold(true).Underline(true)
			if c.focus == codeFocusMethod {
				style = style.Reverse(true)
			}
		}
		methods = methods.Push(text.New(method.label()).Style(style))
	}

	input := textinput.New(c.text, c.cursor).
		Focused(c.focus == codeFocusInput).
		OnInput(func(text string, cursor int) tview.Msg { return codeMsg{text, cursor} }).
		OnSubmit(verifyMsg{})

	label := "Verify"
	switch {
	case m.retryIn > 0:
		label = "Waiting..."
	case m.submitting:
		label = "Verifying..."
	}
	buttons := row.New(codeButton(label, c.focus == codeFocusVerify, verifyMsg{})).Spacing(2).Height(layout.Shrink)
	if c.method == methodSMS {
		buttons = buttons.Push(codeButton("Send SMS", c.focus == codeFocusSMS, sendSMSMsg{}))
	}

	var status []string
	if c.method == methodSMS && c.phone != "" {
		status = append(status, "Code sent to "+c.phone)
	}
	status = append(status, keys.Back("").Help().Key+": back")
	var warning tview.Widget
	if m.retryIn > 0 {
		warning = retryWarning(m.retryIn, m.retryReason)
	}
	return column.New(
		text.New("Two-factor code required"),
		methods.Height(layout.Shrink),
		row.New(text.New("Code").Width(layout.Fixed(len("Code")+1)), input).Height(layout.Shrink),
		buttons,
		text.New(strings.Join(status, " - ")),
		warning,
	).Spacing(1).AlignX(tview.HorizontalAlignmentCenter)
}

// codeButton is a button of the code form that sends onClick.
func codeButton(label string, focused bool, onClick tview.Msg) tview.Widget {
	return button.New(text.New(label)).
		Style(buttonStyle(focused, false)).
		Width(layout.Fixed(len(label) + 4)).
		Focused(focused).
		OnClick(onClick)
}
