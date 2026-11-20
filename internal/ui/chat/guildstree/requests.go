package guildstree

import (
	"log/slog"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
)

// requestLabel marks a DM that waits in your message requests.
const requestLabel = " (request)"

// requestsChangedMsg reports that a message request was accepted or rejected, so the DMs are listed again.
type requestsChangedMsg struct{}

// currentRequest returns the message request highlighted, if one is.
func (m Model) currentRequest() (discord.Channel, bool) {
	if m.current == nil {
		return discord.Channel{}, false
	}
	id, ok := m.current.Reference().(discord.ChannelID)
	if !ok {
		return discord.Channel{}, false
	}
	channel, err := m.state.Cabinet.Channel(id)
	if err != nil || !channel.IsMessageRequest {
		return discord.Channel{}, false
	}
	return *channel, true
}

// confirmRequest asks whether to accept, or reject, the message request channel, defaulting to No.
func (m Model) confirmRequest(channel discord.Channel, accept bool) tui.Cmd {
	name := ui.ChannelToString(channel, m.cfg.Icons, m.state)
	text, run := "Reject the message request from "+name+"? It is deleted.", m.rejectRequest
	if accept {
		text, run = "Accept the message request from "+name+"? It moves to your DMs.", m.acceptRequest
	}
	return ui.ShowDialog(text, ui.DialogButton{Label: "No"}, ui.DialogButton{Label: "Yes", Cmd: run(channel.ID)})
}

func (m Model) acceptRequest(id discord.ChannelID) tui.Cmd {
	state := m.state
	return cmd.Task(func() tui.Msg {
		channel, err := state.AcceptMessageRequest(id)
		if err != nil {
			return ui.ErrorDialog("accept the message request", err)
		}
		// The gateway's Channel Update follows; storing the answer now lists the DM at once.
		if err := state.Cabinet.ChannelSet(channel, true); err != nil {
			slog.Error("failed to store the accepted DM", "channel_id", id, "err", err)
		}
		return requestsChangedMsg{}
	})
}

func (m Model) rejectRequest(id discord.ChannelID) tui.Cmd {
	state := m.state
	return cmd.Task(func() tui.Msg {
		if err := state.RejectMessageRequest(id); err != nil {
			return ui.ErrorDialog("reject the message request", err)
		}
		if channel, err := state.Cabinet.Channel(id); err == nil {
			if err := state.Cabinet.ChannelRemove(channel); err != nil {
				slog.Error("failed to drop the rejected DM", "channel_id", id, "err", err)
			}
		}
		return requestsChangedMsg{}
	})
}
