package config

import (
	"strings"
	"testing"
)

func TestOptions(t *testing.T) {
	cfg := Default()
	options := Options(&cfg)
	byPath := map[string]Option{}
	for _, option := range options {
		if _, ok := byPath[option.Path]; ok {
			t.Fatalf("%s twice", option.Path)
		}
		byPath[option.Path] = option
	}
	// Every option the commented default file sets is listed, with the same value.
	for line := range strings.Lines(string(defaultFile)) {
		if !strings.HasPrefix(line, "# ") || !isOption(line[2:]) {
			continue
		}
		key, value, _ := strings.Cut(strings.TrimSpace(line[2:]), " = ")
		found := false
		for path, option := range byPath {
			if (path == key || strings.HasSuffix(path, "."+key)) && option.Value == value {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s = %s is not an option", key, value)
		}
	}

	// evidence: witnesses
	for _, test := range []struct {
		path    string
		kind    OptionKind
		restart bool
	}{
		{"mouse", OptionBool, true},
		{"proxy", OptionText, true},
		{"theme_profile", OptionChoice, false},
		{"openers", OptionFileOnly, false},
		{"keybinds.quit", OptionText, false},
		{"theme.messages_list.reply_style", OptionText, false},
	} {
		option, ok := byPath[test.path]
		if !ok || option.Kind != test.kind || option.Restart != test.restart {
			t.Errorf("%s = %+v, want kind %d, restart %v", test.path, option, test.kind, test.restart)
		}
	}
	if doc := byPath["mouse"].Doc; doc != "Whether to enable the mouse." {
		t.Errorf("mouse doc = %q", doc)
	}
}

func TestSetOption(t *testing.T) {
	const file = "# mine\nmouse = true\n\n[markdown]\n# keep\ntheme = \"monokai\"\n\n[keybinds]\nquit = [\n  \"ctrl+c\",\n  \"ctrl+q\",\n]\n"
	// evidence: witnesses
	for _, test := range []struct {
		name, path, value, want string
	}{
		{"its own line", "mouse", "false", "# mine\nmouse = false\n\n[markdown]\n# keep\ntheme = \"monokai\"\n\n[keybinds]\nquit = [\n  \"ctrl+c\",\n  \"ctrl+q\",\n]\n"},
		{"a value over lines", "keybinds.quit", `"ctrl+q"`, "# mine\nmouse = true\n\n[markdown]\n# keep\ntheme = \"monokai\"\n\n[keybinds]\nquit = \"ctrl+q\"\n"},
		{"in its table", "markdown.enabled", "false", "# mine\nmouse = true\n\n[markdown]\nenabled = false\n# keep\ntheme = \"monokai\"\n\n[keybinds]\nquit = [\n  \"ctrl+c\",\n  \"ctrl+q\",\n]\n"},
		{"top level before the tables", "editor", `"vi"`, "# mine\nmouse = true\n\neditor = \"vi\"\n[markdown]\n# keep\ntheme = \"monokai\"\n\n[keybinds]\nquit = [\n  \"ctrl+c\",\n  \"ctrl+q\",\n]\n"},
		{"a new table", "ui.messages_list.alternate_every", "3", file + "\n[ui.messages_list]\nalternate_every = 3\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := SetOption([]byte(file), test.path, test.value)
			if err != nil || string(got) != test.want {
				t.Fatalf("SetOption = %q, %v\nwant %q", got, err, test.want)
			}
		})
	}

	t.Run("uncomments the commented default", func(t *testing.T) {
		got, err := SetOption(defaultFile, "theme_profile", `"Nord"`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "\ntheme_profile = \"Nord\"\n") || strings.Contains(string(got), "# theme_profile = ") {
			t.Fatal("the commented default was not replaced")
		}
		if strings.Count(string(got), "\n") != strings.Count(string(defaultFile), "\n") {
			t.Fatal("lines were added or lost")
		}
	})

	for _, test := range []struct{ name, path, value string }{
		{"not TOML", "mouse", "yes please"},
		{"the wrong type", "mouse", `"x"`},
		{"an unknown profile", "theme_profile", `"Nope"`},
		{"no such option", "nope", "1"},
	} {
		t.Run("rejects "+test.name, func(t *testing.T) {
			if _, err := SetOption([]byte(file), test.path, test.value); err == nil {
				t.Fatal("no error")
			}
		})
	}
}
