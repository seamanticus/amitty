package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// proof: exhaustive
func TestTypingLabel(t *testing.T) {
	if typingLabel("", config.TypingAnimationDots, 0, tui.Style{}) != nil {
		t.Fatal("a label for nobody typing")
	}
	// Each animation over its frames, which repeat.
	want := map[config.TypingAnimation][]string{
		config.TypingAnimationOff:     {"a is typing...", "a is typing..."},
		config.TypingAnimationDots:    {"a is typing.  ", "a is typing.. ", "a is typing...", "a is typing.  "},
		config.TypingAnimationSpinner: {"\u280b a is typing...", "\u2819 a is typing...", "\u2839 a is typing...", "\u2838 a is typing...", "\u283c a is typing...", "\u2834 a is typing...", "\u2826 a is typing...", "\u2827 a is typing...", "\u2807 a is typing...", "\u280f a is typing...", "\u280b a is typing..."},
		config.TypingAnimationPulsing: {"a is typing...", "a is typing..."},
	}
	for animation := range config.TypingAnimation(len(want)) {
		frames, ok := want[animation]
		if !ok {
			t.Fatalf("animation %d has no expected frames", animation)
		}
		for frame, line := range frames {
			if got := strings.TrimSuffix(discordtest.Draw(t, typingLabel("a is typing", animation, frame, tui.Style{}), len([]rune(line)), 1), "\n"); got != line {
				t.Errorf("animation %d frame %d = %q, want %q", animation, frame, got, line)
			}
		}
	}
	if _, err := config.TypingAnimation(len(want)).MarshalText(); err == nil {
		t.Fatal("more animations than this test checks")
	}
	// Pulsing changes the style alone: dim, normal, bold, normal.
	for frame, want := range []struct{ dim, bold bool }{{true, false}, {}, {false, true}, {}} {
		screen := vt.NewMockTerm(vt.MockOptSize{X: 1, Y: 1})
		term, err := tcell.NewTerminfoScreenFromTty(screen)
		if err != nil || term.Init() != nil {
			t.Fatal(err)
		}
		typingLabel("a", config.TypingAnimationPulsing, frame, tui.Style{}).Draw(term, tui.Rectangle{Width: 1, Height: 1})
		_, style, _ := term.Get(0, 0)
		term.Fini()
		if dim, bold := style.HasDim(), style.HasBold(); dim != want.dim || bold != want.bold {
			t.Errorf("pulsing frame %d: dim %v bold %v, want %v %v", frame, dim, bold, want.dim, want.bold)
		}
	}
}

// evidence: witnesses
func TestTypingAnimation(t *testing.T) {
	m := newTestModel(t)
	m.cfg.TypingIndicator.Animation = config.TypingAnimationDots
	m.typers[1] = time.Now().Add(time.Minute)
	if m.startTyping() == nil || m.startTyping() != nil {
		t.Fatal("want one animation started, not two")
	}
	stale := typingTickMsg{m.typingGeneration - 1}
	if m.advanceTyping(stale) != nil || m.typingFrame != 0 {
		t.Fatal("a tick of an earlier run moved the animation")
	}
	if m.advanceTyping(typingTickMsg{m.typingGeneration}) == nil || m.typingFrame != 1 {
		t.Fatal("a tick did not move the animation a frame")
	}
	// Nobody typing ends it, and a new typer starts it again.
	clear(m.typers)
	if m.advanceTyping(typingTickMsg{m.typingGeneration}) != nil || m.typingAnimating {
		t.Fatal("the animation kept going with nobody typing")
	}
	m.cfg.TypingIndicator.Animation = config.TypingAnimationOff
	if _, ok := m.addTyper(1).(cmd.Task); !ok || m.typingAnimating {
		t.Fatal("off animates, or the typer does not expire")
	}
}
