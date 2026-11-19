package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/seamanticus/amitty/internal/consts"
)

func TestDefaultPath(t *testing.T) {
	t.Run("user config dir fallback", func(t *testing.T) {
		t.Setenv("AppData", "")
		t.Setenv("HOME", "")
		t.Setenv("home", "")
		t.Setenv("XDG_CONFIG_HOME", "")

		// filepath.Join strips the leading dot.
		got := DefaultPath()
		want := filepath.Join(".", consts.Name, fileName)
		if got != want {
			t.Fatalf("got = %v, want = %v", got, want)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("invalid config returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.toml")
		if err := os.WriteFile(path, []byte("invalid ="), os.ModePerm); err != nil {
			t.Fatal(err)
		}

		if _, err := Load(path); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid config does not return error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "good.toml")
		if err := os.WriteFile(path, []byte("mouse = false\n[help]\nenabled = true"), os.ModePerm); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}

		if cfg.Mouse != false {
			t.Fatalf("got = %v, want = false", cfg.Mouse)
		}
		if !cfg.Help.Enabled {
			t.Fatal("help enabled = false, want true")
		}
	})

	t.Run("invalid allowed MIME type returns error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad-mime.toml")
		if err := os.WriteFile(path, []byte("[ui.messages_list.attachments]\nallowed_mime_types = [\"image/\"]"), os.ModePerm); err != nil {
			t.Fatal(err)
		}

		if _, err := Load(path); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("open with bad path returns error (!= ErrNotExist)", func(t *testing.T) {
		if _, err := Load("bad\x00path"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing file uses defaults", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.toml")
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}

		defCfg := Default()
		defCfg.applyDefaults()

		if !reflect.DeepEqual(defCfg, *cfg) {
			t.Fatalf("got = %+v, want = %+v", *cfg, defCfg)
		}
	})
}

// proof: exhaustive
func TestMessagesLimit(t *testing.T) {
	// Every value TOML can give a uint8, so no fetch takes a whole history or several pages.
	for limit := range 256 {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(fmt.Sprintf("messages_limit = %d", limit)), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		want := uint8(limit)
		if limit < 1 || limit > 100 {
			want = 50
		}
		if cfg.MessagesLimit != want {
			t.Fatalf("messages_limit = %d loads as %d, want %d", limit, cfg.MessagesLimit, want)
		}
	}
}

// The generated docs show defaults as they are marshaled, so those must parse back to the same defaults.
func TestDefault(t *testing.T) {
	t.Run("round trips through TOML", func(t *testing.T) {
		data, err := toml.Marshal(Default())
		if err != nil {
			t.Fatal(err)
		}
		// Help descriptions are not part of the TOML.
		got := Config{Keybinds: defaultKeybinds()}
		if err := toml.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if want := Default(); !reflect.DeepEqual(got, want) {
			t.Fatalf("got = %+v, want = %+v", got, want)
		}
	})
	t.Run("the written file sets every option when uncommented", func(t *testing.T) {
		var uncommented []byte
		for line := range strings.Lines(string(defaultFile)) {
			// Options are "# key = value"; doc comments have no " = " after a bare key.
			if option, ok := strings.CutPrefix(line, "# "); ok && isOption(option) {
				line = option
			}
			uncommented = append(uncommented, line...)
		}
		got := Config{Keybinds: defaultKeybinds()}
		meta, err := toml.Decode(string(uncommented), &got)
		if err != nil {
			t.Fatal(err)
		}
		if undecoded := meta.Undecoded(); len(undecoded) > 0 {
			t.Fatalf("keys not in the config: %v", undecoded)
		}
		if want := Default(); !reflect.DeepEqual(got, want) {
			t.Fatalf("got = %+v, want = %+v", got, want)
		}
	})
}

// isOption reports whether a commented line is a key and a value.
func isOption(line string) bool {
	key, _, ok := strings.Cut(line, " = ")
	return ok && key != "" && !strings.ContainsAny(key, " \"`")
}

func TestWriteDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "amitty", fileName)
	if wrote, err := WriteDefault(path); err != nil || !wrote {
		t.Fatalf("first run = %v, %v, want the file written", wrote, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(defaultFile) {
		t.Fatalf("file = %q, %v, want the default file", data, err)
	}
	if err := os.WriteFile(path, []byte("mouse = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if wrote, err := WriteDefault(path); err != nil || wrote {
		t.Fatalf("second run = %v, %v, want the file kept", wrote, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "mouse = false\n" {
		t.Fatalf("file = %q, want the user's kept", data)
	}
}

func TestMIMETypesHas(t *testing.T) {
	tests := []struct {
		name        string
		allowed     MIMETypes
		mediaType   string
		wantAllowed bool
	}{
		{"exact", []string{"application/pdf"}, "application/pdf", true},
		{"exact mismatch", []string{"application/pdf"}, "application/zip", false},
		{"subtype wildcard", []string{"image/*"}, "image/png", true},
		{"wildcard mismatch", []string{"image/*"}, "video/mp4", false},
		{"all wildcard", []string{"*/*"}, "video/mp4", true},
		{"case and parameters", []string{"image/*"}, "Image/PNG; charset=utf-8", true},
		{"invalid content type", []string{"*/*"}, "not-a-mime-type", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.allowed.Has(tt.mediaType); got != tt.wantAllowed {
				t.Fatalf("Has(%q) = %v, want %v", tt.mediaType, got, tt.wantAllowed)
			}
		})
	}
}
