# Troubleshooting

## The log

amitty logs to `~/.cache/amitty/logs.txt`, emptied at each start (see
[Install and run](install.md)), one JSON object per line. Start it with
`-log-level debug` to also log every gateway frame's op and type
(`gateway recv`, `gateway send`) and every API request as it is sent
(`api request`).

`-log-level raw-debug` logs what crosses the wire in full, for
following what Discord sends when it changes:

| `msg` | One record per |
|-------|----------------|
| `api exchange` | API request and its response: `method`, `url`, `status`, `ms`, then `request` and `response`, each with `headers` in the order sent and `body`; `error` instead of a response when none came |
| `gateway send`, `gateway recv` | gateway frame: `op`, `t`, `s` and `body` |
| `remote-auth send`, `remote-auth recv` | QR login frame, with `body` |

A JSON body is logged as JSON, so `jq` reads the log:

```sh
jq 'select(.msg == "api exchange" and .status >= 400)' logs.txt
jq 'select(.msg == "gateway send") | {op, body}' logs.txt
```

**`raw-debug` is a developer tool and its log is explicit.** Only the
`authorization` header, tokens, login tickets and passwords are replaced
by `<redacted>`, and an uploaded file by its size. Everything else is
logged as sent: your email, cookies, captcha answers, QR login keys,
upload URLs, message text and IDs. The file is readable by you alone;
treat it as a secret, never share it, and delete it when done.

Below `raw-debug`, the log can hold channel and user IDs, but never
your token or password; still, read it before sharing it.

## After a crash

A crash prints its trace, and copies it to the end of the log, so it is
kept even when the terminal is garbled. If the terminal is left in a
strange state (no echo, no cursor, odd keys), type `reset` and press
enter, even if you cannot see what you type.

## Common failures

| Symptom | Cause and fix |
|---------|---------------|
| amitty stops at start, naming `token.gpg` | The token does not decrypt: gpg's key or passphrase is missing. Fix gpg, or delete the file to log in again (see [Token storage](token.md)). |
| The login screen comes back on its own | Discord rejected the token (gateway close 4004), which deletes it. Log in again. |
| "did not accept the captcha after 3 tries" | Discord kept rejecting the answers. Wait, then try again or use the QR login (see [Login](login.md#the-captcha-page)). |
| "Discord does not know this location yet and emailed you a link" | A login from a new place or device. Click Verify Login in Discord's email, then log in again; the form keeps what you typed. `credentials/browser.json` makes later runs the same device. |
| The captcha page does not open | No browser handles `xdg-open`. Open the address the login screen shows. |
| Images are blocky | The terminal gets half blocks. Check [Images](images.md#how-amitty-picks-one), and set `image_protocol` if your terminal supports more than amitty detected. |
| Sixel images leave traces | Report it, with the terminal and its version. `ctrl+z` then `fg` redraws everything. |
| Nothing connects | Check `proxy`: a proxy that is down fails every connection (see [Network and privacy](network.md#the-proxy)). |
| A config change does nothing | amitty reads the config at start: restart it. Check the log for "failed to decode config". |

## Reporting a bug

Open an issue at
[seamanticus/amitty](https://github.com/seamanticus/amitty/issues)
with what you did, what happened, `amitty -version`, your terminal,
and the relevant part of the log.

<!-- nav -->

---

[Previous: Themes](themes.md) | [Contents](README.md)
