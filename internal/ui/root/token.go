package root

import (
	"context"
	"log/slog"
	"time"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/seamanticus/discordo/internal/tokenstore"
	"github.com/seamanticus/discordo/internal/ui"
)

// tokenMsg opens the chat with a token.
type tokenMsg string

// loginMsg shows the login, as no token is stored.
type loginMsg struct{}

// storeTimeout bounds one run of the encrypt command.
const storeTimeout = time.Minute

// storeToken saves token, then sends it on to open the chat. A failed save is shown over the chat, which opens anyway: the session needs no stored copy.
func storeToken(store tokenstore.Store, token string) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		if err := saveToken(store, token); err != nil {
			return storedMsg{token: token, err: err}
		}
		return storedMsg{token: token}
	})
}

// storedMsg is a token after its save, with err set if the save failed.
type storedMsg struct {
	token string
	err   error
}

// saveRefreshed saves a token the gateway sent to replace the stored one, reporting only a failure.
func saveRefreshed(store tokenstore.Store, token string) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		if err := saveToken(store, token); err != nil {
			return ui.ErrorDialog("store the refreshed token", err)
		}
		slog.Info("stored the refreshed token")
		return nil
	})
}

func saveToken(store tokenstore.Store, token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return store.Save(ctx, token)
}

// deleteToken removes the stored token, reporting only a failure.
func deleteToken(store tokenstore.Store) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		if err := store.Delete(); err != nil {
			return ui.ErrorDialog("delete the stored token", err)
		}
		return nil
	})
}
