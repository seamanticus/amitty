package chat

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil/httpdriver"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/logger"
	"github.com/seamanticus/amitty/internal/ningen/states/read"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
)

func onRequest(req httpdriver.Request) error {
	if req, ok := req.(*httpdriver.DefaultRequest); ok {
		slog.Debug("new HTTP request", "method", req.Method, "url", req.URL)
	}

	return nil
}

func (m *Model) loadGuildsTree(event *gateway.ReadyEvent) tui.Cmd {
	cmd := m.updatePane(paneGuildsTree, event)
	m.setFocus(paneGuildsTree)
	return cmd
}

func (m *Model) addMessageOrNotify(message *gateway.MessageCreateEvent) tui.Cmd {
	command := m.updatePane(paneGuildsTree, message)

	if channel := m.selectedChannel; channel != nil && channel.ID == message.ChannelID {
		delete(m.typers, message.Author.ID)
		command = cmd.Batch{command, m.messagesList.AddMessage(message.Message)}
		if !m.windowUnfocused || !m.cfg.Notifications.WhenUnfocused {
			return command
		}
	}
	return cmd.Batch{command, m.notify(*message)}
}

func (m Model) notify(message gateway.MessageCreateEvent) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		return m.notification(message)
	})
}

// notification shows a desktop notification for message if it mentions the user, and plays the sound with it.
func (m Model) notification(message gateway.MessageCreateEvent) tui.Msg {
	notifications := m.cfg.Notifications
	popup := notifications.Popup
	chime := notifications.Sound.Enabled
	if !popup && !chime || m.cfg.Status == discord.DoNotDisturbStatus {
		return nil
	}

	if m.state.MessageMentions(&message.Message) == 0 {
		return nil
	}

	content := message.Content
	if content == "" && len(message.Attachments) > 0 {
		content = "Uploaded " + message.Attachments[0].Filename
	}

	if content == "" {
		return nil
	}

	title, err := m.notificationTitle(message)
	if err != nil {
		slog.Error("failed to get notification title", "err", err)
		return nil
	}

	var notify cmd.Batch
	if popup {
		notify = append(notify, cmd.Notify{Title: title, Body: content})
	}
	if chime {
		notify = append(notify, ui.PlaySound(notifications.Sound))
	}
	return notify
}

// notificationTitle returns the author of message, followed by its channel and guild for a guild message.
func (m Model) notificationTitle(message gateway.MessageCreateEvent) (string, error) {
	title := message.Author.DisplayOrUsername()
	channel, err := m.state.Cabinet.Channel(message.ChannelID)
	if err != nil {
		return "", fmt.Errorf("failed to get channel %s from state: %w", message.ChannelID, err)
	}

	if !channel.GuildID.IsValid() {
		return title, nil
	}

	guild, err := m.state.Cabinet.Guild(channel.GuildID)
	if err != nil {
		return "", fmt.Errorf("failed to get guild %s from state: %w", channel.GuildID, err)
	}

	if member := message.Member; member != nil && member.Nick != "" {
		title = member.Nick
	}

	return title + " (#" + channel.Name + ", " + guild.Name + ")", nil
}

func (m Model) refreshMemberNames(event *gateway.GuildMembersChunkEvent) tui.Cmd {
	m.messagesList.InvalidateRendered()
	return m.composer.CacheMemberSearch(event)
}

func (m Model) showTypingIndicator(event *gateway.TypingStartEvent) tui.Cmd {
	if channel := m.selectedChannel; channel == nil || channel.ID != event.ChannelID || ui.IsMe(m.state, event.UserID) {
		return nil
	}
	return m.addTyper(event.UserID)
}

// applyEvent updates the model with a gateway event and returns what the event needs done.
func (m *Model) applyEvent(event gateway.Event) tui.Cmd {
	switch event := event.(type) {
	case *ws.RawEvent:
		slog.Debug("new raw event", "code", event.OriginalCode, "type", event.OriginalType)
		logger.Raw(context.Background(), "gateway event", event.Raw, "code", event.OriginalCode, "type", event.OriginalType)

	case *gateway.ReadyEvent:
		return cmd.Batch{m.loadGuildsTree(event), m.updatePane(paneMembersTree, event), m.refreshToken(event.AuthToken)}

	case *gateway.MessageCreateEvent:
		return m.addMessageOrNotify(event)
	case *gateway.MessageUpdateEvent:
		return m.messagesList.UpdateMessage(event.Message)
	case *gateway.PresenceUpdateEvent:
		return cmd.Batch{m.updatePane(paneGuildsTree, event), m.updatePane(paneMembersTree, event)}
	case *gateway.GuildMemberListUpdateEvent:
		return m.updatePane(paneMembersTree, event)
	case *gateway.MessageDeleteEvent:
		m.messagesList.DeleteMessage(event.ChannelID, event.ID)
	case *gateway.MessageReactionAddEvent:
		return m.messagesList.RefreshMessage(event.ChannelID, event.MessageID)
	case *gateway.MessageReactionAddManyEvent:
		return m.messagesList.RefreshMessage(event.ChannelID, event.MessageID)
	case *gateway.MessageReactionRemoveEvent:
		return m.messagesList.RefreshMessage(event.ChannelID, event.MessageID)
	case *gateway.MessageReactionRemoveAllEvent:
		return m.messagesList.RefreshMessage(event.ChannelID, event.MessageID)
	case *gateway.MessageReactionRemoveEmojiEvent:
		return m.messagesList.RefreshMessage(event.ChannelID, event.MessageID)

	case *gateway.GuildMembersChunkEvent:
		return m.refreshMemberNames(event)
	case *gateway.GuildMemberRemoveEvent:
		m.composer.OnGuildMemberRemove(event)

	case *gateway.TypingStartEvent:
		if m.cfg.TypingIndicator.Receive {
			return m.showTypingIndicator(event)
		}

	case *read.UpdateEvent:
		return m.updatePane(paneGuildsTree, event)
	}
	return nil
}
