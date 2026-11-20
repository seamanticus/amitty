package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestProfiles(t *testing.T) {
	names := ProfileNames()
	// proof: exhaustive over the profiles compiled in.
	t.Run("every profile loads with unique names", func(t *testing.T) {
		seen := map[string]bool{}
		for _, name := range names {
			if seen[name] {
				t.Fatalf("%q twice", name)
			}
			seen[name] = true
			var cfg Config
			if err := cfg.applyProfile(name); err != nil {
				t.Fatal(err)
			}
			// The selection must stand out from the background it is mixed from.
			p, _ := profilePalette(name)
			if r := rolesOf(p); r.selection == p.bg || contrast(r.muted, p.bg) < 3 {
				t.Fatalf("%q: selection %06x on %06x, muted contrast %.2f", name, r.selection, p.bg, contrast(r.muted, p.bg))
			}
		}
		if len(names) != 98 {
			t.Fatalf("%d profiles, want maya's 98", len(names))
		}
	})
}

func TestMix(t *testing.T) {
	// evidence: witnesses
	for _, test := range []struct {
		a, b uint32
		t    float64
		want uint32
	}{
		{0x000000, 0xffffff, 0, 0x000000},
		{0x000000, 0xffffff, 1, 0xffffff},
		{0x000000, 0xffffff, 0.5, 0x808080},
		{0x102030, 0x102030, 0.3, 0x102030},
	} {
		if got := mix(test.a, test.b, test.t); got != test.want {
			t.Errorf("mix(%06x, %06x, %v) = %06x, want %06x", test.a, test.b, test.t, got, test.want)
		}
	}
}

func TestLoadProfile(t *testing.T) {
	load := func(t *testing.T, content string) (*Config, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return Load(path)
	}
	nord, _ := profilePalette("Nord")

	t.Run("colors the theme", func(t *testing.T) {
		cfg, err := load(t, `theme_profile = "Nord"`)
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.Theme.MessagesList.MentionStyle.GetForeground(); got != rgb(nord.colors[4]) {
			t.Fatalf("mention color = %v, want Nord's blue", got)
		}
		if got := cfg.Theme.MessagesList.AlternateStyle.GetBackground(); got != rgb(mix(nord.bg, nord.fg, 0.06)) {
			t.Fatalf("alternate background = %v, want Nord's surface", got)
		}
	})
	t.Run("styles set in the file win", func(t *testing.T) {
		cfg, err := load(t, "theme_profile = \"Nord\"\n[theme.messages_list]\nmention_style = { foreground = \"red\" }\n")
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.Theme.MessagesList.MentionStyle.GetForeground(); got != tcell.ColorRed {
			t.Fatalf("mention color = %v, want red", got)
		}
		if got := cfg.Theme.MessagesList.URLStyle.GetForeground(); got != rgb(nord.colors[6]) {
			t.Fatalf("url color = %v, want Nord's cyan", got)
		}
	})
	t.Run("a file naming none takes TokyoNight", func(t *testing.T) {
		cfg, err := load(t, "")
		if err != nil {
			t.Fatal(err)
		}
		tokyo, _ := profilePalette("TokyoNight")
		if cfg.ThemeProfile != "TokyoNight" || cfg.Theme.MessagesList.MentionStyle.GetForeground() != rgb(tokyo.colors[4]) {
			t.Fatalf("profile %q, mention color %v", cfg.ThemeProfile, cfg.Theme.MessagesList.MentionStyle.GetForeground())
		}
	})
	t.Run("none keeps the terminal's colors and no alternate tint", func(t *testing.T) {
		cfg, err := load(t, `theme_profile = ""`)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Theme.MessagesList.AlternateStyle.Style != tcell.StyleDefault || cfg.BaseStyle() != tcell.StyleDefault {
			t.Fatal("colors without a profile")
		}
	})
	t.Run("an unknown profile fails", func(t *testing.T) {
		if _, err := load(t, `theme_profile = "Nope"`); err == nil {
			t.Fatal("no error")
		}
	})
}

func TestBaseStyle(t *testing.T) {
	nord, _ := profilePalette("Nord")
	// evidence: witnesses
	for _, test := range []struct {
		profile string
		inherit bool
		want    tcell.Style
	}{
		{"", false, tcell.StyleDefault},
		{"Nord", false, tcell.StyleDefault.Foreground(rgb(nord.fg)).Background(rgb(nord.bg))},
		{"Nord", true, tcell.StyleDefault.Foreground(rgb(nord.fg))},
	} {
		cfg := Config{ThemeProfile: test.profile, InheritBackground: test.inherit}
		if got := cfg.BaseStyle(); got != test.want {
			t.Errorf("%q inherit %v = %v, want %v", test.profile, test.inherit, got, test.want)
		}
	}
}
