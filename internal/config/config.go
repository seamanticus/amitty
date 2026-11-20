package config

import (
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/seamanticus/amitty/internal/consts"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
)

const fileName = "config.toml"

// defaultFile is every option at its default, commented out under its doc comment.
//
//go:embed default.toml
var defaultFile []byte

type MIMETypes []string

func (types *MIMETypes) UnmarshalTOML(value any) error {
	values, ok := value.([]any)
	if !ok {
		return errInvalidType
	}

	parsed := make(MIMETypes, 0, len(values))
	for _, value := range values {
		mediaType, ok := value.(string)
		if !ok {
			return errInvalidType
		}
		mediaType, _, err := mime.ParseMediaType(mediaType)
		if err != nil {
			return err
		}
		parsed = append(parsed, mediaType)
	}

	*types = parsed
	return nil
}

func (types MIMETypes) Has(mediaType string) bool {
	mediaType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(types, func(pattern string) bool {
		matches, _ := path.Match(pattern, mediaType)
		return matches
	})
}

// OpenerURL is the key of the opener for links.
const OpenerURL = "url"

// Opener is a command that opens a file or link.
type Opener struct {
	Command string `toml:"command"`
	// Whether the command runs in the terminal, suspending amitty, rather than in the background.
	Terminal bool `toml:"terminal"`
}

// Openers maps a MIME type, a wildcard such as "video/*", or OpenerURL to its opener.
type Openers map[string]Opener

// For returns the opener of key, a MIME type or OpenerURL: the one set for it exactly, else for its wildcard, and whether one is set.
func (o Openers) For(key string) (Opener, bool) {
	if key != OpenerURL {
		if mediaType, _, err := mime.ParseMediaType(key); err == nil {
			key = mediaType
		}
	}
	if opener, ok := o[key]; ok && opener.Command != "" {
		return opener, true
	}
	if major, _, ok := strings.Cut(key, "/"); ok {
		if opener, ok := o[major+"/*"]; ok && opener.Command != "" {
			return opener, true
		}
	}
	return Opener{}, false
}

