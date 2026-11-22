package chat

import (
	"strings"
	"time"

	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/text"
)

// spinnerFrames are the turns of the spinner, in ASCII so every terminal font has them.
var spinnerFrames = [...]string{"-", "\\", "|", "/"}

// typingTickMsg advances the typing animation by a frame; ticks of an earlier run, by generation, are dropped.
type typingTickMsg struct{ generation int }

// typingInterval returns the time between two frames of animation, or 0 when it does not move.
func typingInterval(animation config.TypingAnimation) time.Duration {
	switch animation {
	case config.TypingAnimationOff:
		return 0
	case config.TypingAnimationDots:
		return 400 * time.Millisecond
	case config.TypingAnimationSpinner:
		return 150 * time.Millisecond
	case config.TypingAnimationPulsing:
		return 300 * time.Millisecond
	}
	panic("invariant: unknown typing animation")
}

// startTyping starts the animation unless one runs or it does not move; it stops on its own once nobody types.
func (m *Model) startTyping() tui.Cmd {
	interval := typingInterval(m.cfg.TypingIndicator.Animation)
	if m.typingAnimating || interval == 0 {
		return nil
	}
	m.typingAnimating = true
	m.typingGeneration++
	m.typingFrame = 0
	return tickTyping(m.typingGeneration, interval)
}

func tickTyping(generation int, interval time.Duration) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		time.Sleep(interval)
		return typingTickMsg{generation}
	})
}

// advanceTyping moves the animation a frame and schedules the next, or ends it when nobody types or it was turned off.
func (m *Model) advanceTyping(msg typingTickMsg) tui.Cmd {
	if !m.typingAnimating || msg.generation != m.typingGeneration {
		return nil
	}
	interval := typingInterval(m.cfg.TypingIndicator.Animation)
	if len(m.typers) == 0 || interval == 0 {
		m.typingAnimating = false
		return nil
	}
	m.typingFrame++
	return tickTyping(m.typingGeneration, interval)
}

// typingLabel returns line, who is typing, as the footer drawn at frame of animation in the footer style for focused, or nil for no line.
func typingLabel(line string, animation config.TypingAnimation, frame int, style tui.Style) tui.Widget {
	if line == "" {
		return nil
	}
	switch animation {
	case config.TypingAnimationOff:
		line += "..."
	case config.TypingAnimationDots:
		// The dots take their full width always, so the line does not shift.
		dots := frame%3 + 1
		line += strings.Repeat(".", dots) + strings.Repeat(" ", 3-dots)
	case config.TypingAnimationSpinner:
		line = spinnerFrames[frame%len(spinnerFrames)] + " " + line + "..."
	case config.TypingAnimationPulsing:
		line += "..."
		switch frame % 4 {
		case 0:
			style = style.Dim(true)
		case 2:
			style = style.Bold(true)
		}
	default:
		panic("invariant: unknown typing animation")
	}
	return text.New(line).Style(style).Wrapping(text.WrappingNone)
}
