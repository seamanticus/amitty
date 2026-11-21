package login

import (
	"strings"

	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// logo is the name drawn in one-cell block characters, its rows padded to logoWidth.
const logo = "" +
	"                         ▐█▌   ███    ███            \n" +
	"  ▄█████    ██████████        ██████ ██████ ███   ███\n" +
	"      ███  ███  ██  ███  ███   ███    ███    ██  ███ \n" +
	"▃▅█▀▀▀███  ███  ██  ███  ███   ███    ███     ████▀  \n" +
	"██▂   ███  ███  ██  ███  ███   ███    ███      ███   \n" +
	" ▜███▙▜██▙ ███  ██  ███  ▜██▙  ███    ███     ███    \n" +
	"                                             ██▀     "

const (
	logoWidth  = 53
	logoHeight = 7
	// logoSplit is the column where "tty" starts, drawn in the accent.
	logoSplit = 30
)

// logoView draws the logo with "ami" in base and "tty" in accent. Every character is one cell wide, so a rune index is a column.
func logoView(base, accent tui.Style) tui.Widget {
	var spans []text.Span
	for i, line := range strings.Split(logo, "\n") {
		if i > 0 {
			spans = append(spans, text.Span{Text: "\n"})
		}
		runes := []rune(line)
		split := min(logoSplit, len(runes))
		spans = append(spans,
			text.Span{Text: string(runes[:split]), Style: base},
			text.Span{Text: string(runes[split:]), Style: accent},
		)
	}
	return text.Rich(spans...).Wrapping(text.WrappingNone)
}
