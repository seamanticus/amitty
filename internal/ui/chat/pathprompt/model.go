// Package pathprompt asks for a file path inside the terminal, completing it from the file system.
package pathprompt

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/list"
	"github.com/seamanticus/amitty/internal/tui/scrollbar"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
	"github.com/seamanticus/amitty/internal/tui/viewport"
	"github.com/seamanticus/amitty/internal/ui"
)

// ShowMsg asks to open the prompt titled Title with Path typed in; Submit makes what to do with the path entered.
type ShowMsg struct {
	Title  string
	Path   string
	Submit func(path string) tui.Cmd
}

// SubmittedMsg reports that a path was entered; Cmd acts on it.
type SubmittedMsg struct {
	Cmd tui.Cmd
}

// CancelMsg reports that the prompt was dismissed.
type CancelMsg struct{}

type (
	inputMsg struct {
		text   string
		cursor int
	}
	submitMsg struct{}
	scrollMsg viewport.ScrollState
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

	// highlight is one more than the index of the highlighted entry among the matches, or 0 for none.
	highlight   int
	scrollState viewport.ScrollState
}

// New returns the prompt asked for by show and the command that lists the directory typed in it.
func New(cfg *config.Config, show ShowMsg) (Model, tui.Cmd) {
	m := Model{cfg: cfg, show: show, text: show.Path, cursor: len(show.Path)}
	return m, m.relist()
}

var _ tui.Model[Model] = Model{}

func (Model) Init() tui.Cmd { return nil }

func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case inputMsg:
		if msg.text != m.text {
			m.highlight = 0
		}
		m.text, m.cursor = msg.text, msg.cursor
		return m, m.relist()
	case listedMsg:
		if msg.dir == m.dir {
			m.names, m.highlight = msg.names, 0
		}
		return m, nil
	case scrollMsg:
		m.scrollState = viewport.ScrollState(msg)
		return m, nil
	case submitMsg:
		if entry, ok := m.highlighted(); ok {
			m.insert(entry)
			if strings.HasSuffix(entry, "/") {
				return m, m.relist()
			}
		}
		path, ok := expand(m.text)
		if !ok {
			return m, nil
		}
		submit := m.show.Submit
		return m, cmd.Task(func() tui.Msg { return SubmittedMsg{Cmd: submit(path)} })
	case tui.KeyMsg:
		keybinds := m.cfg.Keybinds.PathPrompt
		switch {
		case keybind.Matches(msg, keybinds.SelectUp.Keybind):
			m.highlight = max(m.highlight-1, 0)
			m.scrollState.ScrollToTarget()
		case keybind.Matches(msg, keybinds.SelectDown.Keybind):
			m.highlight = min(m.highlight+1, len(m.found()))
			m.scrollState.ScrollToTarget()
		case keybind.Matches(msg, keybinds.Complete.Keybind):
			if entry, ok := m.highlighted(); ok {
				m.insert(entry)
			} else {
				m.text = complete(m.text, m.names)
				m.cursor = len(m.text)
			}
			return m, m.relist()
		case keybind.Matches(msg, keybinds.Cancel.Keybind):
			return m, cmd.Task(func() tui.Msg { return CancelMsg{} })
		}
	}
	return m, nil
}

// found returns the entries that match the name typed.
func (m Model) found() []string {
	_, prefix := split(m.text)
	return matches(m.names, prefix)
}

// highlighted returns the highlighted entry, or false if there is none.
func (m Model) highlighted() (string, bool) {
	found := m.found()
	if m.highlight < 1 || m.highlight > len(found) {
		return "", false
	}
	return found[m.highlight-1], true
}

// insert replaces the name typed by entry and clears the highlight.
func (m *Model) insert(entry string) {
	dir, _ := split(m.text)
	m.text, m.highlight = dir+entry, 0
	m.cursor = len(m.text)
}

// relist lists the directory typed when it is not the one listed.
func (m *Model) relist() tui.Cmd {
	dir, _ := split(m.text)
	if m.requested && dir == m.dir {
		return nil
	}
	m.dir, m.requested, m.names = dir, true, nil
	return cmd.Task(func() tui.Msg { return listedMsg{dir: dir, names: readNames(dir)} })
}

// View shows the input in a box titled as asked, over the entries that complete it.
func (m Model) View() tui.Widget {
	cfg := m.cfg
	input := textinput.New(m.text, m.cursor).
		Focused(true).
		Keybind(inputKeybind(cfg.Keybinds.PathPrompt.Submit.Keybind)).
		OnInput(func(text string, cursor int) tui.Msg { return inputMsg{text, cursor} }).
		OnSubmit(submitMsg{})
	found := m.found()
	l := list.New(m.highlight-1, len(found), func(i int) tui.Widget {
		return text.New(found[i]).Wrapping(text.WrappingNone)
	}).SelectedStyle(tcell.StyleDefault.Reverse(true))
	entries := viewport.New(l, m.scrollState).
		Target(l.Target(m.highlight-1)).
		ScrollBar(scrollbar.New(), cfg.UI.ScrollBar.Visibility.ScrollBarVisibility).
		OnChange(func(s *viewport.ScrollState) tui.Msg { return scrollMsg(*s) })
	return ui.Box(column.New(input, entries), cfg, true).Title(ui.Label(m.show.Title, cfg.Theme.Title, true))
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

// readNames returns the sorted names in dir as typed (the working directory when empty), directories ending in a slash, or nil if it cannot be read.
func readNames(dir string) []string {
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
