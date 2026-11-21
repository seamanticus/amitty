package guildstree

import (
	"strconv"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/ui"
)

// markTarget returns the guilds marking node read covers, and their name for the question: a guild, the guild of a channel in one, or every guild of a folder.
func (m Model) markTarget(node *tree.Node) ([]discord.GuildID, string, bool) {
	if node == nil || node == m.root {
		return nil, "", false
	}
	switch ref := node.Reference().(type) {
	case discord.GuildID:
		return []discord.GuildID{ref}, m.guildName(ref), true
	case discord.ChannelID:
		channel, err := m.state.Cabinet.Channel(ref)
		if err != nil || !channel.GuildID.IsValid() {
			return nil, "", false
		}
		return []discord.GuildID{channel.GuildID}, m.guildName(channel.GuildID), true
	case nil:
		var guilds []discord.GuildID
		for _, child := range node.Children() {
			if id, ok := child.Reference().(discord.GuildID); ok {
				guilds = append(guilds, id)
			}
		}
		if len(guilds) == 0 {
			return nil, "", false
		}
		return guilds, "the folder", true
	}
	return nil, "", false
}

func (m Model) guildName(id discord.GuildID) string {
	if guild, err := m.state.Cabinet.Guild(id); err == nil {
		return guild.Name
	}
	return "the server"
}

// unreadIn returns each unread channel of guilds the user can see, with the message to mark it read up to.
func (m Model) unreadIn(guilds []discord.GuildID) map[discord.ChannelID]discord.MessageID {
	reads := make(map[discord.ChannelID]discord.MessageID)
	for _, guildID := range guilds {
		channels, err := m.state.Cabinet.Channels(guildID)
		if err != nil {
			continue
		}
		for _, channel := range channels {
			if m.state.ChannelIsUnread(channel.ID, ningen.UnreadOpts{IncludeMutedCategories: true}) == ningen.ChannelRead {
				continue
			}
			if !m.state.HasPermissions(channel.ID, discord.PermissionViewChannel) {
				continue
			}
			if last := m.state.LastMessage(channel.ID); last.IsValid() {
				reads[channel.ID] = last
			}
		}
	}
	return reads
}

// confirmMarkRead asks whether to mark the unread channels under node read, defaulting to No; nothing is asked when none are unread.
func (m Model) confirmMarkRead(node *tree.Node) tui.Cmd {
	guilds, name, ok := m.markTarget(node)
	if !ok {
		return nil
	}
	reads := m.unreadIn(guilds)
	if len(reads) == 0 {
		return nil
	}
	channels := "1 channel"
	if len(reads) > 1 {
		channels = strconv.Itoa(len(reads)) + " channels"
	}
	state := m.state
	mark := cmd.Task(func() tui.Msg {
		if err := state.ReadState.MarkReadBulk(reads); err != nil {
			return ui.ErrorDialog("mark as read", err)
		}
		return nil
	})
	return ui.ShowDialog("Mark "+channels+" in "+name+" as read?", ui.DialogButton{Label: "No"}, ui.DialogButton{Label: "Yes", Cmd: mark})
}
