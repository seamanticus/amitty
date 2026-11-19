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
		return loadChannel(m.state, uint(m.cfg.MessagesLimit), *channel)
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
	channels, err := m.state.PrivateChannels()
	if err != nil {
		slog.Error("failed to get private channels", "err", err)
		return
	}

	ui.SortPrivateChannels(channels)
	for _, channel := range channels {
		m.createChannelNode(node, channel)
	}

	node.Expand()
}

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
