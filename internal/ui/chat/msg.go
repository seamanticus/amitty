package chat

import (
	"context"
	"errors"
	"github.com/seamanticus/amitty/internal/config"
	"log/slog"
	"time"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/chat/composer"
	"github.com/seamanticus/amitty/internal/ui/chat/guildstree"
	"github.com/seamanticus/amitty/internal/ui/chat/reactionspicker"
)

// ConfigChangedMsg reports that the shared config was replaced by the settings panel; Previous is what it was.
type ConfigChangedMsg struct{ Previous config.Config }

// closeCodeAuthenticationFailed is the close code of the gateway for a token it rejects.
const closeCodeAuthenticationFailed = 4004

// TokenRefreshedMsg is a new token the gateway sent in READY to replace the one the session started with.
type TokenRefreshedMsg string

// refreshToken records token as the session's and asks for it to be stored, if it is new.
func (m *Model) refreshToken(token string) tui.Cmd {
	if token == "" || token == m.token {
		return nil
	}
	m.token = token
	return cmd.Task(func() tui.Msg { return TokenRefreshedMsg(token) })
}

// openTimeout bounds the wait for READY; the session keeps reconnecting on its own afterwards.
const openTimeout = time.Minute

func openState(state *ningen.State) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), openTimeout)
		defer cancel()

		err := state.Open(ctx)
		if err == nil {
			return nil
		}

		// A rejected token never works again, so say so over the login.
		if isAuthenticationFailed(err) {
			return LogoutMsg{What: "open the chat", Err: rejectedTokenError{err}}
		}

		// The deadline closes the session, so the chat would otherwise sit empty with no reason given.
		return ui.ErrorDialog("open chat state", err)
	})
}

func isAuthenticationFailed(err error) bool {
	event, ok := errors.AsType[*ws.CloseEvent](err)
	return ok && event.Code == closeCodeAuthenticationFailed
}

// rejectedTokenError is a stored token the gateway refused (4004): its session was logged out or ended elsewhere.
type rejectedTokenError struct{ err error }

func (e rejectedTokenError) Error() string {
	return "Discord no longer accepts the stored token: its session was logged out or ended, or the password changed. Log in again."
}

func (e rejectedTokenError) Unwrap() error { return e.err }

// LogValue keeps the gateway's own error in the log, which the message shown leaves out.
func (e rejectedTokenError) LogValue() slog.Value {
	return slog.GroupValue(slog.String("message", e.Error()), slog.String("cause", e.err.Error()))
}

// unconfirmedLogoutError is a logout Discord did not confirm, whose session may still be open.
type unconfirmedLogoutError struct{ err error }

func (e unconfirmedLogoutError) Error() string {
	return e.err.Error() + ". The session may still be open: end it in the web client, under User Settings, Devices."
}

func (e unconfirmedLogoutError) Unwrap() error { return e.err }

// closeState stops the events, then closes the session, if any, before sending next, so that next comes after the session is gone.
func closeState(state *ningen.State, stopEvents func(), next tui.Msg) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		shut(state, stopEvents)
		return next
	})
}

// shut stops the events, then closes the session, if any.
func shut(state *ningen.State, stopEvents func()) {
	if stopEvents != nil {
		stopEvents()
	}
	if state != nil {
		if err := state.Close(); err != nil {
			slog.Error("failed to close the session", "err", err)
		}
	}
}

// logoutTimeout bounds the wait for Discord to end the token.
const logoutTimeout = 10 * time.Second

// logOut closes the session, then asks Discord through client to end its token, as the web client does; the login shows either way, with the error if Discord did not answer.
func logOut(state *ningen.State, stopEvents func(), client *api.Client) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		shut(state, stopEvents)
		ctx, cancel := context.WithTimeout(context.Background(), logoutTimeout)
		defer cancel()
		if err := client.WithContext(ctx).Logout(); err != nil {
			return LogoutMsg{What: "log out at Discord", Err: unconfirmedLogoutError{err}}
		}
		return LogoutMsg{}
	})
}

// LoggingOutMsg starts a logout; until its LogoutMsg comes, the screen says so and takes no input, so quitting cannot leave the session open.
type LoggingOutMsg struct{}

// listen waits for the next event, or for ctx to end, which sends nothing.
func listen(ctx context.Context, events *handler.Queue) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		ev, ok := events.Pop(ctx)
		if !ok {
			return nil
		}
		return ev
	})
}

// LogoutMsg asks for the login screen, with Err, if set, as why the session ended or what went wrong on the way, in What.
type LogoutMsg struct {
	What string
	Err  error
}

type (
	QuitMsg        struct{}
	StateClosedMsg struct{}
)

func (m *Model) setSelectedChannel(channel *discord.Channel) tui.Cmd {
	m.selectedChannel = channel
	m.composer.SetChannel(channel)
	cmd := m.membersTree.SetChannel(channel)
	if !m.canFocus(m.focused) {
		m.focused = paneMessagesList
	}
	return cmd
}

func (m *Model) navigateToChannel(channelID discord.ChannelID) tui.Cmd {
	m.closePicker()
	var cmd tui.Cmd
	m.guildsTree, cmd = m.guildsTree.Update(guildstree.NavigateMsg{ChannelID: channelID})
	return cmd
}

// paneMsg is what a pane's widget made of a mouse message within it. A left mouse button press also focuses the pane.
type paneMsg struct {
	pane  pane
	msg   tui.Msg
	focus bool
}

// typingExpiredMsg ends a user's typing indicator unless they typed again and extended it.
type typingExpiredMsg struct {
	userID discord.UserID
}

// addTyper shows userID as typing and returns a command that ends it after the typing duration.
func (m Model) addTyper(userID discord.UserID) tui.Cmd {
	m.typers[userID] = time.Now().Add(composer.TypingDuration)
	return cmd.Task(func() tui.Msg {
		time.Sleep(composer.TypingDuration)
		return typingExpiredMsg{userID}
	})
}

// react reacts as msg asks. The reaction is shown once the gateway reports it.
func (m Model) react(msg reactionspicker.SelectedMsg) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if err := m.state.React(msg.ChannelID, msg.MessageID, msg.Emoji); err != nil {
			slog.Error("failed to react", "channel_id", msg.ChannelID, "message_id", msg.MessageID, "emoji", msg.Emoji, "err", err)
		}
		return nil
	})
}
