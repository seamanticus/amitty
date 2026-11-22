package login

import (
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/tabs"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/login/password"
	"github.com/seamanticus/amitty/internal/ui/login/qr"
	"github.com/seamanticus/amitty/internal/ui/login/token"
)

// tab is a login method shown as a tab.
type tab int

const (
	tabPassword tab = iota
	tabQR
	tabToken
	tabCount
)

// Model shows the login methods as tabs inside a bordered box.
type Model struct {
	cfg      *config.Config
	password password.Model
	qr       qr.Model
	token    token.Model
	active   tab
}

// NewModel returns the login methods, whose requests identify as build and go through browser.
func NewModel(cfg *config.Config, build http.Build, browser *http.Browser) Model {
	return Model{cfg: cfg, password: password.NewModel(build, browser), qr: qr.NewModel(build, browser)}
}

// tabs returns the login methods as tabs set up by the tabs config.
func (m Model) tabs() tabs.Widget {
	cfg := m.cfg.UI.Tabs
	return tabs.New(m.password.Label(), m.qr.Label(), m.token.Label()).
		Width(layout.Fill).
		Alignment(cfg.Alignment.HorizontalAlignment).
		Wrap(cfg.Wrap).
		Separator(cfg.Separator).
		Padding(cfg.Padding[0], cfg.Padding[1]).
		Arrows(cfg.Arrows[0], cfg.Arrows[1]).
		ClickableArrows(cfg.ClickableArrows)
}

var _ tui.Model[Model] = Model{}

// Init initializes the active tab.
func (m Model) Init() tui.Cmd {
	switch m.active {
	case tabPassword:
		return m.password.Init()
	case tabQR:
		return m.qr.Init()
	case tabToken:
		return m.token.Init()
	}
	return nil
}

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	var errCmd tui.Cmd
	switch msg := msg.(type) {
	case selectTabMsg:
		if msg < 0 || tab(msg) >= tabCount {
			return m, nil
		}
		var leaving tui.Cmd
		if m.active == tabQR && tab(msg) != tabQR {
			// Its heartbeats reach only the shown tab, so a hidden connection would be closed by the gateway.
			m.qr, leaving = m.qr.Leave()
		}
		m.active = tab(msg)
		if leaving == nil {
			return m, m.Init()
		}
		return m, cmd.Batch{leaving, m.Init()}
	case password.FailedMsg:
		// The password tab gets it even when inactive, so its submit unlocks.
		var command tui.Cmd
		m.password, command = m.password.Update(msg)
		return m, cmd.Batch{showErrorDialog(msg.Err), command}
	case password.CountdownMsg, password.FingerprintMsg, password.MFAMsg, password.SMSSentMsg,
		password.ChallengedMsg, password.CaptchaMsg, password.CaptchaDoneMsg:
		// Like a failure, these reach the password tab even when another tab is shown.
		var command tui.Cmd
		m.password, command = m.password.Update(msg)
		return m, command
	case error:
		if m.active != tabQR {
			// Only the QR tab fails with a bare error, and leaving it ended its login: this one is late.
			return m, nil
		}
		// The QR tab still gets the error, so it can close its connection.
		errCmd = showErrorDialog(msg)
	}

	var command tui.Cmd
	switch m.active {
	case tabPassword:
		m.password, command = m.password.Update(msg)
	case tabQR:
		m.qr, command = m.qr.Update(msg)
	case tabToken:
		m.token, command = m.token.Update(msg)
	}
	var settling tui.Cmd
	if m.active != tabQR {
		m.qr, settling = m.qr.Settle(msg)
	}
	return m, cmd.Batch{errCmd, command, settling}
}

// View shows the logo above a box of the method tabs and a box of the active method's form.
func (m Model) View() tui.Widget {
	var content tui.Widget
	var title string
	switch m.active {
	case tabPassword:
		content, title = m.password.View(), m.password.Title()
	case tabQR:
		content, title = m.qr.View(), m.qr.Label()
	case tabToken:
		content, title = m.token.View(), m.token.Label()
	}
	t := m.tabs().
		Active(int(m.active)).
		Keybind(tabsKeybind).
		OnSelect(func(i int) tui.Msg { return selectTabMsg(i) })
	theme := m.cfg.Theme
	methods := ui.Box(t, m.cfg, false).Title(ui.Label(methodsTitle, theme.Title, false)).Height(layout.Shrink)
	formBox := ui.Box(content, m.cfg, true).Title(ui.Label(title, theme.Title, true))
	return page{
		logo:    logoView(theme.Title.NormalStyle.Style, theme.Border.ActiveStyle.Style),
		methods: methods,
		form:    formBox,
	}
}

// methodsTitle captions the box of the method tabs.
const methodsTitle = "Authentication Method"
