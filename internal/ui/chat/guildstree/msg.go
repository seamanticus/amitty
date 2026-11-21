package guildstree

import (
	"log/slog"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/ningen"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/tree"
	"github.com/seamanticus/amitty/internal/ui"
)

// currentMsg makes a node of the tree the current one.
type currentMsg struct {
	current *tree.Node
}

// selectedMsg is sent when the user selects a node.
type selectedMsg struct {
	node *tree.Node
}

type ChannelLoadedMsg struct {
	Channel  discord.Channel
	Messages []discord.Message
}

type NavigateMsg struct{ ChannelID discord.ChannelID }

// ShowGuildMsg highlights Guild, opening the folder it is in, and lists it first if it is not listed yet, as when the join answers before the gateway.
type ShowGuildMsg struct{ Guild discord.Guild }

// JoinMsg asks for an invite to join a server with.
type JoinMsg struct{}

func (m *Model) showGuild(guild discord.Guild) {
	m.addJoinedGuild(guild)
	node := m.guildNodes[guild.ID]
	m.expandPathToNode(node)
	m.setCurrentNode(node)
}

func (m *Model) navigate(channelID discord.ChannelID) tui.Cmd {
	channel, err := m.state.Cabinet.Channel(channelID)
	if err != nil {
		slog.Error("failed to get channel from state", "err", err, "channel_id", channelID)
		return nil
	}
	node := m.findNodeByChannelID(channelID)
	if node == nil {
		slog.Error("failed to locate channel in tree", "channel_id", channelID)
		return nil
	}
	m.expandPathToNode(node)
	m.setCurrentNode(node)
	if channel.Type == discord.GuildCategory {
		return nil
	}
	return m.selectNode(node)
}

func (m Model) selectNode(node *tree.Node) tui.Cmd {
	if expandable(node) {
		node.SetExpanded(!node.Expanded())
		return nil
	}

	switch ref := node.Reference().(type) {
	case discord.GuildID:
		m.expandGuild(node, ref)
	case discord.ChannelID:
		return m.selectChannel(node, ref)
	case dmNode:
		m.expandPrivateChannels(node)
	}

	return nil
}

func (m Model) expandGuild(node *tree.Node, guildID discord.GuildID) {
	go m.state.MemberState.Subscribe(guildID)
	channels, err := m.state.Cabinet.Channels(guildID)
	if err != nil {
		slog.Error("failed to get channels", "err", err, "guild_id", guildID)
		return
	}

	ui.SortGuildChannels(channels)
	m.createChannelNodes(node, channels)
	node.Expand()
}

// selectChannel loads the messages of the channel, or shows the threads of a forum.
func (m Model) selectChannel(node *tree.Node, channelID discord.ChannelID) tui.Cmd {
	channel, err := m.state.Cabinet.Channel(channelID)
	if err != nil {
		slog.Error("failed to get channel from state", "err", err, "channel_id", channelID)
		return nil
	}

	if channel.Type != discord.GuildForum {
		return loadChannel(m.state, openLimit, *channel)
	}

	m.expandForum(node, *channel)
	return nil
}

func (m Model) expandForum(node *tree.Node, forum discord.Channel) {
	channels, err := m.state.Cabinet.Channels(forum.GuildID)
	if err != nil {
		slog.Error("failed to get channels for forum threads", "err", err, "guild_id", forum.GuildID)
		return
	}

	for _, child := range channels {
		if child.ParentID == forum.ID && child.Type.IsThread() {
			m.createChannelNode(node, child)
		}
	}

	node.Expand()
}

func (m Model) expandPrivateChannels(node *tree.Node) {
	channels, err := ui.VisiblePrivateChannels(m.state, m.cfg.HideDeletedUsers)
	if err != nil {
		slog.Error("failed to get private channels", "err", err)
		return
	}

	for _, channel := range channels {
		m.createChannelNode(node, channel)
	}

	node.Expand()
}

// openLimit is the messages fetched when a channel is opened, as many as the web client asks for; older ones page by messages_limit.
const openLimit = 10

func loadChannel(state *ningen.State, limit uint, channel discord.Channel) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		messages, err := state.Messages(channel.ID, limit)
		if err != nil {
			slog.Error("failed to get messages", "err", err, "channel_id", channel.ID, "limit", limit)
			return nil
		}

		if lastMessageID := state.LastMessage(channel.ID); lastMessageID.IsValid() {
			go state.ReadState.MarkRead(channel.ID, lastMessageID)
		}

		return ChannelLoadedMsg{Channel: channel, Messages: messages}
	})
}

// RefreshPrivateChannels lists the DMs again under an expanded Direct Messages node, as which are shown may have changed.
func (m *Model) RefreshPrivateChannels() {
	node := m.dmRootNode
	if node == nil {
		return
	}
	setNodeText(node, m.dmNodeText())
	if len(node.Children()) == 0 {
		return
	}
	for _, child := range node.Children() {
		if id, ok := child.Reference().(discord.ChannelID); ok {
			delete(m.channelNodes, id)
		}
		if child == m.current {
			m.setCurrentNode(node)
		}
	}
	node.ClearChildren()
	m.expandPrivateChannels(node)
}
