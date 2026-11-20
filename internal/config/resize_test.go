package config

import (
	"math"
	"strconv"
	"testing"
)

func TestTerminalWidth(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		text string
		want TerminalWidth
		ok   bool
	}{
		{"120", TerminalWidth{size: 120}, true},
		{"0", TerminalWidth{}, true},
		{"1200px", TerminalWidth{size: 1200, pixels: true}, true},
		{" 80 ", TerminalWidth{size: 80}, true},
		{"", TerminalWidth{}, false},
		{"px", TerminalWidth{}, false},
		{"-1", TerminalWidth{}, false},
		{"12em", TerminalWidth{}, false},
		{"1200 px", TerminalWidth{}, false},
		{strconv.Itoa(math.MaxInt32), TerminalWidth{size: math.MaxInt32}, true},
		{strconv.Itoa(math.MaxInt32 + 1), TerminalWidth{}, false},
	} {
		got, err := ParseTerminalWidth(test.text)
		if got != test.want || (err == nil) != test.ok {
			t.Errorf("ParseTerminalWidth(%q) = %+v, %v; want %+v, ok %v", test.text, got, err, test.want, test.ok)
		}
		if !test.ok {
			continue
		}
		// The written form reads back as the same width.
		text, err := test.want.MarshalTOML()
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := Parse([]byte("[ui.resize]\nis_less_than = " + string(text)))
		if err != nil || cfg.UI.Resize.IsLessThan != test.want {
			t.Errorf("%s read back as %+v, %v", text, cfg, err)
		}
	}

	// evidence: witnesses
	for _, test := range []struct {
		width        TerminalWidth
		cols, pixels int
		want         bool
	}{
		{TerminalWidth{size: 120}, 119, 0, true},
		{TerminalWidth{size: 120}, 120, 0, false},
		{TerminalWidth{size: 120}, 0, 0, false},
		{TerminalWidth{size: 1200, pixels: true}, 200, 1199, true},
		{TerminalWidth{size: 1200, pixels: true}, 10, 1200, false},
		{TerminalWidth{size: 1200, pixels: true}, 10, 0, false},
		{TerminalWidth{}, 1, 1, false},
	} {
		if got := test.width.Narrower(test.cols, test.pixels); got != test.want {
			t.Errorf("%+v.Narrower(%d, %d) = %v, want %v", test.width, test.cols, test.pixels, got, test.want)
		}
	}
}
