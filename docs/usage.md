# Using amitty

## The screen

| Pane | Shows |
|------|-------|
| Guilds tree (left) | Your DMs and group DMs, then your guilds, in Discord's order and folders, with their channels and threads. Unread ones stand out. |
| Messages list (middle) | The selected channel's messages, newest at the bottom, with replies, embeds, reactions and image previews. |
| Composer (bottom) | What you type, with the reply or edit in progress and any attachments. |
| Members tree (right) | The channel's members by role, or a DM's recipients. `enter` on one opens your DM with them, or asks whether to start one (at most 3 an hour) and `v` shows their profile: names, server tag, status and activities, roles, member and account dates, badges, your relationship and note; `y` copies their ID and `esc` closes it. In a terminal narrower than 100 columns it folds under the guilds tree; see `members_tree.fold` in the [configuration](config.md). |

`alt+g` and `alt+y` hide or show the guilds and members trees.
In a terminal narrower than `ui.on_resize.is_less_than` (100 columns, or
pixels such as `"1200px"`; 0 turns it off) the guilds tree widens to
`sidebar.on_resize.width_percent` (30%), and the members tree folds under it.
`alt+.` hides or shows the keybind help at the bottom, and `ctrl+.`
switches it between compact and full: the help always lists the keys
of the pane that has focus.

## Focus

| Key | Focuses |
|-----|---------|
| `ctrl+g` | Guilds tree |
| `ctrl+t` | Messages list |
| `ctrl+i` | Composer |
| `ctrl+y` | Members tree |
| `ctrl+h`, `ctrl+l`, `ctrl+up`, `ctrl+down` | The previous or next pane |
| `ctrl+/` | A picker to jump to any channel by name; most terminals send it as `ctrl+_`, which opens it too |

Selecting a channel keeps the focus in the guilds tree; `auto_focus = true` moves it to the composer.

`ctrl+i` reaches amitty only in terminals with an enhanced keyboard
protocol (kitty, ghostty, foot, WezTerm): elsewhere it sends the same
byte as `tab`.

## Guilds tree

A DM from someone you are not friends with that waits in your message
requests is marked "(request)": `a` accepts it, moving it to your DMs,
and `r` rejects and deletes it; both ask first, No selected. DMs that
Discord filtered as spam are not listed.

`j` and `k` (or the arrows) move, `g` and `G` go to the top and bottom,
`enter` opens a channel or expands a guild, folder or category. `J` and
`K` jump to the next or previous unread channel. `-` collapses the
parent, `_` collapses all, `i` copies the ID. `m` marks every unread
channel of the highlighted server (or of a channel's server, or of
every server in a folder) as read, after asking, No selected.

`+` joins a server: paste an invite (`discord.gg/...`,
`discord.com/invite/...` or the code alone) and press `enter`. amitty
looks it up and asks "Join <server> (N members)?", No selected; an
invite to a server you are already in just opens it. Discord asks for a
captcha on every new join, which opens in your browser. A server that
makes new members accept its rules or answer questions first says so:
finish that in the web client or the mobile app. Joins and new DMs
together are limited to 3 an hour.

## Messages list

`j` and `k` select messages; `K` and `J` scroll without selecting.

| Key | Does |
|-----|------|
| `r`, `R` | Reply, with or without mentioning the author |
| `s` | Select the message this one replies to |
| `e` | Edit your message |
| `d` | Delete your message, after confirming |
| `a` | React, with an emoji picked from a list |
| `o` | Open an attachment with its application |
| `O` | Open an attachment or link in the browser |
| `D` | Download an attachment |
| `y`, `u`, `i` | Copy the content, the link or the ID |
| `esc` | Clear the selection |

## Composer

`enter` sends and `shift+enter` adds a line. `up` in an empty composer
edits your last message. `tab` completes `@` mentions and `:` emoji
from a list; `esc` cancels a reply or clears the text. `ctrl+v` pastes
text or an image from the clipboard, `ctrl+e` writes the message in
`$EDITOR`, and `ctrl+u` undoes.

The cursor keys follow readline: `ctrl+a` and `ctrl+shift+e` for the
line's start and end, `alt+b` and `alt+f` (or `alt+left` and
`alt+right`) by word, `ctrl+w` deletes the
word behind. A few readline keys need `shift`, as amitty uses the plain
ones elsewhere; telling `ctrl+shift` from `ctrl` needs a terminal with
an enhanced keyboard protocol, such as kitty, ghostty or foot.

## Files

