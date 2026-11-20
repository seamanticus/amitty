# Overview

amitty is a Discord client for the Linux terminal, for direct messages
and chatting in servers (guilds). It started as a fork of
[discordo](https://github.com/ayn2op/discordo).

## What it is not

amitty leaves out what it does not need, on purpose:

- **No voice or video calls.** Voice and stage channels are shown in
  the guilds tree, but cannot be joined.
- **No bots.** amitty logs in as a user, and cannot run a bot or
  register slash commands.
- **Few new conversations.** amitty opens the DMs and group DMs you
  already have. A new DM with a member starts only after you confirm
  it, and at most 3 an hour, as starting one is among what Discord
  watches most closely for spam (see [Network and privacy](network.md)).
- **Linux only.** It builds and runs on Linux; other systems are not
  supported.

## Its parts

| Directory | Role |
|-----------|------|
| `cmd/` | Startup: flags, the log, the config, the stored token, the screen. |
| `internal/ui/` | The screens: login, the guilds and members trees, the messages list, the composer, pickers. |
| `internal/tui/` | The terminal UI framework, folded in from [ayn2op/tview](https://github.com/ayn2op/tview): widgets, layout, and the sixel layer. |
| `internal/dmwire/` | Discord's protocol: the REST API, the gateway and the data types. Forked from [arikawa](https://github.com/diamondburned/arikawa) and trimmed to what a user client needs. |
| `internal/ningen/` | The client's state on top of the protocol: read states, members, guild order, notes. |
| `internal/http/`, `internal/tls/`, `internal/gateway/` | Every connection to Discord, made to look like a browser's (see [Network and privacy](network.md)). |
| `internal/captcha/` | The captcha page opened in your browser during a login. |
| `internal/tokenstore/` | The encrypted token on disk (see [Token storage](token.md)). |
| `internal/config/` | The options, their defaults, and the generated [configuration](config.md) reference. |

## How a message travels

1. At startup amitty decrypts the stored token, or shows the
   [login](login.md) screen when there is none.
2. It opens the **gateway**, Discord's websocket, which sends the
   account's guilds, channels and DMs, then every change as it happens.
3. Selecting a channel fetches its newest messages over **REST**, the
   HTTP API, and asks the gateway for the members who wrote them.
4. A message you send goes out over REST; it appears in the list when
   the gateway echoes it back, as it does for every client.
5. Images are downloaded from Discord's media proxy, scaled to the
   cells they take, and drawn with the best [method](images.md) your
   terminal supports.

<!-- nav -->

---

[Previous: Contents](README.md) | [Contents](README.md) | [Next: Install and run](install.md)
