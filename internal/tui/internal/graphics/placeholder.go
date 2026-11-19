// Package graphics holds what the image widget and the application share about images drawn in cells: placeholders, which name an image and a cell of it, and the sixel format.
package graphics

import (
	"slices"
	"strings"

	"github.com/gdamore/tcell/v3/color"
)

// Placeholder is kitty's placeholder character, followed in each cell by diacritics for the row, the column and the high byte of the image's id.
// See https://sw.kovidgoyal.net/kitty/graphics-protocol/#unicode-placeholders.
const Placeholder = '\U0010EEEE'

// SixelPlaceholder marks the cells of a sixel image in the same way. The application replaces it before the terminal sees it.
const SixelPlaceholder = '\U0010EEEF'

// MaxID is the highest image id: the low byte has 248 palette indexes, from 8 so tcell does not send them as basic colors, and the high byte a diacritic.
const MaxID = 248 * 256

// MaxCells is the most rows or columns a placeholder can name.
const MaxCells = len(diacritics)

// MinAlpha is the alpha below which a pixel is drawn as transparent, so antialiased edges do not show as dark cells.
const MinAlpha = 50

// Cell returns the text and foreground of the cell at row and col of image id, from 1 to MaxID, with row and col below MaxCells.
func Cell(id, row, col int, sixel bool) (string, color.Color) {
	mark := Placeholder
	if sixel {
		mark = SixelPlaceholder
	}
	fg, msb := split(id)
	return string([]rune{mark, diacritics[row], diacritics[col], diacritics[msb]}), color.PaletteColor(fg)
}

// TerminalID returns id as kitty's graphics protocol sends it: the high byte in the top byte and the foreground in the low one.
func TerminalID(id int) int {
	fg, msb := split(id)
	return msb<<24 | fg
}

func split(id int) (fg, msb int) {
	return (id-1)%248 + 8, (id - 1) / 248
}

// ParseSixelCell returns the image id, row and column that a cell's text and foreground name, and whether they are a sixel placeholder made by Cell.
func ParseSixelCell(text string, fg color.Color) (id, row, col int, ok bool) {
	rest, ok := strings.CutPrefix(text, string(SixelPlaceholder))
	if !ok || !fg.Valid() || fg.IsRGB() {
		return 0, 0, 0, false
	}
	index := int(fg &^ color.IsValid)
	if index < 8 || index > 255 {
		return 0, 0, 0, false
	}

	var numbers [3]int
	runes := []rune(rest)
	if len(runes) != len(numbers) {
		return 0, 0, 0, false
	}
	for i, r := range runes {
		if numbers[i], ok = slices.BinarySearch(diacritics[:], r); !ok {
			return 0, 0, 0, false
		}
	}
	id = numbers[2]*248 + index - 8 + 1
	if id > MaxID {
		return 0, 0, 0, false
	}
	return id, numbers[0], numbers[1], true
}

// IsSixelCell reports whether a cell's text starts with the sixel placeholder, valid or not.
func IsSixelCell(text string) bool {
	return strings.HasPrefix(text, string(SixelPlaceholder))
}
