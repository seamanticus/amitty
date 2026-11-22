package messageslist

import (
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/keybind"
)

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	keybinds := m.cfg.Keybinds.MessagesList
	help := []keybind.Keybind{
		keybinds.SelectUp.Keybind,
		keybinds.SelectDown.Keybind,
	}

	if selectedMessage, ok := m.selectedMessage(); ok {
		help = append(help, keybinds.Cancel.Keybind, keybinds.Reply.Keybind)
		if m.channel != nil && m.isEditable(messageItem{message: *selectedMessage}, m.channel.ID) {
			help = append(help, keybinds.Edit.Keybind)
		}
		if m.canDeleteMessage(*selectedMessage) {
			help = append(help, keybinds.DeleteConfirm.Keybind)
		}
		if len(selectedMessage.Attachments) != 0 || len(m.selectedURLs()) != 0 {
			help = append(help, keybinds.Open.Keybind)
		}
	}

	return help
}

func (m Model) FullHelp() [][]keybind.Keybind {
	keybinds := m.cfg.Keybinds.MessagesList
	full := [][]keybind.Keybind{
		{keybinds.SelectUp.Keybind, keybinds.SelectDown.Keybind, keybinds.SelectTop.Keybind, keybinds.SelectBottom.Keybind},
		{keybinds.ScrollUp.Keybind, keybinds.ScrollDown.Keybind, keybinds.ScrollTop.Keybind, keybinds.ScrollBottom.Keybind},
	}
	// The other keys act on the selected message, so they do nothing without one.
	message, ok := m.selectedMessage()
	if !ok {
		return full
	}
	actions, manage, attachments := m.messageHelp(*message)
	actions = append(actions, keybinds.Cancel.Keybind)
	return append(full, actions, manage, attachments, []keybind.Keybind{keybinds.YankContent.Keybind, keybinds.YankURL.Keybind, keybinds.YankID.Keybind})
}

// messageHelp returns the keybinds that act on message: replying to it, managing it and opening what it has.
func (m Model) messageHelp(message discord.Message) (actions, manage, attachments []keybind.Keybind) {
	keybinds := m.cfg.Keybinds.MessagesList
	actions = append(actions, keybinds.Reply.Keybind, keybinds.ReplyMention.Keybind)

	if message.ReferencedMessage != nil {
		actions = append(actions, keybinds.SelectReply.Keybind)
	}
	if m.canReact() {
		actions = append(actions, keybinds.React.Keybind)
	}
	actions = append(actions, keybinds.ViewProfile.Keybind)

	if m.channel != nil && m.isEditable(messageItem{message: message}, m.channel.ID) {
		manage = append(manage, keybinds.Edit.Keybind)
	}

	if m.canDeleteMessage(message) {
		manage = append(manage, keybinds.DeleteConfirm.Keybind)
		if len(keybinds.Delete.Keys()) != 0 {
			manage = append(manage, keybinds.Delete.Keybind)
		}
	}

	if len(message.Attachments) != 0 || len(m.selectedURLs()) != 0 {
		attachments = append(attachments, keybinds.Open.Keybind, keybinds.OpenInBrowser.Keybind)
	}

	if len(message.Attachments) != 0 {
		attachments = append(attachments, keybinds.Download.Keybind)
	}

	return actions, manage, attachments
}
