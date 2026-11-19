package root

import (
	"log/slog"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/seamanticus/discordo/internal/consts"
	"github.com/seamanticus/discordo/internal/ui/chat"
	"github.com/seamanticus/discordo/internal/ui/login"
	"golang.design/x/clipboard"
)

// quit quits once the chat, if shown, has closed its session.
func (m *Model) quit() tview.Cmd {
	if m.screen.isChat() {
		return m.updateScreen(chat.QuitMsg{})
	}
	return cmd.Quit{}
}

func initClipboard() tview.Cmd {
	return cmd.Task(func() tview.Msg {
		if err := clipboard.Init(); err != nil {
			slog.Error("failed to init clipboard", "err", err)
		}
		return nil
	})
}

func (m *Model) showLogin() tview.Cmd {
	l := login.NewModel(m.cfg, m.build)
	m.screen = loginScreen(l)
	return m.show(l.Init())
}

func (m *Model) showChat(token string) tview.Cmd {
	c := chat.NewModel(m.cfg, m.build, token)
	m.screen = chatScreen(c)
	init := c.Init()
	if m.resize != nil {
		init = cmd.Batch{init, m.updateScreen(m.resize)}
	}
	return m.show(init)
}

// show closes any open dialog after the shown model changed and returns what it needs done, with init its first command.
func (m *Model) show(init tview.Cmd) tview.Cmd {
	m.dialog = nil
	return cmd.Batch{cmd.SetTitle(consts.Name), init}
}

func (m *Model) finishDialog(index int) tview.Cmd {
	if m.dialog == nil || index < 0 || index >= len(m.dialog.Buttons) {
		m.dialog = nil
		return nil
	}
	button := m.dialog.Buttons[index]
	if !button.KeepOpen {
		m.dialog = nil
	}
	return button.Cmd
}