type (
	Timestamps struct {
		Enabled bool `toml:"enabled"`
		// https://pkg.go.dev/time#Layout
		Format string `toml:"format"`
	}

	DateSeparator struct {
		Enabled bool `toml:"enabled"`
		// https://pkg.go.dev/time#Layout
		Format string `toml:"format"`
		// The fill character used on both sides of the date label.
		Character string `toml:"character"`
	}

	Notifications struct {
		Enabled bool `toml:"enabled"`
		// Whether to also notify about messages in the open channel while the terminal window is unfocused.
		// Requires a terminal that reports focus.
		WhenUnfocused bool `toml:"when_unfocused"`
	}

	TypingIndicator struct {
		// Whether to send typing status.
		Send bool `toml:"send"`
		// Whether to receive typing status.
		Receive bool `toml:"receive"`
	}

	Icons struct {
		GuildCategory   string `toml:"guild_category"`
		GuildText       string `toml:"guild_text"`
		GuildVoice      string `toml:"guild_voice"`
		GuildStageVoice string `toml:"guild_stage_voice"`

		GuildAnnouncementThread string `toml:"guild_announcement_thread"`
		GuildPublicThread       string `toml:"guild_public_thread"`
		GuildPrivateThread      string `toml:"guild_private_thread"`

		GuildAnnouncement string `toml:"guild_announcement"`
		GuildForum        string `toml:"guild_forum"`
		GuildStore        string `toml:"guild_store"`
	}

	PickerConfig struct {
		Width  int `toml:"width"`
		Height int `toml:"height"`
	}

	MarkdownConfig struct {
		// Whether to parse and render markdown in messages.
		Enabled bool `toml:"enabled"`
		// Replace spoiler contents with a placeholder.
		MaskSpoilers bool `toml:"mask_spoilers"`
		// Theme for fenced code blocks. Available themes: https://xyproto.github.io/splash/docs
		Theme string `toml:"theme"`
		// Guess the language of a fenced code block that names none, to highlight it.
		DetectCodeLanguage bool `toml:"detect_code_language"`
	}

	HelpConfig struct {
		// Whether to show keybind help.
		Enabled bool `toml:"enabled"`
		// Show compact key modifiers in help, e.g. "^x" instead of "ctrl+x".
		CompactModifiers bool `toml:"compact_modifiers"`
		// [left, right]
		Padding   [2]int `toml:"padding"`
		Separator string `toml:"separator"`
	}

	ComposerConfig struct {
		// Maximum height (in newline-separated rows) the composer grows to before it starts scrolling internally.
		// Set to 1 for a fixed single-line input. Values below 1 fall back to 10.
		MaxHeight int `toml:"max_height"`
	}

	SidebarMarkersConfig struct {
		Expanded  string `toml:"expanded"`
		Collapsed string `toml:"collapsed"`
		Leaf      string `toml:"leaf"`
	}

	SidebarIndentsConfig struct {
		Guild    int `toml:"guild"`
		Category int `toml:"category"`
		Channel  int `toml:"channel"`
		Forum    int `toml:"forum"`
		GroupDM  int `toml:"group_dm"`
		DM       int `toml:"dm"`
	}

	// PaneConfig is a side pane that can be toggled at runtime.
	PaneConfig struct {
		// Whether the pane is visible at start-up. It can still be toggled at runtime with its toggle keybind.
		Visible bool `toml:"visible"`
		// Percentage (%) of the available width used by the pane.
		// Valid values are 1-99. Invalid values fall back to 20.
		WidthPercent int `toml:"width_percent"`
	}

	SidebarConfig struct {
		PaneConfig
		Markers SidebarMarkersConfig `toml:"markers"`
		// Indentation for different item types.
		Indents SidebarIndentsConfig `toml:"indents"`
		// The sidebar while the layout is resized, the terminal being narrower than ui.resize.is_less_than.
		Resize SidebarResizeConfig `toml:"resize"`
	}

	SidebarResizeConfig struct {
		// Percentage (%) of the available width the sidebar takes while resized, in place of width_percent, with or without the members tree folded in. Valid values are 1-99. Invalid values fall back to 35.
		WidthPercent int `toml:"width_percent"`
	}

	MembersTreeConfig struct {
		PaneConfig
		// Whether the members tree is folded under the guilds tree, in the sidebar: "off", "on", or "auto" while the layout is resized (see ui.resize.is_less_than).
		Fold FoldMode `toml:"fold"`
		// Percentage (%) of the folded column's height the members tree takes, below the guilds tree. Valid values are 1-99. Invalid values fall back to 50.
		FoldHeightPercent int `toml:"fold_height_percent"`
	}

	Config struct {
		// Whether to focus the composer automatically when a channel is selected.
		// Set to false to preview channels without moving focus.
		AutoFocus bool `toml:"auto_focus"`
		// Whether to enable the mouse.
		Mouse bool `toml:"mouse"`
		// The program opened by the `composer.open_editor` keybind. "default" uses `$EDITOR`.
		Editor string `toml:"editor"`
		// The program the `composer.open_file_picker` keybind runs in place of amitty's path prompt, such as "fzf --multi" or "yazi --chooser-file={file}"; "" uses the prompt.
		// It runs in the terminal while amitty is suspended. It writes the chosen paths, one per line, to stdout, or to the file named where `{file}` is written.
		FilePicker string `toml:"file_picker"`
		// Programs that open attachments by MIME type, and links by the key "url", in place of `xdg-open`, such as
		// `{ "video/*" = { command = "mpv {}" }, "text/plain" = { command = "nvim {}", terminal = true }, url = { command = "firefox {}" } }`.
		// `{}` is replaced by the file or link, which is appended when there is no `{}`. An exact type wins over a wildcard.
		// A command runs in the background unless `terminal = true`, which suspends amitty while it runs.
		Openers Openers `toml:"openers"`

		// The command that encrypts the token: it reads the token on stdin and writes what is stored in `$XDG_STATE_HOME/amitty/token.gpg` to stdout.
		TokenEncryptCmd string `toml:"token-encrypt-cmd"`
		// The command that decrypts the stored token: it reads the file on stdin and writes the token to stdout. It runs before the screen starts, so a terminal pinentry works.
		TokenDecryptCmd string `toml:"token-decrypt-cmd"`

		// The proxy every connection to Discord goes through, such as "socks5h://127.0.0.1:9050" or "http://user:pass@host:3128"; "" connects directly. Schemes: http, https, socks5, socks5h (both SOCKS5 schemes resolve names at the proxy). The environment is not read, and nothing bypasses a set proxy.
		Proxy string `toml:"proxy"`

		// "default" (unknown), "online", "dnd", "idle", "invisible" or "offline".
		Status           discord.Status `toml:"status"`
		HideBlockedUsers bool           `toml:"hide_blocked_users"`
		// Whether to leave DMs with deleted accounts out of the guilds tree's Direct Messages and the channels picker.
		HideDeletedUsers bool `toml:"hide_deleted_users"`

		// The number of messages to fetch when a text-based channel is selected, from 1 to 100.
		MessagesLimit uint8 `toml:"messages_limit"`

		Markdown        MarkdownConfig  `toml:"markdown"`
		Help            HelpConfig      `toml:"help"`
		Picker          PickerConfig    `toml:"picker"`
		Timestamps      Timestamps      `toml:"timestamps"`
		DateSeparator   DateSeparator   `toml:"date_separator"`
		Notifications   Notifications   `toml:"notifications"`
		TypingIndicator TypingIndicator `toml:"typing_indicator"`
		// The guilds tree pane.
		Sidebar SidebarConfig `toml:"sidebar"`
		// The members tree pane, sized from the width right of the guilds tree.
		MembersTree MembersTreeConfig `toml:"members_tree"`
		Composer    ComposerConfig    `toml:"composer"`

		// Layout and behavior of the widgets.
		UI    UIConfig `toml:"ui"`
		Icons Icons    `toml:"icons"`

		// Global shortcuts. A keybind is a key such as "ctrl+k", or a list of keys.
		Keybinds Keybinds `toml:"keybinds"`
		// A color scheme every style of [theme] starts from, such as "Catppuccin Mocha"; "" keeps the terminal's own colors. Styles set under [theme] override it.
		// The profiles are listed in docs/themes.md. The profile paints its background and text color everywhere.
		ThemeProfile string `toml:"theme_profile"`
		// Whether a theme profile leaves the background to the terminal, keeping its transparency, and paints only text.
		InheritBackground bool `toml:"inherit_background"`
		// Styles of the widgets, each written as
		// `{ foreground = "", background = "", attributes = "" or ["", ""], underline = "", underline_color = "" }`.
		Theme ThemeConfig `toml:"theme"`
	}
)

