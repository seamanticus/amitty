package root

import (
	"github.com/seamanticus/amitty/internal/ui/chat"
	"github.com/seamanticus/amitty/internal/ui/login"
)

// screenTag says which case of screen is set; only Match reads it.
type screenTag uint8

const (
	screenTagLogin screenTag = iota
	screenTagChat
)

// screen is the model shown above help: the login or the chat, never both and never neither. Its zero value is the login, which is shown whenever no token is known.
type screen struct {
	tag   screenTag
	login login.Model
	chat  chat.Model
}

func loginScreen(m login.Model) screen { return screen{tag: screenTagLogin, login: m} }
func chatScreen(m chat.Model) screen   { return screen{tag: screenTagChat, chat: m} }

// Match returns onLogin or onChat applied to the model shown.
func (s screen) Match[R any](onLogin func(login.Model) R, onChat func(chat.Model) R) R {
	switch s.tag {
	case screenTagLogin:
		return onLogin(s.login)
	case screenTagChat:
		return onChat(s.chat)
	}
	panic("invariant: unknown screen tag")
}

// isChat reports whether the chat is shown.
func (s screen) isChat() bool {
	return s.Match(func(login.Model) bool { return false }, func(chat.Model) bool { return true })
}
