// Package pathprompt asks for a file path inside the terminal, completing it from the file system.
package pathprompt

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/ayn2op/tview/column"
	"github.com/ayn2op/tview/keybind"
	"github.com/ayn2op/tview/text"
	"github.com/ayn2op/tview/textinput"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/ui"
)

// ShowMsg asks to open the prompt titled Title with Path typed in; Submit makes what to do with the path entered.
type ShowMsg struct {
	Title  string
	Path   string
	Submit func(path string) tview.Cmd
}

// SubmittedMsg reports that a path was entered; Cmd acts on it.
type SubmittedMsg struct {
	Cmd tview.Cmd
}

// CancelMsg reports that the prompt was dismissed.
type CancelMsg struct{}

type (
	inputMsg struct {
		text   string
		cursor int
	}
	submitMsg struct{}
	// listedMsg holds the names in dir, as typed, with directories ending in a slash.
	listedMsg struct {
		dir   string
		names []string
	}
)

// Model is a one-line path input over the entries of the directory typed so far.
type Model struct {
	cfg  *config.Config
	show ShowMsg

	text string
	// cursor is the byte offset of the cursor in text.
	cursor int

	// dir is the directory part of text that names lists, once requested.
	dir       string
	requested bool
	names     []string
}

// New returns the prompt asked for by show and the command that lists the directory typed in it.
func New(cfg *config.Config, show ShowMsg) (Model, tview.Cmd) {
	m := Model{cfg: cfg, show: show, text: show.Path, cursor: len(show.Path)}
	return m, m.relist()
}

var _ tview.Model[Model] = Model{}

func (Model) Init() tview.Cmd { return nil }

func (m Model) Update(msg tview.Msg) (Model, tview.Cmd) {
	switch msg := msg.(type) {
	case inputMsg:
		m.text, m.cursor = msg.text, msg.cursor
		return m, m.relist()
	case listedMsg:
		if msg.dir == m.dir {
			m.names = msg.names
		}
		return m, nil
	case submitMsg:
		path, ok := expand(m.text)
		if !ok {
			return m, nil
		}
		submit := m.show.Submit
		return m, cmd.Task(func() tview.Msg { return SubmittedMsg{Cmd: submit(path)} })
	case tview.KeyMsg:
		keybinds := m.cfg.Keybinds.PathPrompt
		switch {
		case keybind.Matches(msg, keybinds.Complete.Keybind):
			m.text = complete(m.text, m.names)
			m.cursor = len(m.text)
			return m, m.relist()
		case keybind.Matches(msg, keybinds.Cancel.Keybind):
			return m, cmd.Task(func() tview.Msg { return CancelMsg{} })
		}
	}
	return m, nil
}

// relist lists the directory typed when it is not the one listed.
func (m *Model) relist() tview.Cmd {
	dir, _ := split(m.text)
	if m.requested && dir == m.dir {
		return nil
	}
	m.dir, m.requested, m.names = dir, true, nil
	return cmd.Task(func() tview.Msg { return listedMsg{dir: dir, names: list(dir)} })
}

// View shows the input in a box titled as asked, over the entries that complete it.
func (m Model) View() tview.Widget {
	cfg := m.cfg
	input := textinput.New(m.text, m.cursor).
		Focused(true).
		Keybind(inputKeybind(cfg.Keybinds.PathPrompt.Submit.Keybind)).
		OnInput(func(text string, cursor int) tview.Msg { return inputMsg{text, cursor} }).
		OnSubmit(submitMsg{})
	_, prefix := split(m.text)
	entries := text.New(strings.Join(matches(m.names, prefix), "\n")).Wrapping(text.WrappingNone)
	return ui.Box(column.New(input, entries), cfg, true).Title(ui.Label(m.show.Title, cfg.Theme.Title, true))
}

// inputKeybind is the input's default keybind with submit as the only key that submits.
func inputKeybind(submit keybind.Keybind) func(tview.KeyMsg) textinput.Action {
	return func(key tview.KeyMsg) textinput.Action {
		if keybind.Matches(key, submit) {
			return textinput.ActionSubmit
		}
		if action := textinput.DefaultKeybind(key); action != textinput.ActionSubmit {
			return action
		}
		return textinput.ActionNone
	}
}

// split cuts text after its last slash into the directory typed and the name being typed.
func split(text string) (dir, prefix string) {
	i := strings.LastIndexByte(text, '/')
	return text[:i+1], text[i+1:]
}

// matches returns the names starting with prefix; hidden ones only when prefix starts with a dot.
func matches(names []string, prefix string) []string {
	var found []string
	for _, name := range names {
		if strings.HasPrefix(name, prefix) && (strings.HasPrefix(prefix, ".") || !strings.HasPrefix(name, ".")) {
			found = append(found, name)
		}
	}
	return found
}

// complete extends the name being typed in text by what every matching name shares.
func complete(text string, names []string) string {
	dir, prefix := split(text)
	found := matches(names, prefix)
	if len(found) == 0 {
		return text
	}
	common := found[0]
	for _, name := range found[1:] {
		n := 0
		for n < len(common) && n < len(name) && common[n] == name[n] {
			n++
		}
		common = common[:n]
	}
	return dir + common
}

// expand returns the path text names, with a leading ~ for the home directory, or false if text is blank.
func expand(text string) (string, bool) {
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	if text != "~" && !strings.HasPrefix(text, "~/") {
		return text, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return text, true
	}
	return filepath.Join(home, text[1:]), true
}

// list returns the sorted names in dir as typed (the working directory when empty), directories ending in a slash, or nil if it cannot be read.
func list(dir string) []string {
	path := "."
	if dir != "" {
		path, _ = expand(dir)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if isDir(path, entry) {
			name += "/"
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// isDir reports whether entry, in dir, is a directory or a link to one.
func isDir(dir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, entry.Name()))
	return err == nil && info.IsDir()
}
