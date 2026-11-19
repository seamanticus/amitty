# amitty &middot; [![license](https://img.shields.io/github/license/seamanticus/amitty?logo=github)](https://github.com/seamanticus/amitty/blob/master/LICENSE)

amitty is a lightweight and secure Discord terminal client for Linux, for chat and direct messages. It is a fork of [discordo](https://github.com/ayn2op/discordo).

![Preview](.github/preview.png)
![Picker](.github/picker.png)

## Installation

### Prebuilt binaries

You can download and install a [prebuilt binary here](https://nightly.link/seamanticus/amitty/workflows/ci/master) for Linux.

### Building from source

Requires [Go](https://go.dev/dl).

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

## License

Copyright (C) 2025-present ayn2op
Copyright (C) 2026-present seamanticus

This project is licensed under the GNU General Public License v3.0 (GPL-3.0).
See the [LICENSE](./LICENSE) file for the full license text.

`internal/dmwire` (from [arikawa](https://github.com/diamondburned/arikawa)) and `internal/ningen` (from [ningen](https://github.com/diamondburned/ningen)) are ISC licensed, Copyright 2020 diamondburned; see the `LICENSE` file in each.

# Disclaimer

> [!IMPORTANT]
> Automated user accounts or "self-bots" are against Discord's Terms of Service. I am not responsible for any loss caused by using "self-bots" or amitty.
