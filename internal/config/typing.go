package config

// TypingAnimation is how the line saying who is typing moves while someone is.
type TypingAnimation uint8

const (
	// TypingAnimationOff shows the line still.
	TypingAnimationOff TypingAnimation = iota
	// TypingAnimationDots grows its trailing dots one at a time.
	TypingAnimationDots
	// TypingAnimationSpinner turns a spinner before it.
	TypingAnimationSpinner
	// TypingAnimationPulsing cycles it between dim, normal and bold.
	TypingAnimationPulsing
	typingAnimationCount
)

var typingAnimations = enum[TypingAnimation]{
	{"off", TypingAnimationOff},
	{"dots", TypingAnimationDots},
	{"spinner", TypingAnimationSpinner},
	{"pulsing", TypingAnimationPulsing},
}

func (a *TypingAnimation) UnmarshalText(text []byte) (err error) {
	*a, err = typingAnimations.parse(string(text))
	return err
}

func (a TypingAnimation) MarshalText() ([]byte, error) { return typingAnimations.text(a) }

func (TypingAnimation) choices() []string { return typingAnimations.names() }
