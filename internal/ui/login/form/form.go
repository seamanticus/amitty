// Package form holds the parts the login forms share.
package form

import (
	"github.com/gdamore/tcell/v3"
	"github.com/rockorager/go-uucode"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/button"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/row"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/tui/textinput"
)

// ButtonHeight is the rows a button takes: its label between two border lines.
const ButtonHeight = 3

// Button returns label in a rounded border, reversed while focused and underlined while hovered. The caller sets what it does.
func Button(label string, focused, hovered bool) button.Widget {
	label = " " + label + " "
	style := tcell.StyleDefault
	switch {
	case focused:
		style = style.Reverse(true)
	case hovered:
		style = style.Underline(true)
	}
	border := tcell.StyleDefault.Bold(focused)
	content := box.New(text.New(label).Style(style).Wrapping(text.WrappingNone)).
		Borders(tui.BordersAll).
		BorderSet(tui.BorderSetRound()).
		BorderStyle(border)
	width := uucode.StringWidth(label) + 2
	return button.New(content).
		Width(layout.Fixed(width)).
		Height(layout.Fixed(ButtonHeight)).
		Focused(focused).
		Hovered(hovered)
}

// FieldWidth is the cells an input takes, enough for an email address or a token's start.
const FieldWidth = 40

// Field returns input underlined so it shows as a line while empty, at most FieldWidth wide, after its label in a centered row.
func Field(name string, labelWidth int, input textinput.Widget) tui.Widget {
	input = input.Style(tcell.StyleDefault.Underline(true)).Width(layout.Fixed(FieldWidth))
	return row.New(Label(name, labelWidth), input).Width(layout.Shrink).Height(layout.Shrink)
}

// Label returns name as the label of an input, right-aligned in width cells so the colons of a column of labels line up.
func Label(name string, width int) tui.Widget {
	return text.New(name + " : ").Width(layout.Fixed(width)).AlignX(tui.HorizontalAlignmentRight).Wrapping(text.WrappingNone)
}

// LabelWidth returns the width of the widest label of names.
func LabelWidth(names ...string) int {
	width := 0
	for _, name := range names {
		width = max(width, uucode.StringWidth(name+" : "))
	}
	return width
}