//go:generate go run generator.go

// Default returns the default configuration.
func Default() Config {
	pane := PaneConfig{Visible: true, WidthPercent: 20}
	return Config{
		AutoFocus: true,
		Mouse:     true,
		Editor:    "default",
		Openers:   Openers{},

		TokenEncryptCmd: "gpg --encrypt --default-recipient-self",
		TokenDecryptCmd: "gpg --decrypt --quiet",

		Status:           "default",
		HideBlockedUsers: true,
		HideDeletedUsers: true,

		MessagesLimit: 50,

		Markdown:        MarkdownConfig{Enabled: true, Theme: "monokai"},
		Help:            HelpConfig{Enabled: true, CompactModifiers: true, Padding: [2]int{1, 1}, Separator: " • "},
		Picker:          PickerConfig{Width: 80, Height: 25},
		Timestamps:      Timestamps{Enabled: true, Format: "3:04PM"},
		DateSeparator:   DateSeparator{Enabled: true, Format: "January 2, 2006", Character: "─"},
		Notifications:   Notifications{Enabled: true, WhenUnfocused: true},
		TypingIndicator: TypingIndicator{Send: true, Receive: true},
		Sidebar: SidebarConfig{
			PaneConfig: pane,
			Markers:    SidebarMarkersConfig{Expanded: "▾ ", Collapsed: "▸ "},
			Indents:    SidebarIndentsConfig{Guild: 2, Category: 1, Channel: 2, Forum: 2, GroupDM: 1, DM: 2},
			Resize:     SidebarResizeConfig{WidthPercent: 35},
		},
		MembersTree: MembersTreeConfig{PaneConfig: pane, Fold: FoldAuto, FoldHeightPercent: 50},
		Composer:    ComposerConfig{MaxHeight: 10},

		UI: defaultUI(),
		Icons: Icons{
			GuildText:       "#",
			GuildVoice:      "♪ ",
			GuildStageVoice: "♪ ",

			GuildAnnouncementThread: "a-",
			GuildPublicThread:       "› ",
			GuildPrivateThread:      "› ",

			GuildAnnouncement: "a-",
			GuildForum:        "≡ ",
			GuildStore:        "s-",
		},

		Keybinds: defaultKeybinds(),
		Theme:    defaultTheme(),
	}
}

