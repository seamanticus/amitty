package tui_test

import (
	"log"
	"strconv"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/text"
)

type incrementMsg struct{}

type model struct{ count int }

var _ tui.Model[model] = model{}

func (m model) Init() tui.Cmd {
	return cmd.SetTitle("Counter")
}

func (m model) Update(msg tui.Msg) (model, tui.Cmd) {
	switch msg := msg.(type) {
	case incrementMsg:
		m.count++
	case tui.KeyMsg:
		if keybind.String(msg) == "q" {
			return m, cmd.Quit{}
		}
	}
	return m, nil
}

func (m model) View() tui.Widget {
	return column.New(
		text.New("Count: "+strconv.Itoa(m.count)),
		row.New(
			button.New(text.New("Increment")).
				OnClick(incrementMsg{}).
				Focused(true),
		),
	)
}

// A counter that counts clicks on a button and quits on q.
func Example() {
	if err := tui.NewApplication(model{}).Run(); err != nil {
		log.Fatal(err)
	}
}
