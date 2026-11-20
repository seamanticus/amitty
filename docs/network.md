# Network and privacy

## The proxy

Every connection to Discord, for the API, the gateway, images and
attachments, goes through the `proxy` in the
[configuration](config.md), or directly when it is empty:

```toml
proxy = "socks5h://127.0.0.1:9050"
```

- Schemes: `http`, `https`, `socks5` and `socks5h`. Both SOCKS5
  schemes resolve names at the proxy, so no DNS query leaves your
  machine. SOCKS4 is refused.
- A proxy that is down fails the connection: amitty never falls back
  to a direct one.
- `HTTP_PROXY` and the other environment variables are not read; only
  `proxy` counts.

## What Discord sees

Discord accepts user clients other than its own only at its
discretion. amitty does its best to look like Discord's web client in
a browser, so it is treated as one:

- **TLS**: connections present Chrome's TLS fingerprint and speak
  HTTP/2, as Chrome does.
- **Headers**: requests carry Chrome's user agent and client hints,
  and the headers Discord's web client sends (`X-Super-Properties`,
  `X-Discord-Locale` and others), in the order Chrome sends them.
- **Build**: the client build number is fetched from discord.com at
  startup, as the web client has it.
- **Fingerprint**: logins carry the `X-Fingerprint` Discord gives
  browsers.

What amitty does not do helps as much: it joins no guilds, sends
nothing on its own, and starts a new DM only when you confirm it, at
most 3 an hour, counted until amitty quits.

## The risk

Using any client other than Discord's own is against Discord's
[Terms of Service](https://discord.com/terms), and accounts using one
can be limited or banned. Looking like the web client lowers the odds;
it does not remove them. You use amitty at your own risk.

<!-- nav -->

---

[Previous: Images](images.md) | [Contents](README.md) | [Next: Configuration](config.md)
