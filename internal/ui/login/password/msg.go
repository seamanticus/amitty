package password

import (
	"context"
	"errors"

	"github.com/ayn2op/arikawa/v3/utils/httputil"
	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/seamanticus/discordo/internal/http"
)

type TokenMsg string

// FailedMsg ends a login attempt that did not produce a token.
type FailedMsg struct{ Err error }

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

func submitLogin(login, password string) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		headers := http.Headers()
		headers.Set("Referer", "https://discord.com/login")

		client := http.NewClient("")
		client.OnRequest = append(client.OnRequest, httputil.WithHeaders(headers))

		ctx, cancel := context.WithTimeout(context.Background(), http.AuthTimeout)
		defer cancel()

		loginResp, err := client.WithContext(ctx).Login(login, password)
		if err != nil {
			return FailedMsg{err}
		}

		if loginResp.Token != "" {
			return TokenMsg(loginResp.Token)
		}

		if loginResp.MFA {
			return FailedMsg{errors.New("multi-factor authentication is required; use token or QR login")}
		}

		return FailedMsg{errors.New("login response did not include a token")}
	})
}
