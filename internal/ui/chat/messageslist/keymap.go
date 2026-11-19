package messageslist

import (
	"github.com/ayn2op/tview/help"
	"github.com/ayn2op/tview/keybind"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/dmwire/discord"
)

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	keybinds := m.cfg.Keybinds.MessagesList
	help := []keybind.Keybind{
		keybinds.SelectUp.Keybind,
		keybinds.SelectDown.Keybind,
		keybinds.Cancel.Keybind,
	}

	if selectedMessage, ok := m.selectedMessage(); ok {
		if !ui.IsMe(m.state, selectedMessage.Author.ID) {
			help = append(help, keybinds.Reply.Keybind)
		}
		if len(selectedMessage.Attachments) != 0 || len(m.selectedURLs()) != 0 {
			help = append(help, keybinds.Open.Keybind)
		}
	}

	return help
}

func (m Model) FullHelp() [][]keybind.Keybind {
	keybinds := m.cfg.Keybinds.MessagesList
	var actions, manage, attachments []keybind.Keybind
	if message, ok := m.selectedMessage(); ok {
		actions, manage, attachments = m.messageHelp(*message)
	}

	actions = append(actions, keybinds.Cancel.Keybind)

	return [][]keybind.Keybind{
		{keybinds.SelectUp.Keybind, keybinds.SelectDown.Keybind, keybinds.SelectTop.Keybind, keybinds.SelectBottom.Keybind},
		{keybinds.ScrollUp.Keybind, keybinds.ScrollDown.Keybind, keybinds.ScrollTop.Keybind, keybinds.ScrollBottom.Keybind},
		actions,
		manage,
		attachments,
		{keybinds.YankContent.Keybind, keybinds.YankURL.Keybind, keybinds.YankID.Keybind},
	}
}

// messageHelp returns the keybinds that act on message: replying to it, managing it and opening what it has.
func (m Model) messageHelp(message discord.Message) (actions, manage, attachments []keybind.Keybind) {
	keybinds := m.cfg.Keybinds.MessagesList
	mine := ui.IsMe(m.state, message.Author.ID)
	if !mine {
		actions = append(actions, keybinds.Reply.Keybind, keybinds.ReplyMention.Keybind)
	}

	if message.ReferencedMessage != nil {
		actions = append(actions, keybinds.SelectReply.Keybind)
	}
	if m.canReact() {
		actions = append(actions, keybinds.React.Keybind)
	}

	if mine {
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
