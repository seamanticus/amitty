package box

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/internal/screentest"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// areaWidget records the area it is drawn and handles messages in.
type areaWidget struct{ area *tui.Rectangle }

func (e areaWidget) Draw(_ tui.Screen, area tui.Rectangle) { *e.area = area }

func (e areaWidget) Handle(msg tui.Msg, area tui.Rectangle) tui.Msg {
	*e.area = area
	return msg
}

func TestWidgetDraw(t *testing.T) {
	screen := screentest.New(t, 8, 3)
	var inner tui.Rectangle
	New(areaWidget{&inner}).Borders(tui.BordersAll).Title(text.New("ab")).Draw(screen, tui.Rectangle{Width: 8, Height: 3})
	t.Run("title in the border", func(t *testing.T) {
		if got := screentest.Row(screen, 0, 8); got != "┌──ab──┐" {
			t.Fatalf("top = %q", got)
		}
	})
	t.Run("sides and bottom", func(t *testing.T) {
		for y, want := range map[int]string{1: "│      │", 2: "└──────┘"} {
			t.Run(fmt.Sprintf("row %d", y), func(t *testing.T) {
				if got := screentest.Row(screen, y, 8); got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			})
		}
	})
	t.Run("child inside the border", func(t *testing.T) {
		if want := (tui.Rectangle{X: 1, Y: 1, Width: 6, Height: 1}); inner != want {
			t.Fatalf("inner = %+v, want %+v", inner, want)
		}
	})
	t.Run("line", func(t *testing.T) {
		for name, tt := range map[string]struct {
			borders tui.Borders
			want    string
		}{
			"a top border alone is a line, without corners": {tui.BordersTop, "─── today ──"},
			"a corner only where two borders meet":          {tui.BordersTop | tui.BordersLeft, "┌── today ──"},
		} {
			t.Run(name, func(t *testing.T) {
				screen := screentest.New(t, 12, 1)
				New(areaWidget{new(tui.Rectangle)}).Borders(tt.borders).Title(text.New(" today ")).Draw(screen, tui.Rectangle{Width: 12, Height: 1})
				if got := screentest.Row(screen, 0, 12); got != tt.want {
					t.Fatalf("line = %q, want %q", got, tt.want)
				}
			})
		}
	})
	t.Run("cut title", func(t *testing.T) {
		screen := screentest.New(t, 8, 3)
		New(areaWidget{new(tui.Rectangle)}).Borders(tui.BordersAll).Title(text.New("abcdefgh")).Draw(screen, tui.Rectangle{Width: 8, Height: 3})
		if got := screentest.Row(screen, 0, 8); got != "┌abcdef┐" {
			t.Fatalf("top = %q", got)
		}
	})
	// evidence: witnesses
	t.Run("label inset", func(t *testing.T) {
		for _, tt := range []struct {
			width int
			title string
			want  string
		}{
			{10, "ab", "┌─ab─────┐"},
			// The inset gives way before the title is cut.
			{6, "abcd", "┌abcd┐"},
			{7, "abcd", "┌abcd─┐"},
		} {
			screen := screentest.New(t, tt.width, 3)
			New(areaWidget{new(tui.Rectangle)}).Borders(tui.BordersAll).LabelInset(1).Title(text.New(tt.title)).TitleAlignment(tui.HorizontalAlignmentLeft).Draw(screen, tui.Rectangle{Width: tt.width, Height: 3})
			if got := screentest.Row(screen, 0, tt.width); got != tt.want {
				t.Errorf("width %d: top = %q, want %q", tt.width, got, tt.want)
			}
		}
	})
	t.Run("title over styled cells", func(t *testing.T) {
		screen := screentest.New(t, 8, 3)
		screen.FillArea(0, 0, 8, 3, 'x', tcell.StyleDefault.Background(color.Red))
		New(areaWidget{new(tui.Rectangle)}).Borders(tui.BordersAll).Title(text.New("ab")).Draw(screen, tui.Rectangle{Width: 8, Height: 3})
		for x := 3; x <= 4; x++ {
			if _, style, _ := screen.Get(x, 0); style.GetBackground() != tcell.ColorDefault {
				t.Fatalf("title cell %d background = %v, want the box's", x, style.GetBackground())
			}
		}
	})
}

func TestWidgetHandle(t *testing.T) {
	var inner, title tui.Rectangle
	b := New(areaWidget{&inner}).Padding(1, 0, 2, 0).Title(areaWidget{&title}).TitleAlignment(tui.HorizontalAlignmentLeft)
	area := tui.Rectangle{Width: 8, Height: 3}

	t.Run("child inside the padding", func(t *testing.T) {
		b.Handle(nil, area)
		if want := (tui.Rectangle{X: 2, Y: 2, Width: 6, Height: 1}); inner != want {
			t.Fatalf("inner = %+v, want %+v", inner, want)
		}
	})
	t.Run("a click on the title goes to it", func(t *testing.T) {
		b.Handle(tui.MouseMsg{EventMouse: tcell.NewEventMouse(1, 0, tcell.Button1, tcell.ModNone), Action: tui.MouseLeftDown}, area)
		if want := (tui.Rectangle{X: 1, Y: 0, Width: 6, Height: 1}); title != want {
			t.Fatalf("title = %+v, want %+v", title, want)
		}
	})
}

func (areaWidget) Size() (width, height layout.Length) { return layout.Fill, layout.Fill }

func (areaWidget) Layout(limits layout.Limits) layout.Size {
	return layout.Atomic(limits, layout.Fill, layout.Fill)
}
