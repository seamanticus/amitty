// Package tint merges a style into everything a widget draws, as for a selected or alternate item.
package tint

import (
	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/layout"
)

// Screen merges Style under every cell drawn through it, so an item can be highlighted without rendering it differently from the rest.
type Screen struct {
	tui.Screen
	Style tui.Style
}

func (s *Screen) SetContent(x int, y int, primary rune, combining []rune, style tui.Style) {
	s.Screen.SetContent(x, y, primary, combining, tui.MergeStyle(s.Style, style))
}

func (s *Screen) Put(x int, y int, str string, style tui.Style) (string, int) {
	return s.Screen.Put(x, y, str, tui.MergeStyle(s.Style, style))
}

func (s *Screen) FillArea(x, y, width, height int, r rune, style tui.Style) {
	s.Screen.FillArea(x, y, width, height, r, tui.MergeStyle(s.Style, style))
}

func (s *Screen) PutStr(x int, y int, str string) {
	s.PutStrStyled(x, y, str, tcell.StyleDefault)
}

func (s *Screen) PutStrStyled(x int, y int, str string, style tui.Style) {
	s.Screen.PutStrStyled(x, y, str, tui.MergeStyle(s.Style, style))
}

// Widget draws its child over its area filled with a style, which every cell the child draws is merged over.
type Widget struct {
	child tui.Widget
	style tui.Style
}

var _ tui.Widget = Widget{}

// New returns child tinted with style.
func New(child tui.Widget, style tui.Style) Widget {
	return Widget{child: child, style: style}
}

// Size returns the size of the child.
func (w Widget) Size() (width, height layout.Length) { return w.child.Size() }

// Layout lays out the child.
func (w Widget) Layout(limits layout.Limits) layout.Size { return w.child.Layout(limits) }

// Draw fills area with the style and draws the child through a Screen that merges it.
func (w Widget) Draw(screen tui.Screen, area tui.Rectangle) {
	tinted := &Screen{Screen: screen, Style: w.style}
	tinted.FillArea(area.X, area.Y, area.Width, area.Height, ' ', tcell.StyleDefault)
	w.child.Draw(tinted, area)
}

// Handle passes msg to the child.
func (w Widget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg { return w.child.Handle(msg, area) }
