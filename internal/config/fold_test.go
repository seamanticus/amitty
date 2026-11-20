package config

import (
	"math"
	"strconv"
	"testing"
)

func TestFoldWidth(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		text string
		want FoldWidth
		ok   bool
	}{
		{"120", FoldWidth{size: 120}, true},
		{"0", FoldWidth{}, true},
		{"1200px", FoldWidth{size: 1200, pixels: true}, true},
		{" 80 ", FoldWidth{size: 80}, true},
		{"", FoldWidth{}, false},
		{"px", FoldWidth{}, false},
		{"-1", FoldWidth{}, false},
		{"12em", FoldWidth{}, false},
		{"1200 px", FoldWidth{}, false},
		{strconv.Itoa(math.MaxInt32), FoldWidth{size: math.MaxInt32}, true},
		{strconv.Itoa(math.MaxInt32 + 1), FoldWidth{}, false},
	} {
		got, err := ParseFoldWidth(test.text)
		if got != test.want || (err == nil) != test.ok {
			t.Errorf("ParseFoldWidth(%q) = %+v, %v; want %+v, ok %v", test.text, got, err, test.want, test.ok)
		}
		if !test.ok {
			continue
		}
		// The written form reads back as the same width.
		text, err := test.want.MarshalTOML()
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := Parse([]byte("[members_tree]\nfold_below = " + string(text)))
		if err != nil || cfg.MembersTree.FoldBelow != test.want {
			t.Errorf("%s read back as %+v, %v", text, cfg, err)
		}
	}

	// evidence: witnesses
	for _, test := range []struct {
		width        FoldWidth
		cols, pixels int
		want         bool
	}{
		{FoldWidth{size: 120}, 119, 0, true},
		{FoldWidth{size: 120}, 120, 0, false},
		{FoldWidth{size: 120}, 0, 0, false},
		{FoldWidth{size: 1200, pixels: true}, 200, 1199, true},
		{FoldWidth{size: 1200, pixels: true}, 10, 1200, false},
		{FoldWidth{size: 1200, pixels: true}, 10, 0, false},
		{FoldWidth{}, 1, 1, false},
	} {
		if got := test.width.Narrower(test.cols, test.pixels); got != test.want {
			t.Errorf("%+v.Narrower(%d, %d) = %v, want %v", test.width, test.cols, test.pixels, got, test.want)
		}
	}
}
