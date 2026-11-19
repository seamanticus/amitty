package backdrop

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
)

func TestWidgetDraw(t *testing.T) {
	screen := screentest.New(t, 4, 2)
	screen.Put(2, 1, "a", tcell.StyleDefault)
	New().Draw(screen, tui.Rectangle{Width: 4, Height: 2})
	str, style, _ := screen.Get(2, 1)
	if str != "a" || !style.HasDim() {
		t.Fatalf("cell = %q dim=%v, want %q dim=true", str, style.HasDim(), "a")
	}
}
