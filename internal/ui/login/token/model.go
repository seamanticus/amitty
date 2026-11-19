package token

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
	"github.com/seamanticus/amitty/internal/ui/login/keys"
)

// Model is a token input above a button. Tab and the arrow keys move the focus, Enter in the input moves to the button, and Enter on the button or clicking it logs in.
type Model struct {
	token string
	// cursor is the byte offset of the cursor in token.
	cursor int

	buttonFocused bool
	// buttonHovered reports whether the pointer is over the button.
	buttonHovered bool
}

func (Model) Label() string {
	return "Token"
}

var _ tui.Model[Model] = Model{}

var _ help.KeyMap = Model{}

func (Model) ShortHelp() []keybind.Keybind {
	return []keybind.Keybind{keys.FocusNext(), keys.FocusPrevious()}
}

func (m Model) FullHelp() [][]keybind.Keybind { return [][]keybind.Keybind{m.ShortHelp()} }

func (Model) Init() tui.Cmd { return nil }

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case tokenMsg:
		m.token, m.cursor = msg.text, msg.cursor
	case focusButtonMsg:
		m.buttonFocused = true
	case hoverMsg:
		m.buttonHovered = bool(msg)
	case tui.KeyMsg:
		if keybind.Matches(msg, keys.FocusNext(), keys.FocusPrevious()) {
			m.buttonFocused = !m.buttonFocused
		}
	case submitMsg:
		if m.token == "" {
			return m, nil
		}
		return m, submitToken(m.token)
	}
	return m, nil
}

func (m Model) View() tui.Widget {
	input := textinput.New(m.token, m.cursor).
		Mask("*").
		Focused(!m.buttonFocused).
		OnInput(func(text string, cursor int) tui.Msg { return tokenMsg{text, cursor} }).
		OnSubmit(focusButtonMsg{})
	buttonStyle := tcell.StyleDefault
	switch {
	case m.buttonFocused:
		buttonStyle = buttonStyle.Reverse(true)
	case m.buttonHovered:
		buttonStyle = buttonStyle.Underline(true)
	}
	submit := button.New(text.New("Login")).
		Style(buttonStyle).
		Width(layout.Fixed(len("Login") + 4)).
		Focused(m.buttonFocused).
		Hovered(m.buttonHovered).
		OnHover(func(hovered bool) tui.Msg { return hoverMsg(hovered) }).
		OnClick(submitMsg{})
	return column.New(
		row.New(text.New("Token").Width(layout.Fixed(len("Token")+1)), input).Height(layout.Shrink),
		submit,
	).Spacing(1).AlignX(tui.HorizontalAlignmentCenter)
}
