package ui

import "testing"

func TestXDGOpenError(t *testing.T) {
	for code := range 6 {
		_, ok := xdgOpenError(code)
		if want := code >= 1 && code <= 4; ok != want {
			t.Errorf("xdgOpenError(%d) ok = %v, want %v", code, ok, want)
		}
	}
}
