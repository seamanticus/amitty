package token

import (
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

type TokenMsg string

// tokenMsg is the token and cursor that typing in the input left.
type tokenMsg struct {
	text   string
	cursor int
}

type (
	focusButtonMsg struct{}
	hoverMsg       bool
	submitMsg      struct{}
)

func submitToken(token string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		return TokenMsg(token)
	})
}
