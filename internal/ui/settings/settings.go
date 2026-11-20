// Package settings is the panel that edits config.toml: every option, listed by its TOML path, toggled, picked or typed, and written back to its own line of the file.
package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/picker"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
	"github.com/seamanticus/amitty/internal/ui"
	uipicker "github.com/seamanticus/amitty/internal/ui/picker"
)

// Msg carries a message of the panel, from its widgets or its commands, which the model showing it passes to Update.
type Msg struct{ msg tui.Msg }

// ClosedMsg reports that the panel was closed.
type ClosedMsg struct{}

// PreviewMsg asks to show Config, which the file would give with a choice being browsed, without writing it; or, once the choice is dropped, what the file gives.
type PreviewMsg struct{ Config *config.Config }

// SavedMsg reports that an option was written to the file, which now gives Config. Restart reports whether the option applies only after a restart.
type SavedMsg struct {
	Config  *config.Config
	Path    string
	Restart bool
}

type (
	pickedMsg struct{ option config.Option }
	chosenMsg struct{ value string }
	inputMsg  struct {
		text   string
		cursor int
	}
	submitMsg struct{}
	backMsg   struct{}
	savedMsg  struct {
		option config.Option
		cfg    *config.Config
	}
	failedMsg struct{ err error }
)

type step uint8

const (
	stepList step = iota
	stepChoice
	stepText
)

// Model is the panel: the list of options, and the option being edited.
type Model struct {
	cfg     *config.Config
	file    string
	step    step
	list    uipicker.Model
	choices uipicker.Model
	editing config.Option
	// previewed is the choice last previewed while browsing a profile.
	previewed string
	text      string
	cursor    int
	// status is the result of the last save, shown under the list.
	status string
}

// New returns the panel for cfg, which the file at path gives.
func New(cfg *config.Config, path string) Model {
	m := Model{cfg: cfg, file: path, list: uipicker.NewModel(cfg, "Settings"), choices: uipicker.NewModel(cfg, "")}
	m.list.SetItems(items(config.Options(cfg)))
	return m
}

func items(options []config.Option) picker.Items {
	list := make(picker.Items, len(options))
	for i, option := range options {
		value := option.Value
		switch {
		case option.Kind == config.OptionFileOnly:
			value += " (edit in the file)"
		case option.Restart:
			value += " (on restart)"
		}
		list[i] = picker.Item{Content: text.New(option.Path + " = " + value).Wrapping(text.WrappingNone), FilterText: option.Path, Reference: option}
	}
	return list
}

// wrap returns msg as the panel's.
func wrap(msg tui.Msg) tui.Msg { return Msg{msg} }

// Update changes the panel with msg, one of its own.
func (m Model) Update(msg Msg) (Model, tui.Cmd) {
	switch msg := msg.msg.(type) {
	case pickedMsg:
		return m.edit(msg.option)
	case chosenMsg:
		m.step = stepList
		return m, m.save(m.editing, msg.value)
	case inputMsg:
		m.text, m.cursor = msg.text, msg.cursor
		return m, nil
	case submitMsg:
		return m, m.save(m.editing, m.text)
	case backMsg:
		if m.step == stepList {
			return m, cmd.Task(func() tui.Msg { return ClosedMsg{} })
		}
		var revert tui.Cmd
		if m.step == stepChoice && m.previewed != m.editing.Value {
			revert = m.preview("")
		}
		m.step = stepList
		return m, revert
	case savedMsg:
		m.step = stepList
		m.status = "Saved " + msg.option.Path + "."
		if msg.option.Restart {
			m.status += " It applies after a restart."
		}
		m.list.ReplaceItems(items(config.Options(msg.cfg)))
		return m, cmd.Task(func() tui.Msg { return SavedMsg{Config: msg.cfg, Path: msg.option.Path, Restart: msg.option.Restart} })
	case failedMsg:
		m.status = msg.err.Error()
		return m, nil
	}

	var command tui.Cmd
	switch m.step {
	case stepList:
		m.list, command = m.list.Update(msg.msg)
	case stepChoice:
		m.choices, command = m.choices.Update(msg.msg)
		// A profile is shown as the cursor reaches it, so it can be judged before it is chosen.
		if item, ok := m.choices.Selected(); ok && m.editing.Path == previewPath {
			if value, ok := item.Reference.(string); ok && value != m.previewed {
				m.previewed = value
				command = cmd.Batch{command, m.preview(value)}
			}
		}
	case stepText:
	}
	return m, command
}

// edit starts editing option: a bool is toggled at once, a choice opens the list of its values, and other values are typed.
func (m Model) edit(option config.Option) (Model, tui.Cmd) {
	m.editing, m.status = option, ""
	switch option.Kind {
	case config.OptionBool:
		value := "true"
		if option.Value == "true" {
			value = "false"
		}
		return m, m.save(option, value)
	case config.OptionChoice:
		// A short set of values is stepped through like a bool; a long one is picked from a list.
		if option.Path != previewPath {
			next := (slices.Index(option.Choices, option.Value) + 1) % len(option.Choices)
			return m, m.save(option, option.Choices[next])
		}
		m.step, m.previewed = stepChoice, option.Value
		choices := make(picker.Items, len(option.Choices))
		for i, choice := range option.Choices {
			label := choice
			if choice == `""` {
				label = `"" (none: the terminal's colors)`
			}
			choices[i] = picker.Item{Content: text.New(label), FilterText: choice, Reference: choice}
		}
		m.choices = uipicker.NewModel(m.cfg, option.Path)
		m.choices.SetItems(choices)
		return m, nil
	case config.OptionText:
		m.step, m.text, m.cursor = stepText, option.Value, len(option.Value)
		return m, nil
	case config.OptionFileOnly:
		m.status = option.Path + " holds entries of your own: edit it in " + m.file + "."
		return m, nil
	}
	panic("invariant: unknown option kind")
}

