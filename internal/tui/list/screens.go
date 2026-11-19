package list

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
)

// styledScreen merges a style into every cell drawn through it. The list wraps the cursor item's screen with it so the selected item can be highlighted without rendering it differently from the rest.
type styledScreen struct {
	tui.Screen
	style tui.Style
}

func (s *styledScreen) SetContent(x int, y int, primary rune, combining []rune, style tui.Style) {
	s.Screen.SetContent(x, y, primary, combining, tui.MergeStyle(s.style, style))
}

func (s *styledScreen) Put(x int, y int, str string, style tui.Style) (string, int) {
	return s.Screen.Put(x, y, str, tui.MergeStyle(s.style, style))
}

func (s *styledScreen) FillArea(x, y, width, height int, r rune, style tui.Style) {
	s.Screen.FillArea(x, y, width, height, r, tui.MergeStyle(s.style, style))
}

func (s *styledScreen) PutStr(x int, y int, str string) {
	s.PutStrStyled(x, y, str, tcell.StyleDefault)
}

func (s *styledScreen) PutStrStyled(x int, y int, str string, style tui.Style) {
	s.Screen.PutStrStyled(x, y, str, tui.MergeStyle(s.style, style))
}
