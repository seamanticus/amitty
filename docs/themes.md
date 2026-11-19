# Themes

amitty draws in your terminal's own colors by default. A **theme
profile** recolors every style at once from a color scheme:

```toml
theme_profile = "Catppuccin Mocha"
```

- Text, borders, mentions, links and status dots take the scheme's
  colors; the background stays your terminal's, so its transparency and
  font settings keep working.
- The selected message and every other message card get a background
  tint blended from the scheme's background, so cards stand apart. Set
  `ui.messages_list.alternate_every` to change how often, or `0` to
  turn it off.
- Any style you set under `[theme]` wins over the profile's, so a
  profile is a starting point. Without a profile, set
  `theme.messages_list.alternate_style` yourself, such as
  `{ background = "#1e2030" }`.

## How a scheme becomes a theme

The schemes are terminal palettes from
[iTerm2-Color-Schemes](https://github.com/mbadolato/iTerm2-Color-Schemes)
(MIT), the 98 curated by
[maya](https://github.com/agentty-org/maya) (MIT), whose mapping amitty
follows:

| Role | From the palette | Styles |
|------|------------------|--------|
| primary | blue (color 4) | mentions, focused titles and borders, embed titles |
| secondary | green (color 2) | emoji |
| accent | magenta (color 5) | your own reactions |
| success, warning, error | green, yellow, red | online, idle, do not disturb; attachments |
| link | cyan (color 6) | links |
| text | foreground | messages, author names |
| muted | bright black (color 8), lifted to 3:1 contrast | unfocused titles, reactions, embed details, offline |
| border | 22% from the background toward the text | borders, tree lines, scroll track |
| surface | 6% from the background toward the text | the alternate message tint |
| selection | 24% from the background toward blue | the selected message and suggestion |

The palettes are pinned to one upstream commit, so a profile looks the
same on every build. `internal/config/genprofiles` regenerates them.

## Profiles

| Name |
|------|
| `Dracula` |
| `Nord` |
| `Zenburn` |
| `Argonaut` |
| `Chalk` |
| `Cobalt2` |
| `Homebrew` |
| `Ocean` |
| `Oceanic Next` |
| `Seti` |
| `Snazzy` |
| `Spacedust` |
| `Spacegray` |
| `Spacegray Eighties` |
| `Tomorrow` |
| `Tomorrow Night` |
| `Tomorrow Night Eighties` |
| `Wez` |
| `iTerm2 Solarized Dark` |
| `iTerm2 Solarized Light` |
| `Solarized Dark Higher Contrast` |
| `Solarized Darcula` |
| `Gruvbox Dark` |
| `Gruvbox Dark Hard` |
| `Gruvbox Light` |
| `Gruvbox Material Dark` |
| `Gruvbox Material Light` |
| `TokyoNight` |
| `TokyoNight Day` |
| `TokyoNight Moon` |
| `TokyoNight Storm` |
| `Catppuccin Mocha` |
| `Catppuccin Latte` |
| `Catppuccin Frappe` |
| `Catppuccin Macchiato` |
| `Nightfox` |
| `Duskfox` |
| `Nordfox` |
| `Terafox` |
| `Carbonfox` |
| `Dayfox` |
| `Dawnfox` |
| `Atom One Dark` |
| `Atom One Light` |
| `One Half Dark` |
| `One Half Light` |
| `Rose Pine` |
| `Rose Pine Dawn` |
| `Rose Pine Moon` |
| `Kanagawa Wave` |
| `Kanagawa Dragon` |
| `Kanagawa Lotus` |
| `Everforest Dark Hard` |
| `Everforest Light Med` |
| `Ayu` |
| `Ayu Light` |
| `Ayu Mirage` |
| `GitHub` |
| `GitHub Dark` |
| `GitHub Dark Dimmed` |
| `Material` |
| `Material Dark` |
| `Material Darker` |
| `Material Ocean` |
| `Monokai Classic` |
| `Monokai Pro` |
| `Monokai Pro Light` |
| `Monokai Remastered` |
| `Monokai Soda` |
| `Xcode Dark` |
| `Xcode Light` |
| `Flexoki Dark` |
| `Flexoki Light` |
| `Zenwritten Dark` |
| `Zenwritten Light` |
| `Pencil Dark` |
| `Pencil Light` |
| `Vesper` |
| `Poimandres` |
| `Aura` |
| `Andromeda` |
| `Horizon` |
| `Night Owl` |
| `Moonfly` |
| `Firewatch` |
| `Hopscotch` |
| `Neutron` |
| `Jellybeans` |
| `Misterioso` |
| `Obsidian` |
| `Twilight` |
| `Wombat` |
| `Molokai` |
| `Belafonte Night` |
| `Novel` |
| `Man Page` |
| `Piatto Light` |
| `Breeze` |

<!-- nav -->

---

[Previous: Configuration](config.md) | [Contents](README.md) | [Next: Troubleshooting](troubleshooting.md)
