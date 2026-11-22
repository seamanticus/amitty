package root

import (
	"github.com/seamanticus/amitty/internal/tui/layout"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tokenstore"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/backdrop"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/center"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/dialog"
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/inert"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/stack"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/chat"
	"github.com/seamanticus/amitty/internal/ui/login"
	"github.com/seamanticus/amitty/internal/ui/login/password"
	"github.com/seamanticus/amitty/internal/ui/login/qr"
	"github.com/seamanticus/amitty/internal/ui/login/token"
	"github.com/seamanticus/amitty/internal/ui/settings"
)

type Model struct {
	screen screen
	// helpVisible reports whether help is shown below the inner model, and helpShowAll whether it shows every keybind.
	helpVisible, helpShowAll bool
	// dialog is the dialog shown, or nil when none is open, and dialogFocus its focused button.
	dialog      *ui.DialogMsg
	dialogFocus int
	// resize is the last size of the window, or nil before the first, kept for the models shown later.
	resize tui.ResizeMsg

	cfg *config.Config
	// configPath is the file the settings panel edits.
	configPath string
	// settings is the open settings panel, or nil.
	settings *settings.Model

	// build is what every request and the gateway identify as, fetched once at startup.
	build http.Build
	// browser holds the cookies and installation every request shares, and how they connect.
	browser *http.Browser
	// store keeps the token between runs.
	store tokenstore.Store
}

// NewModel returns the application, opening the chat with token if set and the login otherwise. configPath is the file cfg was read from.
func NewModel(cfg *config.Config, configPath string, build http.Build, browser *http.Browser, store tokenstore.Store, token string) Model {
	m := Model{cfg: cfg, configPath: configPath, build: build, browser: browser, store: store, helpVisible: cfg.Help.Enabled}
	if token == "" {
		m.screen = loginScreen(login.NewModel(cfg, build, browser))
	} else {
		m.screen = chatScreen(chat.NewModel(cfg, build, browser, token))
	}
	return m
}

// step is a screen changed by a message and the command it returned.
type step struct {
	screen screen
	cmd    tui.Cmd
}

// updateScreen updates the model shown above help with msg.
func (m *Model) updateScreen(msg tui.Msg) tui.Cmd {
	next := m.screen.Match(
		func(l login.Model) step {
			l, c := l.Update(msg)
			return step{loginScreen(l), c}
		},
		func(c chat.Model) step {
			c, command := c.Update(msg)
			return step{chatScreen(c), command}
		},
	)
	m.screen = next.screen
	return next.cmd
}

// helpView returns the help for the keybinds that currently apply.
func (m Model) helpView() help.Widget {
	cfg := m.cfg
	return help.New(m).
		ShortKeyStyle(cfg.Theme.Help.ShortKeyStyle.Style).
		ShortDescStyle(cfg.Theme.Help.ShortDescStyle.Style).
		FullKeyStyle(cfg.Theme.Help.FullKeyStyle.Style).
		FullDescStyle(cfg.Theme.Help.FullDescStyle.Style).
		CompactModifiers(cfg.Help.CompactModifiers).
		ShortSeparator(cfg.Help.Separator).
		ShowAll(m.helpShowAll)
}

type (
	dialogFocusMsg int
	dialogDoneMsg  int
)

// dialogView returns the view of the open dialog.
func (m Model) dialogView() tui.Widget {
	labels := make([]string, len(m.dialog.Buttons))
	for i, button := range m.dialog.Buttons {
		labels[i] = button.Label
	}
	style := m.cfg.Theme.Dialog.Style.Style
	widget := dialog.New().
		Text(m.dialog.Text).
		Buttons(labels...).
		Focus(m.dialogFocus).
		ButtonStyle(style).
		ActivatedStyle(style.Reverse(true)).
		OnFocus(func(i int) tui.Msg { return dialogFocusMsg(i) }).
		OnDone(func(i int, _ string) tui.Msg { return dialogDoneMsg(i) })
	if bg := style.GetBackground(); bg != tcell.ColorDefault {
		widget = widget.Background(bg)
	}

	if fg := style.GetForeground(); fg != tcell.ColorDefault {
		widget = widget.TextColor(fg)
	}

	return widget
}

func (m *Model) toggleHelp() {
	if m.cfg.Help.Enabled {
		m.helpVisible = !m.helpVisible
	}
}

var _ tui.Model[Model] = Model{}

