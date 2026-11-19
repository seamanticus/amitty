package password

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	dhttp "github.com/seamanticus/discordo/internal/http"
	"github.com/seamanticus/dmwire/api"
	"github.com/seamanticus/dmwire/utils/httputil"
)

// retryDelay is how long a login whose connection failed waits before its one retry.
const retryDelay = 5

type TokenMsg string

// FailedMsg ends a login attempt that did not produce a token.
type FailedMsg struct{ Err error }

// CountdownMsg counts down to the one retry of a login whose connection failed, with left seconds to go.
type CountdownMsg struct {
	left int
	err  error
}

// inputMsg is the text and cursor that typing in an input left.
type inputMsg struct {
	text   string
	cursor int
}

type (
	loginMsg    inputMsg
	passwordMsg inputMsg
	focusMsg    focus
	hoverMsg    bool
	submitMsg   struct{}
)

// submitLogin posts the credentials once with fingerprint. A first attempt whose connection failed starts the countdown to a retry; a final one fails.
func submitLogin(build dhttp.Build, fingerprint, login, password string, final bool) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dhttp.AuthTimeout)
		defer cancel()
		return countdownOr(requestLogin(ctx, authClient(dhttp.NewClient(""), build, fingerprint), login, password), final)
	})
}

// countdownOr returns msg, or the countdown to a retry when msg is a first attempt whose connection failed.
func countdownOr(msg tview.Msg, final bool) tview.Msg {
	if failed, ok := msg.(FailedMsg); ok && !final && retryable(failed.Err) {
		return CountdownMsg{left: retryDelay, err: failed.Err}
	}
	return msg
}

// tick sends the countdown one second later.
func tick(msg CountdownMsg) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		time.Sleep(time.Second)
		return CountdownMsg{left: msg.left - 1, err: msg.err}
	})
}

// retryable reports whether err means the login did not reach Discord or a proxy in front of it failed, so posting again is not a second guess at the password.
func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if httpErr, ok := errors.AsType[*httputil.HTTPError](err); ok {
		switch httpErr.Status {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	_, ok := errors.AsType[httputil.RequestError](err)
	return ok
}

// authClient makes client send the headers of the web client's login page, with fingerprint if set, and post once: retrying is the login's decision, not the transport's.
func authClient(client *api.Client, build dhttp.Build, fingerprint string) *api.Client {
	headers := dhttp.Headers(build)
	headers.Set("Referer", "https://discord.com/login")
	if fingerprint != "" {
		headers.Set("X-Fingerprint", fingerprint)
	}
	client.OnRequest = append(client.OnRequest, httputil.WithHeaders(headers))
	// 0 would retry forever.
	client.Retries = 1
	return client
}

// requestLogin posts the credentials with client and returns a TokenMsg, an MFAMsg or a FailedMsg.
func requestLogin(ctx context.Context, client *api.Client, login, password string) tview.Msg {
	loginResp, err := client.WithContext(ctx).Login(login, password)
	if err != nil {
		return FailedMsg{err}
	}

	if loginResp.Token != "" {
		return TokenMsg(loginResp.Token)
	}

	if loginResp.MFA {
		if loginResp.Ticket == "" {
			return FailedMsg{errors.New("login response did not include a two-factor ticket")}
		}
		if t, ok := ticketFromLogin(loginResp); ok {
			return MFAMsg{t}
		}
		return FailedMsg{errors.New("this account only allows a security key as second factor; use QR login")}
	}

	return FailedMsg{errors.New("login response did not include a token")}
}