// previewPath is the one option whose choices are previewed as the cursor moves over them, as a profile is judged by looking at it.
const previewPath = "theme_profile"

// preview returns the command that shows the config the file gives with the option being edited set to value, without writing it, or as it is for "".
func (m Model) preview(value string) tui.Cmd {
	file, path := m.file, m.editing.Path
	return cmd.Task(func() tui.Msg {
		data, err := os.ReadFile(file)
		if err != nil && !os.IsNotExist(err) {
			return nil
		}
		if value != "" {
			if data, err = config.SetOption(data, path, value); err != nil {
				return nil
			}
		}
		cfg, err := config.Parse(data)
		if err != nil {
			return nil
		}
		return PreviewMsg{Config: cfg}
	})
}

// save writes value to option's line of the file and reports the config the file then gives.
func (m Model) save(option config.Option, value string) tui.Cmd {
	file := m.file
	return cmd.Task(func() tui.Msg {
		cfg, err := write(file, option.Path, value)
		if err != nil {
			return wrap(failedMsg{err})
		}
		return wrap(savedMsg{option: option, cfg: cfg})
	})
}

// write sets the option at path to value in the file, replacing it in one rename so a failed write keeps the old file.
func write(file, path, value string) (*config.Config, error) {
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}
	edited, err := config.SetOption(data, path, value)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Parse(edited)
	if err != nil {
		return nil, err
	}

	mode := os.FileMode(0o600)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".config-*.toml")
	if err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	// After the rename there is nothing left to remove.
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.Write(edited)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), mode)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), file)
	}
	if err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	return cfg, nil
}

// View shows the step the panel is at, with the selected option's documentation and the last save's result below.
func (m Model) View() tui.Widget {
	var body tui.Widget
	doc := m.status
	switch m.step {
	case stepList:
		body = m.list.View(func(item picker.Item) tui.Msg {
			option, ok := item.Reference.(config.Option)
			if !ok {
				return nil
			}
			return pickedMsg{option}
		}, backMsg{})
		if item, ok := m.list.Selected(); ok && doc == "" {
			if option, ok := item.Reference.(config.Option); ok {
				doc = option.Doc
			}
		}
	case stepChoice:
		body = m.choices.View(func(item picker.Item) tui.Msg {
			value, ok := item.Reference.(string)
			if !ok {
				return nil
			}
			return chosenMsg{value}
		}, backMsg{})
		doc = m.editing.Doc
	case stepText:
		keybinds := m.cfg.Keybinds.Picker
		input := textinput.New(m.text, m.cursor).
			Focused(true).
			Keybind(func(key tui.KeyMsg) textinput.Action {
				switch {
				case keybind.Matches(key, keybinds.Select.Keybind):
					return textinput.ActionSubmit
				case keybind.Matches(key, keybinds.Cancel.Keybind):
					return textinput.ActionNone
				}
				if action := textinput.DefaultKeybind(key); action != textinput.ActionSubmit {
					return action
				}
				return textinput.ActionNone
			}).
			OnInput(func(text string, cursor int) tui.Msg { return inputMsg{text, cursor} }).
			OnSubmit(submitMsg{})
		body = ui.Box(cancelOn(input, keybinds.Cancel.Keybind), m.cfg, true).Title(ui.Label(m.editing.Path+" (TOML)", m.cfg.Theme.Title, true)).Height(layout.Fixed(3))
		if doc == "" {
			doc = m.editing.Doc
		}
	}
	return wrapped{column.New(body, text.New(doc).Style(m.cfg.Theme.Footer.NormalStyle.Style).Height(layout.Shrink))}
}

// wrapped makes the messages the panel's widgets produce the panel's; anything else, such as a gateway event passing by, goes on as it is.
type wrapped struct{ tui.Widget }

func (w wrapped) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	out := w.Widget.Handle(msg, area)
	if owns(out) {
		return wrap(out)
	}
	return out
}

// owns reports whether msg is one the panel's widgets make.
func owns(msg tui.Msg) bool {
	switch msg.(type) {
	case pickedMsg, chosenMsg, inputMsg, submitMsg, backMsg:
		return true
	}
	var picker uipicker.Model
	return picker.Owns(msg)
}

// cancelOn turns the cancel key into backMsg before child sees it, as a text input takes every key.
func cancelOn(child tui.Widget, cancel keybind.Keybind) tui.Widget { return canceling{child, cancel} }

type canceling struct {
	tui.Widget
	cancel keybind.Keybind
}

func (w canceling) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	if key, ok := msg.(tui.KeyMsg); ok && keybind.Matches(key, w.cancel) {
		return backMsg{}
	}
	return w.Widget.Handle(msg, area)
}

// ShortHelp returns the picker's keys, or those that save or leave a value being typed.
func (m Model) ShortHelp() []keybind.Keybind {
	keybinds := m.cfg.Keybinds.Picker
	if m.step == stepText {
		return []keybind.Keybind{keybinds.Select.WithHelp(keybinds.Select.Help().Key, "save"), keybinds.Cancel.WithHelp(keybinds.Cancel.Help().Key, "back")}
	}
	return keybinds.ShortHelp()
}

// FullHelp returns ShortHelp's keys, grouped.
func (m Model) FullHelp() [][]keybind.Keybind {
	if m.step == stepText {
		return [][]keybind.Keybind{m.ShortHelp()}
	}
	return m.cfg.Keybinds.Picker.FullHelp()
}