- **Attaching**: `ctrl+\` asks for a path in the terminal; `tab`
  completes it from the file system. To use another picker, set
  `file_picker`: amitty suspends itself while it runs, then attaches
  every path it chose, one per line on its stdout, or in the file named
  where `{file}` is written:

  ```toml
  file_picker = "fzf --multi"
  # file_picker = "yazi --chooser-file={file}"
  ```

  Attached files are uploaded when you send the message, as the web
  client uploads them; if an upload fails, nothing is sent and a
  dialog says why.

- **Opening**: `o` downloads the attachment to
  `~/.cache/amitty/attachments` and opens it with `xdg-open`, so videos
  go to your video player and documents to their viewer. Types outside
  `allowed_mime_types` (by default images, video and audio) ask first.
  `O` opens a link or attachment page in the browser.
- **Openers**: `openers` sets the program for a MIME type, a wildcard,
  or links (`url`), in place of `xdg-open`. `{}` is the file or link,
  appended when absent; an exact type wins over a wildcard. Programs
  start in the background, or with `terminal = true` take the terminal
  while amitty is suspended:

  ```toml
  [openers]
  "video/*" = { command = "mpv {}" }
  "image/*" = { command = "imv {}" }
  "text/plain" = { command = "nvim {}", terminal = true }
  url = { command = "firefox {}" }
  ```

- **Saving**: `D` asks where to save it, in the path prompt starting in
  the download directory, and asks again before replacing a file. The
  directory is `downloads.dir`, else `$XDG_DOWNLOAD_DIR` (from the
  environment or `~/.config/user-dirs.dirs`), else `~/Downloads`; it is
  created if missing. `mode = "dir"` saves there without asking,
  numbering a taken name (`photo (1).png`). `picker` asks with another
  program, run in that directory; a directory it chooses gets the file's
  own name:

  ```toml
  [downloads]
  mode = "ask"
  dir = "~/Downloads/discord"
  picker = "yazi --cwd-file={file}"  # the directory yazi quits in
  # picker = "fzf --walker=dir,hidden"
  ```

## Notifications

A message that mentions you, or any message in a DM, shows a desktop
notification (through the terminal) and plays a short sound, unless
your status is Do not disturb. amitty ships 14 sounds (`bell.wav`,
`blip.wav`, `bubble.wav`, `chime.wav`, `coin.wav`, `ding.wav`,
`dong.wav`, `glass.wav`, `knock.wav`, `marimba.wav`, `pluck.wav`,
`pop.wav`, `sparkle.wav`, `triad.wav`), written at start (when the sound is on) to
`$XDG_DATA_HOME/amitty/sounds/` (usually
`~/.local/share/amitty/sounds/`). `notifications.sound.name` picks a
file there; files of your own dropped in that directory, in any format
the player reads, are listed too. In the settings panel each plays as
you move over it. A file elsewhere goes in `notifications.sound.path`
(a leading `~/` is your home).

It plays through the first of `ffplay` (any format), `pw-play`,
`paplay` and `aplay` found, or `notifications.sound.command`, such as
`"mpv --no-video {}"`. `notifications.popup` and
`notifications.sound.enabled` switch the notification and the chime on
or off independently, so `popup = false` leaves the chime alone.

## Settings

`ctrl+k` opens the settings panel; `ctrl+k` or `esc` closes it. It lists
every option of the [configuration](config.md) by its path, with the
selected one's description below. Type to filter, such as `avatar`.

- `enter` on a true/false option flips it, and on one with up to six
  fixed values (avatar shape, border set, scroll bar, status) steps to
  the next.
- An option with more values, `theme_profile` or `markdown.theme`,
  opens them as a list; each is shown as the cursor reaches it, `enter`
  keeps it and `esc` goes back to yours.
- `enter` on any other option opens its value to edit as TOML, such as
  `"ctrl+q"` or `{ attributes = "bold" }`, saved with `enter`.
- A change is written to `config.toml` at once, on the option's own
  line: your comments and the rest of the file stay as they are. A value
  that is not valid for the option is refused, and the file kept.
- Changes apply at once, but for those marked "(on restart)": the
  proxy, the token commands, the mouse, focus reports and the image
  protocol, which amitty uses only at startup.
- Tables of your own entries, such as `openers`, are edited in the file.

## Leaving

`ctrl+c` quits. `ctrl+z` suspends amitty to the shell; `fg` brings it
back. `ctrl+d` logs out and deletes the stored token.

<!-- nav -->

---

[Previous: Token storage](token.md) | [Contents](README.md) | [Next: Images](images.md)
