<div align="center">
  <img src=".github/logo_512.png" />
</div>

A minimalistically focused terminal client for Linux for DM's and chatting with friends on Discord servers. Written in Go.

### Building from source

Requires [Go 1.28](https://go.dev/dl).

```sh
git clone https://github.com/seamanticus/amitty
cd amitty
go build ./cmd/amitty
```

#### Tags

- `no_syntax_highlighting` to build without syntax highlighting in code blocks.

## Usage

### Password (UI)

1. Run the `amitty` executable with no arguments.

2. Open the "Password" tab, enter your email address or E.164-formatted phone number and password, then click the "Login" button.

### QR (UI)

1. Run the `amitty` executable with no arguments.

2. Click on the "Login with QR" button.

3. Follow the instructions in the QR Login screen.

### Token (UI)

1. Run the `amitty` executable with no arguments.

2. Enter your token and click on the "Login" button to save it.

### Stored token

A token from any login is encrypted with `token-encrypt-cmd` and stored in `$XDG_STATE_HOME/amitty/token.gpg` (`~/.local/state/amitty/token.gpg` when unset). At startup, before the screen opens, `token-decrypt-cmd` decrypts it, so gpg can ask for the passphrase in the terminal. The defaults are:

```toml
token-encrypt-cmd = "gpg --encrypt --default-recipient-self"
token-decrypt-cmd = "gpg --decrypt --quiet"
```

Both commands read stdin and write stdout, so any tool that does works, such as `age`. Logging out, or Discord rejecting the token, deletes the file; a token Discord refreshes is written back.

## Configuration

The configuration file allows you to configure and customize the behavior, keybindings, and theme of the application. It is at `$XDG_CONFIG_HOME/amitty/config.toml`, or `$HOME/.config/amitty/config.toml`.

Use `-config-path` to change the path.

amitty uses the default configuration if a configuration file is not found in the aforementioned path; however, the default configuration file is not written to the path. [The configuration reference can be found here](./docs/config.md).

## Logs

The log file is replaced on each run. It is at `$XDG_CACHE_HOME/amitty/logs.txt`, or `$HOME/.cache/amitty/logs.txt`.

Use `-log-path` to change the path, and `-log-level` (`debug`, `info`, `warn`, or `error`) to set the verbosity of what is logged.

## License and credits

This is a fork of ayn2op/discordo

Copyright (C) 2025-present ayn2op
Copyright (C) 2026-present seamanticus

This project is licensed under the GNU General Public License v3.0 (GPL-3.0).
See the [LICENSE](./LICENSE) file for the full license text.

`internal/dmwire` (from [arikawa](https://github.com/diamondburned/arikawa)) and `internal/ningen` (from [ningen](https://github.com/diamondburned/ningen)) are ISC licensed, Copyright 2020 diamondburned; see the `LICENSE` file in each.
