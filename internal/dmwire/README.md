# dmwire

amitty's Discord protocol layer, as a single user account sees it: text channels and direct messages over the gateway and REST API. No voice, no bot framework, no sharding.

## Packages

- `discord`: the API types (snowflakes, channels, messages, users, guilds).
- `api`: the REST client, with per-route rate limits.
- `gateway`: the websocket gateway: identify, heartbeat, resume, events.
- `session`: a gateway and an API client together.
- `state`: a session that caches what the gateway sends (`state/store`).
- `ningen`: client-side state on top of `state`: read states, unread and mention indicators, member lists.
- `utils`: HTTP (`httputil`, swappable drivers), websocket, JSON helpers, multipart sends.

## Origin

dmwire is a fork of [arikawa](https://github.com/diamondburned/arikawa) by diamondburned, through [ayn2op's fork](https://github.com/ayn2op/arikawa), and folds in [ningen](https://github.com/ayn2op/ningen). Both are ISC licensed; see `LICENSE`.
