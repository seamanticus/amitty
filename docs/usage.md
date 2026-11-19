# Using amitty

## The screen

| Pane | Shows |
|------|-------|
| Guilds tree (left) | Your DMs and group DMs, then your guilds, in Discord's order and folders, with their channels and threads. Unread ones stand out. |
| Messages list (middle) | The selected channel's messages, newest at the bottom, with replies, embeds, reactions and image previews. |
| Composer (bottom) | What you type, with the reply or edit in progress and any attachments. |
| Members tree (right) | The channel's members by role, or a DM's recipients. |

`alt+g` and `alt+m` hide or show the guilds and members trees.
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
| `ctrl+h`, `ctrl+l` | The previous or next pane |
| `ctrl+k` | A picker to jump to any channel by name |

Selecting a channel focuses the composer, unless `auto_focus = false`.

## Guilds tree

`j` and `k` (or the arrows) move, `g` and `G` go to the top and bottom,
`enter` opens a channel or expands a guild, folder or category. `J` and
`K` jump to the next or previous unread channel. `-` collapses the
parent, `_` collapses all, `i` copies the ID.

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
line's start and end, `alt+b` and `alt+f` by word, `ctrl+w` deletes the
word behind. A few readline keys need `shift`, as amitty uses the plain
ones elsewhere; telling `ctrl+shift` from `ctrl` needs a terminal with
an enhanced keyboard protocol, such as kitty, ghostty or foot.

## Files

- **Attaching**: `ctrl+\` asks for a path in the terminal; `tab`
  completes it from the file system.
- **Opening**: `o` downloads the attachment to
  `~/.cache/amitty/attachments` and opens it with `xdg-open`, so videos
  go to your video player and documents to their viewer. Types outside
  `allowed_mime_types` (by default images, video and audio) ask first.
- **Saving**: `D` asks where to save it, and asks again before
  replacing a file.

## Leaving

`ctrl+c` quits. `ctrl+z` suspends amitty to the shell; `fg` brings it
back. `ctrl+d` logs out and deletes the stored token.

<!-- nav -->

---

[Previous: Token storage](token.md) | [Contents](README.md) | [Next: Images](images.md)
