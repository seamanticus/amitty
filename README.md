<div align="center">
  <img src=".github/logo_512.png" />
</div>

Focus on friends, not the noise. No microphone, no buzzing, no bots:
just clean DMs and feature-rich chatting.

amitty brings Discord's DMs and servers to your Linux terminal, through
a minimal, keyboard-driven TUI written in Go.

## Features

- DMs, group DMs and servers, with threads, forums and folders
- replies, edits, reactions, mentions and custom emoji
- images drawn in the terminal: kitty graphics (kitty, ghostty, Rio),
  sixels (foot) and half blocks everywhere else
- videos and files open in their own applications with `xdg-open`
- attachments from a path prompt or the clipboard
- password with 2FA, QR and token logins
- the token encrypted with gpg, or any tool you choose
- one proxy for every connection, SOCKS5 included
- every key, style and icon configurable

## Install

```sh
git clone https://github.com/seamanticus/amitty
cd amitty
go build ./cmd/amitty
./amitty
```

Requires [Go 1.27](https://go.dev/dl), `gpg` and `xdg-open`.

## Essentials

- Log in from the **Password**, **QR** or **Token** tab.
- `ctrl+k` jumps to a channel, `ctrl+.` shows every key, `ctrl+c`
  quits.
- The config is written to `~/.config/amitty/config.toml` on first run,
  every option commented out.

Everything else is in the [manual](docs/README.md).

> Using a client other than Discord's own is against Discord's Terms
> of Service. Use amitty at your own risk.

## Credits

amitty is a fork of [discordo](https://github.com/ayn2op/discordo) by
ayn2op and its contributors. Its protocol layer started as
[arikawa](https://github.com/diamondburned/arikawa) and its state layer
as ningen, both by diamondburned; its terminal UI as
[tview](https://github.com/ayn2op/tview) by ayn2op, after rivo's
original. Thank you all.

License: [GPL-3.0](LICENSE). The folded-in arikawa, ningen and tview
keep their own licenses in their directories.
