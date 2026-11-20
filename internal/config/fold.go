package config

import (
	"fmt"
	"strconv"
	"strings"
)

// FoldMode is when the members tree is folded under the guilds tree.
type FoldMode uint8

const (
	FoldOff FoldMode = iota
	FoldOn
	// FoldAuto folds while the terminal is narrower than fold_below.
	FoldAuto
	foldModeCount
)

var foldModes = enum[FoldMode]{{"off", FoldOff}, {"on", FoldOn}, {"auto", FoldAuto}}

func (f *FoldMode) UnmarshalText(text []byte) (err error) {
	*f, err = foldModes.parse(string(text))
	return err
}

func (f FoldMode) MarshalText() ([]byte, error) { return foldModes.text(f) }

func (FoldMode) choices() []string { return foldModes.names() }

// FoldWidth is a terminal width, in columns, or in pixels when written with "px". The zero value is 0 columns, which no terminal is narrower than.
type FoldWidth struct {
	size   int
	pixels bool
}

// ParseFoldWidth reads a width such as "120" (columns) or "1200px".
func ParseFoldWidth(text string) (FoldWidth, error) {
	digits, pixels := strings.CutSuffix(strings.TrimSpace(text), "px")
	size, err := strconv.ParseUint(digits, 10, 31)
	if err != nil {
		return FoldWidth{}, fmt.Errorf("%q: want columns such as 120 or pixels such as \"1200px\"", text)
	}
	return FoldWidth{size: int(size), pixels: pixels}, nil
}

func (w *FoldWidth) UnmarshalTOML(value any) error {
	var text string
	switch value := value.(type) {
	case int64:
		text = strconv.FormatInt(value, 10)
	case string:
		text = value
	default:
		return fmt.Errorf("%v: want columns such as 120 or pixels such as \"1200px\"", value)
	}
	width, err := ParseFoldWidth(text)
	if err != nil {
		return err
	}
	*w = width
	return nil
}

// MarshalTOML writes columns as a number and pixels as a string, as they are read.
func (w FoldWidth) MarshalTOML() ([]byte, error) {
	if w.pixels {
		return []byte(strconv.Quote(strconv.Itoa(w.size) + "px")), nil
	}
	return []byte(strconv.Itoa(w.size)), nil
}

// Narrower reports whether a terminal cols wide and pixels wide, each 0 if unknown, is narrower than w. An unknown width is never narrower, as some terminals do not report theirs in pixels.
func (w FoldWidth) Narrower(cols, pixels int) bool {
	if w.pixels {
		return pixels > 0 && pixels < w.size
	}
	return cols > 0 && cols < w.size
}
