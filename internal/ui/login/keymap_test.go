package login

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ayn2op/tview/keybind"
	"github.com/seamanticus/amitty/internal/config"
	"github.com/seamanticus/amitty/internal/http"
)

func TestModelShortHelp(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel(cfg, http.Build{})
	for _, tt := range []struct {
		name               string
		active             tab
		wantPrev, wantNext bool
	}{
		{"first tab", tabPassword, false, true},
		{"middle tab", tabQR, true, true},
		{"last tab", tabToken, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m.active = tt.active
			short := m.ShortHelp()
			has := func(desc string) bool {
				return slices.ContainsFunc(short, func(k keybind.Keybind) bool { return k.Help().Desc == desc })
			}
			if has("prev tab") != tt.wantPrev || has("next tab") != tt.wantNext {
				t.Fatalf("prev = %v, next = %v", has("prev tab"), has("next tab"))
			}
			// The tab's own keybinds follow, so its keys are listed.
			if own := m.activeKeyMap().ShortHelp(); len(own) == 0 || !slices.ContainsFunc(short, func(k keybind.Keybind) bool { return k.Help() == own[0].Help() }) {
				t.Fatalf("help %v lacks the tab's own %v", short, own)
			}
		})
	}
}

// evidence: witnesses
func TestModelSelectTab(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{-1, int(tabCount), int(tabCount) + 1} {
		m, _ := NewModel(cfg, http.Build{}).Update(selectTabMsg(i))
		if m.active != tabPassword {
			t.Fatalf("index %d selected tab %d", i, m.active)
		}
	}
	for want := range tabCount {
		if m, _ := NewModel(cfg, http.Build{}).Update(selectTabMsg(want)); m.active != want {
			t.Fatalf("index %d selected tab %d", want, m.active)
		}
	}
}
