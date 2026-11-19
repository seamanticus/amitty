package tui_test

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
)

func TestPrint(t *testing.T) {
	for _, tt := range []struct {
		name      string
		text      string
		width     int
		alignment tui.HorizontalAlignment
		want      string
		drawn     int
	}{
		{"left", "ab", 6, tui.HorizontalAlignmentLeft, "ab    ", 2},
		{"center", "ab", 6, tui.HorizontalAlignmentCenter, "  ab  ", 2},
		{"right", "ab", 6, tui.HorizontalAlignmentRight, "    ab", 2},
		{"cut at the end", "abcdefgh", 6, tui.HorizontalAlignmentLeft, "abcdef", 6},
		{"cut at the start", "abcdefgh", 6, tui.HorizontalAlignmentRight, "cdefgh", 6},
		{"cut at both ends", "abcdefgh", 6, tui.HorizontalAlignmentCenter, "bcdefg", 6},
		{"wide cluster that does not fit", "abcde👍", 6, tui.HorizontalAlignmentLeft, "abcde ", 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			screen := screentest.New(t, 6, 1)
			if drawn := tui.Print(screen, tt.text, 0, 0, tt.width, tt.alignment, tcell.StyleDefault); drawn != tt.drawn {
				t.Fatalf("drawn = %d, want %d", drawn, tt.drawn)
			}
			if got := screentest.Row(screen, 0, 6); got != tt.want {
				t.Fatalf("row = %q, want %q", got, tt.want)
			}
		})
	}
}
