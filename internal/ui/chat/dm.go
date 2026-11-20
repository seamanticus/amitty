package chat

import (
	"log/slog"
	"time"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
)

// newDMLimit and newDMWindow bound how many new DMs amitty starts, as opening DMs is what Discord's spam filter watches. The count is kept in memory only.
const (
	newDMLimit  = 3
	newDMWindow = time.Hour
)

// dmStarts are the times new DMs were started, oldest first.
type dmStarts []time.Time

// allow reports whether another DM may start at now, recording it if so, and keeps only the starts within the window.
func (s *dmStarts) allow(now time.Time) bool {
	kept := (*s)[:0]
	for _, start := range *s {
		if now.Sub(start) < newDMWindow {
			kept = append(kept, start)
		}
	}
	*s = kept
	if len(kept) >= newDMLimit {
		return false
	}
	*s = append(kept, now)
	return true
}

type (
	// startDMMsg asks to start a new DM with userID, once confirmed.
	startDMMsg struct{ userID discord.UserID }
	// dmStartedMsg reports the DM Discord made, to open it.
	dmStartedMsg struct{ channelID discord.ChannelID }
)

// openDM opens your DM with userID, or asks whether to start one, defaulting to No. name is who it is with, for the question.
func (m *Model) openDM(userID discord.UserID, name string) tui.Cmd {
	if ui.IsMe(m.state, userID) {
		return nil
	}
	if channel, ok := ui.DirectMessage(m.state, userID); ok && !channel.IsSpam {
		return m.navigateToChannel(channel.ID)
	}
	start := cmd.Task(func() tui.Msg { return startDMMsg{userID} })
	return ui.ShowDialog("Start a DM with "+name+"?", ui.DialogButton{Label: "No"}, ui.DialogButton{Label: "Yes", Cmd: start})
}

// startDM starts a DM with userID unless newDMLimit were started within newDMWindow of now.
func (m *Model) startDM(userID discord.UserID, now time.Time) tui.Cmd {
	if !m.dmStarts.allow(now) {
		return ui.ShowDialog("You started 3 new DMs in the last hour. Try again later.", ui.DialogButton{Label: "OK"})
	}
	return createDM(m.state, userID)
}

func createDM(state *ningen.State, userID discord.UserID) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		// The client's, not the store's of the same name, which only looks the DM up.
		channel, err := state.Client.CreatePrivateChannel(userID)
		if err != nil {
			return ui.ErrorDialog("start the DM", err)
		}
		// The gateway's Channel Create follows; storing the answer now lists the DM at once.
		if err := state.Cabinet.ChannelSet(channel, true); err != nil {
			slog.Error("failed to store the new DM", "channel_id", channel.ID, "err", err)
		}
		return dmStartedMsg{channel.ID}
	})
}
