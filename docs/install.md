# Install and run

## Building

amitty needs [Go 1.27](https://go.dev/dl) and nothing else: every
dependency is pure Go, including the clipboard, which speaks Wayland
natively and X11 otherwise.

```sh
git clone https://github.com/seamanticus/amitty
cd amitty
go build ./cmd/amitty
```

The binary is `./amitty`. Copy it anywhere on your `PATH`.

The build tag `no_syntax_highlighting` leaves out the highlighting of
code blocks in messages, which makes the binary smaller:

```sh
go build -tags no_syntax_highlighting ./cmd/amitty
```

## Running

`amitty` takes no arguments. Its flags are:

| Flag | Default | Meaning |
|------|---------|---------|
| `-config-path` | `$XDG_CONFIG_HOME/amitty/config.toml` | The [configuration](config.md) file. |
| `-log-path` | `$XDG_CACHE_HOME/amitty/logs.txt` | The log file, emptied at each start. |
| `-log-level` | `info` | `debug`, `info`, `warn` or `error`. `debug` also logs the type of every gateway event. |
| `-version` | | Print the version and exit. |

On the first run, amitty writes a configuration file at
`-config-path` with every option commented out at its default under a
short description. It never touches the file again: uncomment an
option to change it.

## Where files live

| Path | Holds |
|------|-------|
| `~/.config/amitty/config.toml` | The configuration. |
| `~/.local/state/amitty/token.gpg` | The encrypted token (see [Token storage](token.md)). |
| `~/.cache/amitty/logs.txt` | The log, and the trace of any crash (see [Troubleshooting](troubleshooting.md)). |

Each follows its XDG variable when it is set: `$XDG_CONFIG_HOME`,
`$XDG_STATE_HOME` and `$XDG_CACHE_HOME`.

## What it needs at run time

- A terminal. For images beyond half blocks, one with kitty graphics or
  sixels (see [Images](images.md)).
- `gpg`, or the tool you set in its place, to keep the token
  (see [Token storage](token.md)).
- `xdg-open`, to open attachments, links and the captcha page in your
  desktop's applications.

<!-- nav -->

---

[Previous: Overview](overview.md) | [Contents](README.md) | [Next: Login](login.md)