func (m Model) Init() tui.Cmd {
	return cmd.Batch{
		cmd.SetTitle(consts.Name),
		cmd.SetBase{Style: m.cfg.BaseStyle()},
		initClipboard(),
		m.screen.Match(login.Model.Init, chat.Model.Init),
	}
}

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case storedMsg:
		command := m.showChat(msg.token)
		// Shown after the chat, as showing it closes any dialog.
		if msg.err != nil {
			dialog := ui.ErrorDialog("store the token", msg.err)
			m.dialog, m.dialogFocus = &dialog, 0
		}
		return m, command

	case password.TokenMsg:
		return m, storeToken(m.store, string(msg))
	case qr.TokenMsg:
		return m, storeToken(m.store, string(msg))
	case token.TokenMsg:
		return m, storeToken(m.store, string(msg))
	case chat.TokenRefreshedMsg:
		return m, saveRefreshed(m.store, string(msg))

	case chat.StateClosedMsg:
		return m, cmd.Quit{}
	case chat.LogoutMsg:
		// Both a logout and a rejected token (4004) end the token for good.
		command := cmd.Batch{m.showLogin(), deleteToken(m.store)}
		// Shown after the login, as showing it closes any dialog.
		if msg.Err != nil {
			dialog := ui.ErrorDialog(msg.What, msg.Err)
			m.dialog, m.dialogFocus = &dialog, 0
		}
		return m, command
	case ui.DialogMsg:
		if m.dialog == nil {
			m.dialog, m.dialogFocus = &msg, 0
		}
		return m, nil
	case dialogFocusMsg:
		m.dialogFocus = int(msg)
		return m, nil
	case dialogDoneMsg:
		return m, m.finishDialog(int(msg))

	case settings.Msg:
		if m.settings == nil {
			return m, nil
		}
		next, command := m.settings.Update(msg)
		m.settings = &next
		return m, command
	case settings.ClosedMsg:
		m.settings = nil
		return m, nil
	case settings.SavedMsg:
		return m, m.applyConfig(msg.Config)
	case settings.PreviewMsg:
		return m, m.applyConfig(msg.Config)

	case tui.KeyMsg:
		// The dialog takes all input while it is open, so nothing behind it reacts.
		if m.dialog != nil {
			return m, nil
		}

		keybinds := m.cfg.Keybinds
		// The panel takes all input while it is open, but for the key that closes it.
		if m.settings != nil {
			if keybind.Matches(msg, keybinds.Settings.Keybind) {
				m.settings = nil
			}
			return m, nil
		}
		switch {
		case keybind.Matches(msg, keybinds.Settings.Keybind):
			panel := settings.New(m.cfg, m.configPath)
			m.settings = &panel
			return m, nil
		case keybind.Matches(msg, keybinds.ToggleHelp.Keybind):
			m.toggleHelp()
			return m, nil
		case keybind.Matches(msg, keybinds.ToggleFullHelp.Keybind):
			if m.cfg.Help.Enabled {
				m.helpShowAll = !m.helpShowAll
			}
			return m, nil
		case keybind.Matches(msg, keybinds.Suspend.Keybind):
			return m, suspend()
		case keybind.Matches(msg, keybinds.Quit.Keybind):
			return m, ui.ShowDialog(
				"Are you sure you want to quit?",
				ui.DialogButton{Label: "No"},
				ui.DialogButton{Label: "Yes", Cmd: m.quit()},
			)
		}
	case tui.PasteMsg:
		if m.dialog != nil || m.settings != nil {
			return m, nil
		}
	case tui.ResizeMsg:
		m.resize = msg
	}

	return m, m.updateScreen(msg)
}

// applyConfig replaces the shared config with next. Every model reads it as it draws and updates, so that applies it; options used once at startup keep their values.
func (m *Model) applyConfig(next *config.Config) tui.Cmd {
	previous := *m.cfg
	*m.cfg = config.Live(previous, *next)
	m.helpVisible = m.helpVisible && m.cfg.Help.Enabled
	return cmd.Batch{cmd.SetBase{Style: m.cfg.BaseStyle()}, m.updateScreen(chat.ConfigChangedMsg{Previous: previous})}
}

// View shows the inner model above help, with the dialog on top of both when one is open.
func (m Model) View() tui.Widget {
	var helpView tui.Widget
	innerView := m.screen.Match(
		func(l login.Model) tui.Widget { return l.View() },
		func(c chat.Model) tui.Widget { return c.View() },
	)
	if m.helpVisible {
		helpView = box.New(m.helpView()).Padding(0, 0, m.cfg.Help.Padding[0], m.cfg.Help.Padding[1]).Height(layout.Shrink)
	}
	var content tui.Widget = column.New(innerView, helpView)
	if m.settings != nil {
		content = stack.New(
			inert.New(content),
			backdrop.New().Style(m.cfg.Theme.Dialog.BackgroundStyle.Style),
			center.New(column.New(m.settings.View()).Width(layout.Fixed(m.cfg.Picker.Width)).Height(layout.Fixed(m.cfg.Picker.Height))),
		)
	}
	if m.dialog == nil {
		return content
	}
	return stack.New(inert.New(content), backdrop.New().Style(m.cfg.Theme.Dialog.BackgroundStyle.Style), m.dialogView())
}
