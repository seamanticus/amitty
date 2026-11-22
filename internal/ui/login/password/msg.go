package password

import (
	"context"
	"errors"
	"time"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

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

// submitLogin posts the credentials once with fingerprint and, if set, the captcha solution. A first attempt whose connection failed starts the countdown to a retry; a final one fails.
func submitLogin(build dhttp.Build, browser *dhttp.Browser, fingerprint string, solution captcha.Solution, login, password string, final bool) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dhttp.AuthTimeout)
		defer cancel()
		return countdownOr(requestLogin(ctx, withSolution(authClient(browser.Client(""), build, fingerprint), solution), login, password), final)
	})
}

// withSolution makes client send solution, the answer to the captcha its last request got.
func withSolution(client *api.Client, solution captcha.Solution) *api.Client {
	client.OnRequest = append(client.OnRequest, solution.Option())
	return client
}

// countdownOr returns msg, or the countdown to a retry when msg is a first attempt whose connection failed.
func countdownOr(msg tui.Msg, final bool) tui.Msg {
	if failed, ok := msg.(FailedMsg); ok && !final && dhttp.Retryable(failed.Err) {
		return CountdownMsg{left: dhttp.RetryDelay, err: failed.Err}
	}
	return msg
}

// tick sends the countdown one second later.
func tick(msg CountdownMsg) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		time.Sleep(time.Second)
		return CountdownMsg{left: msg.left - 1, err: msg.err}
	})
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

// requestLogin posts the credentials with client and returns a TokenMsg, an MFAMsg, a ChallengedMsg or a FailedMsg.
func requestLogin(ctx context.Context, client *api.Client, login, password string) tui.Msg {
	loginResp, err := client.WithContext(ctx).Login(login, password)
	if challenge, ok := captcha.FromError(err); ok {
		return ChallengedMsg{challenge}
	}
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
