package config

import (
	"fmt"

	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/sound"
)

// SoundName is the file name of a sound in the sounds dir, such as "chime.wav": one amitty ships or one of the user's own.
type SoundName string

func (s *SoundName) UnmarshalText(text []byte) error {
	if !sound.IsName(string(text)) {
		return fmt.Errorf("not a file name in the sounds dir: %q", text)
	}
	*s = SoundName(text)
	return nil
}

func (s SoundName) MarshalText() ([]byte, error) { return []byte(s), nil }

// choices lists the sounds dir as it is now, so files dropped in it are offered too.
func (SoundName) choices() []string {
	dataDir, err := consts.DataDir()
	if err != nil {
		return sound.Shipped()
	}
	return sound.Names(sound.Dir(dataDir))
}
