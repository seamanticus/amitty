package config

import (
	"fmt"
	"math"
	"slices"

	"github.com/gdamore/tcell/v3"
)

// palette is a terminal color scheme: its foreground, background, cursor and 16 ANSI colors, as 0xRRGGBB.
type palette struct {
	fg, bg uint32
	colors [16]uint32
}

type namedPalette struct {
	name    string
	palette palette
}

// ProfileNames returns the names theme_profile takes, in maya's curated order.
func ProfileNames() []string {
	var names []string
	for _, p := range profilePalettes() {
		names = append(names, p.name)
	}
	return names
}

func profilePalette(name string) (palette, bool) {
	profiles := profilePalettes()
	i := slices.IndexFunc(profiles, func(p namedPalette) bool { return p.name == name })
	if i < 0 {
		return palette{}, false
	}
	return profiles[i].palette, true
}

// roles are the meanings a palette is read for, following maya's scripts/gen_themes.py: six ANSI colors keep the meaning every terminal program gives them, and the chrome is blended from the background toward the text, which holds for dark and light schemes alike.
type roles struct {
	primary, secondary, accent uint32
	success, error, warning    uint32
	link, text, muted          uint32
	surface, quote, border     uint32
	selection                  uint32
}

func rolesOf(p palette) roles {
	c := p.colors
	return roles{
		primary:   c[4],
		secondary: c[2],
		accent:    c[5],
		success:   c[2],
		error:     c[1],
		warning:   c[3],
		link:      c[6],
		text:      p.fg,
		muted:     readable(c[8], p.bg, p.fg),
		surface:   mix(p.bg, p.fg, 0.06),
		quote:     mix(p.bg, p.fg, 0.12),
		border:    mix(p.bg, p.fg, 0.22),
		selection: mix(p.bg, c[4], 0.24),
	}
}

// mix blends a into b by t, from 0 (a) to 1 (b), per channel.
func mix(a, b uint32, t float64) uint32 {
	var out uint32
	for _, shift := range []uint{16, 8, 0} {
		ca, cb := float64(a>>shift&0xff), float64(b>>shift&0xff)
		out |= uint32(math.Round(ca+(cb-ca)*t)) << shift
	}
	return out
}

// luminance is WCAG's relative luminance.
func luminance(c uint32) float64 {
	channel := func(v uint32) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c>>16&0xff) + 0.7152*channel(c>>8&0xff) + 0.0722*channel(c&0xff)
}

// contrast is WCAG's contrast ratio, from 1 (the same) to 21 (black on white).
func contrast(a, b uint32) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// readable blends c toward fg in twentieths until it contrasts 3:1 with bg, the floor for quiet but legible text.
func readable(c, bg, fg uint32) uint32 {
	candidate := c
	for i := 0; i <= 20 && contrast(candidate, bg) < 3; i++ {
		candidate = mix(c, fg, float64(i+1)/20)
	}
	return candidate
}

func rgb(c uint32) tcell.Color { return tcell.NewHexColor(int32(c)) }

// profileTheme returns the theme of p: the default theme's attributes in p's colors. The selection and the alternate messages take a background blended from p's, which the frame's base paints elsewhere.
func profileTheme(p palette) ThemeConfig {
	r := rolesOf(p)
	plain := tcell.StyleDefault
	fg := func(c uint32) tcell.Style { return plain.Foreground(rgb(c)) }
	focus := StyleThemeConfig{Style{fg(r.border)}, Style{fg(r.primary).Bold(true)}}
	text := StyleThemeConfig{Style{fg(r.muted)}, Style{fg(r.primary).Bold(true)}}
	tree := TreeThemeConfig{
		GraphicsColor: rgb(r.border).CSS(),
		OnlineStyle:   Style{fg(r.success)},
		IdleStyle:     Style{fg(r.warning)},
		DNDStyle:      Style{fg(r.error)},
		OfflineStyle:  Style{fg(r.muted)},
	}
	theme := defaultTheme()
	theme.Title, theme.Footer, theme.Border = text, text, focus
	theme.GuildsTree, theme.MembersTree = GuildsTreeThemeConfig{tree}, tree
	theme.ScrollBar = ScrollBarThemeConfig{TrackStyle: Style{fg(r.border)}, ThumbStyle: Style{fg(r.muted)}}

	list := &theme.MessagesList
	// A role color, where there is one, takes its place; the primary color sets names apart from the text.
	list.AuthorStyle = Style{fg(r.primary).Bold(true)}
	list.MentionStyle = Style{fg(r.primary).Bold(true)}
	list.EmojiStyle = Style{fg(r.secondary)}
	list.URLStyle = Style{fg(r.link)}
	list.AttachmentStyle = Style{fg(r.warning)}
	list.ReactionStyle = Style{fg(r.muted)}
	list.OwnReactionStyle = Style{fg(r.accent).Bold(true)}
	list.MessageStyle = Style{fg(r.text)}
	list.SelectedMessageStyle = Style{plain.Background(rgb(r.selection))}
	list.AlternateStyle = Style{plain.Background(rgb(r.surface))}
	list.ReplyStyle = Style{fg(r.muted).Italic(true).Background(rgb(r.quote))}
	list.Embeds = MessagesListEmbedsThemeConfig{
		ProviderStyle:    Style{fg(r.muted).Italic(true)},
		AuthorStyle:      Style{fg(r.text).Italic(true)},
		TitleStyle:       Style{fg(r.primary).Bold(true)},
		DescriptionStyle: Style{fg(r.muted)},
		FieldNameStyle:   Style{fg(r.text).Bold(true).Underline(true)},
		FieldValueStyle:  Style{fg(r.text)},
		FooterStyle:      Style{fg(r.muted).Italic(true)},
		URLStyle:         Style{fg(r.link).Underline(true)},
	}

	theme.Autocomplete.SelectedStyle = Style{plain.Background(rgb(r.selection))}
	theme.Help = HelpThemeConfig{ShortKeyStyle: Style{fg(r.muted)}, ShortDescStyle: Style{fg(r.text)}, FullKeyStyle: Style{fg(r.muted)}, FullDescStyle: Style{fg(r.text)}}
	return theme
}

// BaseStyle returns the style a frame's uncolored cells take: the profile's text and background, its text alone with inherit_background, or none without a profile.
func (cfg *Config) BaseStyle() tcell.Style {
	p, ok := profilePalette(cfg.ThemeProfile)
	if !ok {
		return tcell.StyleDefault
	}
	style := tcell.StyleDefault.Foreground(rgb(p.fg))
	if !cfg.InheritBackground {
		style = style.Background(rgb(p.bg))
	}
	return style
}

// applyProfile sets the theme of the profile name, or leaves the default theme for "".
func (cfg *Config) applyProfile(name string) error {
	if name == "" {
		return nil
	}
	p, ok := profilePalette(name)
	if !ok {
		return fmt.Errorf("unknown theme_profile %q; the profiles are listed in docs/themes.md", name)
	}
	cfg.Theme = profileTheme(p)
	return nil
}
