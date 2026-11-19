package password

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/dmwire/api"
	"github.com/seamanticus/dmwire/utils/httputil"
)

// method is a second factor that a two-factor login accepts.
type method uint8

const (
	methodTOTP method = iota
	methodSMS
	methodBackup
	methodCount
)

// label is how the method picker names m.
func (m method) label() string {
	labels := [...]string{methodTOTP: "Authenticator", methodSMS: "SMS", methodBackup: "Backup code"}
	// proof: compile-time
	_ = [1]struct{}{}[len(labels)-int(methodCount)]
	return labels[m]
}

// path is the authenticator type of m in /auth/mfa/{type}.
func (m method) path() string {
	paths := [...]string{methodTOTP: "totp", methodSMS: "sms", methodBackup: "backup"}
	// proof: compile-time
	_ = [1]struct{}{}[len(paths)-int(methodCount)]
	return paths[m]
}

// JSON error codes that reject the ticket itself rather than the code.
const (
	codeInvalidTicket  httputil.ErrorCode = 60006
	codeInvalidSession httputil.ErrorCode = 60009
)

// ticket is the pending second step of a login that the password alone did not complete; fingerprint is what the login was sent with, which the step must repeat.
type ticket struct {
	id, instance string
	fingerprint  string
	allowed      [methodCount]bool
}

// ticketFromLogin returns the ticket of a login that needs a code, and false when resp offers no method this client supports.
func ticketFromLogin(resp *api.LoginResponse) (ticket, bool) {
	t := ticket{id: resp.Ticket, instance: resp.LoginInstanceID}
	t.allowed[methodTOTP], t.allowed[methodSMS], t.allowed[methodBackup] = resp.TOTP, resp.SMS, resp.Backup
	return t, t.allowed != [methodCount]bool{}
}

// first returns the first allowed method in picker order.
func (t ticket) first() method {
	for m := range methodCount {
		if t.allowed[m] {
			return m
		}
	}
	return methodTOTP
}

// next returns the allowed method step places after m, wrapping around, or m when no other is allowed. step is 1 for forward and methodCount-1 for backward.
func (t ticket) next(m, step method) method {
	for range methodCount {
		m = (m + step) % methodCount
		if t.allowed[m] {
			return m
		}
	}
	return m
}

// MFAMsg asks for the second factor of a login.
type MFAMsg struct{ ticket ticket }

// SMSSentMsg reports the redacted phone number that an SMS code was sent to.
type SMSSentMsg struct{ phone string }

// expiredError is a rejected ticket, after which the login starts over.
type expiredError struct{ err error }

func (e expiredError) Error() string {
	return "two-factor session expired, log in again: " + e.err.Error()
}
func (e expiredError) Unwrap() error { return e.err }

// failed returns err as a FailedMsg, marking a rejected ticket as expired.
func failed(err error) FailedMsg {
	if httpErr, ok := errors.AsType[*httputil.HTTPError](err); ok && (httpErr.Code == codeInvalidTicket || httpErr.Code == codeInvalidSession) {
		return FailedMsg{expiredError{err}}
	}
	return FailedMsg{err}
}

// submitCode posts the code once, with the same retry rule as the password.
func submitCode(build dhttp.Build, t ticket, m method, code string, final bool) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dhttp.AuthTimeout)
		defer cancel()
		return countdownOr(requestCode(ctx, authClient(dhttp.NewClient(""), build, t.fingerprint), t, m, code), final)
	})
}

// normalizeCode drops the spaces and dashes that codes are displayed with.
func normalizeCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, code)
}

// requestCode posts code for method m of t with client and returns a TokenMsg or a FailedMsg.
func requestCode(ctx context.Context, client *api.Client, t ticket, m method, code string) tview.Msg {
	body := struct {
		Code            string `json:"code"`
		Ticket          string `json:"ticket"`
		LoginInstanceID string `json:"login_instance_id,omitempty"`
	}{normalizeCode(code), t.id, t.instance}
	resp, err := client.WithContext(ctx).RequestJSON[*api.LoginResponse](http.MethodPost, api.EndpointMFA+m.path(), httputil.WithJSONBody(body))
	if err != nil {
		return failed(err)
	}
	if resp == nil || resp.Token == "" {
		return FailedMsg{errors.New("two-factor response did not include a token")}
	}
	return TokenMsg(resp.Token)
}

// sendSMS asks Discord once to text a code for t; a failure is not retried, so no second SMS is sent.
func sendSMS(build dhttp.Build, t ticket) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dhttp.AuthTimeout)
		defer cancel()
		return requestSMS(ctx, authClient(dhttp.NewClient(""), build, t.fingerprint), t)
	})
}

// requestSMS asks with client for an SMS code for t and returns an SMSSentMsg or a FailedMsg.
func requestSMS(ctx context.Context, client *api.Client, t ticket) tview.Msg {
	phone, err := client.WithContext(ctx).SendMFASMS(t.id)
	if err != nil {
		return failed(err)
	}
	return SMSSentMsg{phone}
}
