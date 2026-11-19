package password

import (
	"errors"
	"strings"
	"testing"

	"github.com/ayn2op/tview"
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// codeHeight fits the code form.
const codeHeight = 13

// codingModel returns a model that submitted a password and was asked for a code allowing methods.
func codingModel(t *testing.T, methods ...method) Model {
	t.Helper()
	tk := ticket{id: "tk", instance: "li"}
	for _, m := range methods {
		tk.allowed[m] = true
	}
	m := Model{login: "u", password: "hunter2", passwordCursor: 7}
	m, _ = m.Update(submitMsg{})
	m, _ = m.Update(MFAMsg{tk})
	return m
}

func sendCode(m *Model, msg tview.Msg) { send(m, msg) }

func TestModelMFA(t *testing.T) {
	t.Run("the code step clears the password and unlocks", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		if m.password != "" || m.passwordCursor != 0 || m.submitting || !m.coding() {
			t.Fatalf("password = %q, cursor = %d, submitting = %v, coding = %v", m.password, m.passwordCursor, m.submitting, m.coding())
		}
		if m.code.focus != codeFocusInput || m.login != "u" {
			t.Fatalf("focus = %d, login = %q", m.code.focus, m.login)
		}
	})

	t.Run("the first allowed method is picked", func(t *testing.T) {
		if got := codingModel(t, methodBackup, methodSMS).code.method; got != methodSMS {
			t.Fatalf("method = %d, want SMS", got)
		}
	})

	t.Run("typing fills the code and Enter verifies once", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		for _, r := range "123456" {
			sendCode(&m, key(tcell.KeyRune, string(r)))
		}
		if m.code.text != "123456" {
			t.Fatalf("code = %q", m.code.text)
		}
		if got := m.View().Handle(key(tcell.KeyEnter, ""), area); got != (verifyMsg{}) {
			t.Fatalf("Enter = %#v, want verifyMsg", got)
		}
		m, command := m.Update(verifyMsg{})
		if command == nil || !m.submitting {
			t.Fatal("verify started no request")
		}
		if _, command := m.Update(verifyMsg{}); command != nil {
			t.Fatal("second verify started a request")
		}
	})

	t.Run("an empty code does not verify", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		m.code.text = " - "
		if _, command := m.Update(verifyMsg{}); command != nil {
			t.Fatal("empty code started a request")
		}
	})

	t.Run("a wrong code keeps the step and clears the input", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		m.code.text, m.code.cursor = "000000", 6
		m, _ = m.Update(verifyMsg{})
		m, _ = m.Update(FailedMsg{&httputil.HTTPError{Status: 400, Code: 60008}})
		if !m.coding() || m.submitting || m.code.text != "" || m.code.cursor != 0 {
			t.Fatalf("coding = %v, submitting = %v, code = %q", m.coding(), m.submitting, m.code.text)
		}
	})

	t.Run("an expired ticket returns to the form", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		m.code.text = "123456"
		m, _ = m.Update(verifyMsg{})
		m, _ = m.Update(failed(&httputil.HTTPError{Status: 400, Code: codeInvalidTicket}))
		if m.coding() || m.submitting || m.focus != focusPassword || m.login != "u" {
			t.Fatalf("coding = %v, submitting = %v, focus = %d, login = %q", m.coding(), m.submitting, m.focus, m.login)
		}
	})

	t.Run("Esc returns to the form unless a request is in flight", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		m.code.text = "1"
		m, _ = m.Update(verifyMsg{})
		sendCode(&m, key(tcell.KeyEsc, ""))
		if !m.coding() {
			t.Fatal("Esc left the step while verifying")
		}
		m, _ = m.Update(FailedMsg{errors.New("wrong")})
		sendCode(&m, key(tcell.KeyEsc, ""))
		if m.coding() || m.focus != focusPassword {
			t.Fatalf("coding = %v, focus = %d", m.coding(), m.focus)
		}
	})

	t.Run("Left and Right switch among allowed methods on the picker", func(t *testing.T) {
		m := codingModel(t, methodTOTP, methodBackup)
		sendCode(&m, key(tcell.KeyRight, ""))
		if m.code.method != methodTOTP {
			t.Fatal("Right in the input switched the method")
		}
		sendCode(&m, key(tcell.KeyBacktab, ""))
		sendCode(&m, key(tcell.KeyRight, ""))
		if m.code.method != methodBackup {
			t.Fatalf("method = %d, want backup", m.code.method)
		}
		sendCode(&m, key(tcell.KeyRight, ""))
		if m.code.method != methodTOTP {
			t.Fatalf("method = %d, want TOTP after wrapping", m.code.method)
		}
		sendCode(&m, key(tcell.KeyLeft, ""))
		if m.code.method != methodBackup {
			t.Fatalf("method = %d, want backup", m.code.method)
		}
	})

	t.Run("the send button is focusable only for SMS", func(t *testing.T) {
		for _, tt := range []struct {
			methods []method
			want    bool
		}{{[]method{methodTOTP}, false}, {[]method{methodSMS}, true}} {
			m := codingModel(t, tt.methods...)
			seen := false
			for range codeFocusCount {
				sendCode(&m, key(tcell.KeyTab, ""))
				seen = seen || m.code.focus == codeFocusSMS
			}
			if seen != tt.want {
				t.Fatalf("methods %v: send focused = %v, want %v", tt.methods, seen, tt.want)
			}
		}
	})

	t.Run("Send SMS asks once and shows the phone", func(t *testing.T) {
		m := codingModel(t, methodSMS)
		m, command := m.Update(sendSMSMsg{})
		if command == nil || !m.submitting {
			t.Fatal("send started no request")
		}
		if _, command := m.Update(sendSMSMsg{}); command != nil {
			t.Fatal("second send started a request")
		}
		m, _ = m.Update(SMSSentMsg{"+***1234"})
		if m.submitting || m.code.phone != "+***1234" {
			t.Fatalf("submitting = %v, phone = %q", m.submitting, m.code.phone)
		}
		if drawn := discordtest.Draw(t, m.View(), area.Width, codeHeight); !strings.Contains(drawn, "+***1234") {
			t.Fatalf("phone not drawn:\n%s", drawn)
		}
	})

	t.Run("Send SMS does nothing for another method", func(t *testing.T) {
		m := codingModel(t, methodTOTP, methodSMS)
		if _, command := m.Update(sendSMSMsg{}); command != nil {
			t.Fatal("send started a request with TOTP picked")
		}
	})

	t.Run("a failed connection counts down and retries the code", func(t *testing.T) {
		m := codingModel(t, methodTOTP)
		m.code.text = "123456"
		m, _ = m.Update(verifyMsg{})
		reason := errors.New("refused")
		m, command := m.Update(CountdownMsg{left: 1, err: reason})
		if command == nil || m.retryIn != 1 {
			t.Fatalf("retryIn = %d, command = %v", m.retryIn, command)
		}
		m, command = m.Update(CountdownMsg{left: 0, err: reason})
		if command == nil || !m.submitting || !m.coding() {
			t.Fatalf("retry: command = %v, submitting = %v", command, m.submitting)
		}
	})

	t.Run("the view lists only allowed methods", func(t *testing.T) {
		drawn := discordtest.Draw(t, codingModel(t, methodTOTP, methodBackup).View(), area.Width, codeHeight)
		for label, want := range map[string]bool{methodTOTP.label(): true, methodBackup.label(): true, "Send SMS": false, "esc: back": true} {
			if strings.Contains(drawn, label) != want {
				t.Errorf("%q drawn = %v, want %v:\n%s", label, !want, want, drawn)
			}
		}
	})
}
