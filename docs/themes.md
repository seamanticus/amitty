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
| ------ | ------------------ | -------- |
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
| ------ |
| `Andromeda` |
| `Argonaut` |
| `Atom One Dark` |
| `Atom One Light` |
| `Aura` |
| `Ayu Light` |
| `Ayu Mirage` |
| `Ayu` |
| `Belafonte Night` |
| `Breeze` |
| `Carbonfox` |
| `Catppuccin Frappe` |
| `Catppuccin Latte` |
| `Catppuccin Macchiato` |
| `Catppuccin Mocha` |
| `Chalk` |
| `Cobalt2` |
| `Dawnfox` |
| `Dayfox` |
| `Dracula` |
| `Duskfox` |
| `Everforest Dark Hard` |
| `Everforest Light Med` |
| `Firewatch` |
| `Flexoki Dark` |
| `Flexoki Light` |
| `GitHub Dark Dimmed` |
| `GitHub Dark` |
| `GitHub` |
| `Gruvbox Dark Hard` |
| `Gruvbox Dark` |
| `Gruvbox Light` |
| `Gruvbox Material Dark` |
| `Gruvbox Material Light` |
| `Homebrew` |
| `Hopscotch` |
| `Horizon` |
| `Jellybeans` |
| `Kanagawa Dragon` |
| `Kanagawa Lotus` |
| `Kanagawa Wave` |
| `Man Page` |
| `Material Dark` |
| `Material Darker` |
| `Material Ocean` |
| `Material` |
| `Misterioso` |
| `Molokai` |
| `Monokai Classic` |
| `Monokai Pro Light` |
| `Monokai Pro` |
| `Monokai Remastered` |
| `Monokai Soda` |
| `Moonfly` |
| `Neutron` |
| `Night Owl` |
| `Nightfox` |
| `Nord` |
| `Nordfox` |
| `Novel` |
| `Obsidian` |
| `Ocean` |
| `Oceanic Next` |
| `One Half Dark` |
| `One Half Light` |
| `Pencil Dark` |
| `Pencil Light` |
| `Piatto Light` |
| `Poimandres` |
| `Rose Pine Dawn` |
| `Rose Pine Moon` |
| `Rose Pine` |
| `Seti` |
| `Snazzy` |
| `Solarized Darcula` |
| `Solarized Dark Higher Contrast` |
| `Spacedust` |
| `Spacegray Eighties` |
| `Spacegray` |
| `Terafox` |
| `TokyoNight Day` |
| `TokyoNight Moon` |
| `TokyoNight Storm` |
| `TokyoNight` |
| `Tomorrow Night Eighties` |
| `Tomorrow Night` |
| `Tomorrow` |
| `Twilight` |
| `Vesper` |
| `Wez` |
| `Wombat` |
| `Xcode Dark` |
| `Xcode Light` |
| `Zenburn` |
| `Zenwritten Dark` |
| `Zenwritten Light` |
| `iTerm2 Solarized Dark` |
| `iTerm2 Solarized Light` |

<!-- nav -->

---

[Previous: Configuration](config.md) | [Contents](README.md) | [Next: Troubleshooting](troubleshooting.md)
