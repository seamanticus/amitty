// Package joinprompt asks for a server invite to join, and shows how the join goes until it ends.
package joinprompt

import (
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
	"github.com/seamanticus/amitty/internal/ui"
)

// SubmittedMsg reports the invite typed, a link or a code.
type SubmittedMsg struct{ Input string }

// CancelMsg reports that the prompt was dismissed.
type CancelMsg struct{}

type inputMsg struct {
	text   string
	cursor int
}

type submitMsg struct{}

// Model is a one-line invite input over a status line, which the join it starts fills in.
type Model struct {
	cfg  *config.Config
	text string
	// cursor is the byte offset of the cursor in text.
	cursor int
	status string
}

// New returns an empty prompt.
func New(cfg *config.Config) Model {
	return Model{cfg: cfg}
}

// SetStatus shows status below the input; empty shows none.
func (m *Model) SetStatus(status string) { m.status = status }

var _ tui.Model[Model] = Model{}

func (Model) Init() tui.Cmd { return nil }

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case inputMsg:
		m.text, m.cursor = msg.text, msg.cursor
		return m, nil
	case submitMsg:
		input := m.text
		return m, cmd.Task(func() tui.Msg { return SubmittedMsg{input} })
	case tui.KeyMsg:
		if keybind.Matches(msg, m.cfg.Keybinds.JoinPrompt.Cancel.Keybind) {
			return m, cmd.Task(func() tui.Msg { return CancelMsg{} })
		}
	}
	return m, nil
}

// Sizes of the prompt: the input's width, which scrolls a longer invite, and the blank cells around the content.
const (
	inputWidth  = 35
	paddingTop  = 2
	paddingSide = 5
)

// View shows the input in a box titled Join a server, over the status, as large as they are with padding around them.
func (m Model) View() tui.Widget {
	cfg := m.cfg
	input := textinput.New(m.text, m.cursor).
		Focused(true).
		Style(tcell.StyleDefault.Underline(true)).
		Width(layout.Fixed(inputWidth)).
		Keybind(inputKeybind(cfg.Keybinds.JoinPrompt.Submit.Keybind)).
		OnInput(func(text string, cursor int) tui.Msg { return inputMsg{text, cursor} }).
		OnSubmit(submitMsg{})
	status := "Paste an invite link or code"
	style := tcell.StyleDefault.Dim(true)
	if m.status != "" {
		status, style = m.status, tcell.StyleDefault.Foreground(color.Yellow)
	}
	content := column.New(input, text.New(status).Style(style)).Spacing(1).Width(layout.Shrink).Height(layout.Shrink)
	bottom := cfg.UI.Border.Padding[1]
	return ui.Box(content, cfg, true).
		Title(ui.Label("Join a server", cfg.Theme.Title, true)).
		Padding(paddingTop, bottom, paddingSide, paddingSide).
		Width(layout.Shrink).
		Height(layout.Shrink)
}

// inputKeybind is the input's default keybind with submit as the only key that submits.
func inputKeybind(submit keybind.Keybind) func(tui.KeyMsg) textinput.Action {
	return func(key tui.KeyMsg) textinput.Action {
		if keybind.Matches(key, submit) {
			return textinput.ActionSubmit
		}
		if action := textinput.DefaultKeybind(key); action != textinput.ActionSubmit {
			return action
		}
		return textinput.ActionNone
	}
}