var dir = sync.OnceValue(func() string {
	path, err := os.UserConfigDir()
	if err != nil {
		slog.Info("user config dir cannot be determined; falling back to the current dir", "err", err)
		path = "."
	}
	return filepath.Join(path, consts.Name)
})

func DefaultPath() string {
	return filepath.Join(dir(), fileName)
}

// WriteDefault writes the commented default configuration to path, creating its directory, unless a file is already there. It reports whether it wrote one.
func WriteDefault(path string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, fmt.Errorf("create config dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create config: %w", err)
	}
	if _, err := file.Write(defaultFile); err != nil {
		_ = file.Close()
		return false, fmt.Errorf("write config: %w", err)
	}
	return true, file.Close()
}

// Load reads the configuration file and parses it.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		slog.Info("config file does not exist, falling back to the default config", "path", path, "err", err)
	case err != nil:
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	return Parse(data)
}

// Parse returns the configuration that the contents of a config file give.
func Parse(data []byte) (*Config, error) {
	cfg, err := decode(data)
	if err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return &cfg, nil
}

// decode returns data decoded over the defaults, or over a profile's theme if it names one, before applyDefaults fixes values up.
func decode(data []byte) (Config, error) {
	cfg := Default()
	var profile struct {
		ThemeProfile string `toml:"theme_profile"`
	}
	if _, err := toml.Decode(string(data), &profile); err != nil {
		return Config{}, fmt.Errorf("failed to decode config: %w", err)
	}
	if err := cfg.applyProfile(profile.ThemeProfile); err != nil {
		return Config{}, err
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to decode config: %w", err)
	}
	return cfg, nil
}

func (cfg *Config) applyDefaults() {
	if cfg.Editor == "default" {
		cfg.Editor = os.Getenv("EDITOR")
	}

	if cfg.Status == "default" {
		cfg.Status = discord.UnknownStatus
	}

	// 0 would fetch a channel's whole history, and more than 100 several pages per fetch.
	if cfg.MessagesLimit < 1 || cfg.MessagesLimit > 100 {
		cfg.MessagesLimit = 50
	}

	if cfg.Composer.MaxHeight <= 0 {
		cfg.Composer.MaxHeight = 10
	}

	for _, pane := range []*PaneConfig{&cfg.Sidebar.PaneConfig, &cfg.MembersTree.PaneConfig} {
		if pane.WidthPercent <= 0 || pane.WidthPercent >= 100 {
			pane.WidthPercent = 20
		}
	}
	if tree := &cfg.MembersTree; tree.FoldHeightPercent <= 0 || tree.FoldHeightPercent >= 100 {
		tree.FoldHeightPercent = 50
	}
	if resize := &cfg.Sidebar.Resize; resize.WidthPercent <= 0 || resize.WidthPercent >= 100 {
		resize.WidthPercent = 35
	}

	if cfg.DateSeparator.Format == "" {
		cfg.DateSeparator.Format = "January 2, 2006"
	}

	if cfg.DateSeparator.Character == "" {
		cfg.DateSeparator.Character = "─"
	}
}
