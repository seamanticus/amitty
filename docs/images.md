# Images

amitty draws avatars, custom emoji and image previews in the terminal.
Videos and other files are not drawn: `o` opens them in their
application (see [Using amitty](usage.md#files)).

## Three methods

| Method | Terminals | How it looks |
|--------|-----------|--------------|
| **Kitty graphics** | kitty, ghostty, Rio | Full resolution, drawn by the terminal. |
| **Sixels** | foot | Full resolution, 255 colors with dithering. |
| **Half blocks** | any other | Two colored pixels per cell, `▀` and `▄`. Rough, but readable anywhere. |

Images are scaled down to the cells they take, never up: an avatar is
`ui.messages_list.avatars.height` rows tall, a preview at most 40
columns wide and 20 rows tall.

## How amitty picks one

`image_protocol` in `[ui.messages_list]` decides:

- `"auto"` (the default): amitty asks the terminal its name at
  startup, and uses kitty graphics in kitty, ghostty and Rio, sixels in
  foot, and half blocks in any other.
- `"kitty"`, `"sixel"` or `"halfblocks"`: that method, whatever the
  terminal. Use it for a terminal amitty does not know, or inside a
  multiplexer that hides the terminal's name.

WezTerm and Konsole implement kitty graphics, but not the placeholders
amitty draws them with, so `auto` gives them half blocks.

## How it works

Both kitty graphics and sixels start the same way: each cell of an
image is drawn as a **placeholder**, a private character whose
combining marks and color name the image, its row and its column.
Placeholders are ordinary text, so scrolling, clipping and dialogs
drawn on top need no special care: a covered cell is no longer a
placeholder.

- **Kitty graphics**: amitty sends each image to the terminal once, and
  the terminal draws it wherever its placeholders are. This is kitty's
  [Unicode placeholder](https://sw.kovidgoyal.net/kitty/graphics-protocol/#unicode-placeholders)
  protocol.
- **Sixels**: the terminal knows nothing of placeholders, so amitty
  finds them itself after each frame, blanks them, and writes the
  part of the image they cover as sixels at their cells. An image
  that stays in place is not written again; one that moves or goes is
  redrawn or erased.

The size of a cell in pixels comes from the terminal. One that does not
report it is assumed to have cells 10 pixels wide and 20 tall.

## Limits

- Under a dialog, sixel images are hidden rather than dimmed: a
  terminal cannot dim sixels. They come back when the dialog closes.
- Animated GIFs show their first frame.
- Inside tmux or zellij, set `image_protocol` yourself; their support
  for either protocol varies.

<!-- nav -->

---

[Previous: Using amitty](usage.md) | [Contents](README.md) | [Next: Network and privacy](network.md)
