# amitty manual

This manual says what amitty does, how it does it, and what that means
for you. It is written for people who build amitty from source and run
it in a Linux terminal. It assumes you know Discord and your terminal,
but not how Discord's API or amitty's code work.

Each page covers one subject, and each fact has one home: other pages
link to it instead of repeating it.

The commit history is the record of changes. This manual describes the
code as it is at the commit you are reading it from.

## Contents

- [Overview](overview.md): what amitty is and is not, its parts, and
  how a message travels through them.
- [Install and run](install.md): building, flags, and where files live.
- [Login](login.md): password with 2FA, QR and token logins, the
  captcha page, and when a login is retried.
- [Token storage](token.md): how the token is encrypted, swapping gpg
  for another tool, and what deletes it.
- [Using amitty](usage.md): the panes, focus, keybinds, attaching and
  opening files.
- [Images](images.md): kitty graphics, sixels and half blocks, and how
  amitty picks one.
- [Network and privacy](network.md): the proxy, what Discord sees of
  amitty, and the risk to your account.
- [Configuration](config.md): every option, its meaning and default.
  Generated from the code.
- [Themes](themes.md): theme profiles, and how a color scheme colors
  amitty.
- [Troubleshooting](troubleshooting.md): logs, crash traces, and
  common failures.

## Conventions

- `code` is a name in the code or a literal value. File paths are
  relative to the repository root unless they start with `~` or `$`.
- "Discord" is the service and its API; "the client" is the official
  Discord app or website.
- Keys are written as amitty's config writes them: `ctrl+k`,
  `alt+g`, `shift+tab`. The keys given are the defaults; every one can
  be changed under `[keybinds]` in the [configuration](config.md).
- Pages are listed above in reading order; the footer of each page
  links the previous one, these contents and the next one.

<!-- nav -->

---

[Next: Overview](overview.md)
