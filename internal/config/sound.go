package config

import (
	"fmt"
	"slices"

	"github.com/seamanticus/amitty/internal/sound"
)

// SoundName is the name of a sound amitty ships, such as "chime".
type SoundName string

func (s *SoundName) UnmarshalText(text []byte) error {
	if !slices.Contains(sound.Names(), string(text)) {
		return fmt.Errorf("unknown sound: %q", text)
	}
	*s = SoundName(text)
	return nil
}

func (s SoundName) MarshalText() ([]byte, error) { return []byte(s), nil }

func (SoundName) choices() []string { return sound.Names() }
