package attachmentspicker

import (
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/picker"
	uipicker "github.com/seamanticus/amitty/internal/ui/picker"
)

type Model struct {
	uipicker.Model
}

func NewModel(cfg *config.Config) Model {
	return Model{uipicker.NewModel(cfg, "Attachments")}
}

var _ tui.Model[Model] = Model{}

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	var cmd tui.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

// View shows the picker in a box titled Attachments.
func (m Model) View() tui.Widget {
	return m.Model.View(func(item picker.Item) tui.Msg {
		action, ok := item.Reference.(tui.Cmd)
		if !ok {
			return nil
		}
		return SelectedMsg{Action: action}
	}, CancelMsg{})
}
