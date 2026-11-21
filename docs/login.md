# Login

amitty shows the login screen when it has no stored token: the logo,
when the terminal has room for it, then the **Authentication Method**
box with three tabs, **Password**, **QR** and **Token**, and below it
the box of the chosen method. `ctrl+h` and `ctrl+l` move between the
tabs, `tab` and `shift+tab` between the fields of one.

Leaving the QR tab ends its login; coming back shows a new code.

Every login ends the same way: the token Discord gives is encrypted and
stored (see [Token storage](token.md)), and amitty opens the chat. The
next start skips the login.

## Password

In **Email or phone number**, enter your email address, or your phone
number in E.164 form (`+64211234567`), then your password, and press
**Log in**.

If the account has two-factor authentication, the **Credentials** box
turns into **Two-Factor Authentication** and asks for a code. `left` and `right`
choose how to give it, among the methods the account allows:

- **Authenticator**: the six digits of your TOTP app.
- **SMS**: press **Send SMS**, and enter the code sent to the phone
  shown. The SMS is sent once per press, never again on its own.
- **Backup code**: one of the codes Discord gave you to keep.

`esc` returns to the form, unless a request is in flight.

## QR

The QR tab shows a code to scan with the Discord app on your phone
(Settings, Scan QR Code). The phone then asks you to confirm, and the
login completes.

- The code expires after the time counted down under it. Press `r`
  for a new one.
- Before showing the code, amitty checks the key Discord's server
  proved it holds against the fingerprint it sent, so a code is never
  shown for a session it cannot verify.

## Token

Paste a token and press **Log in**. amitty stores it as it is; a token
Discord rejects is deleted at the first connection (see
[Token storage](token.md)). Prefer the password or QR login: a token
copied out of a browser is tied to that browser's session.

## The captcha page

Discord sometimes answers a password or QR login with an hCaptcha
challenge. amitty then opens a page in your browser with `xdg-open`:

- The page is served by amitty itself at `http://127.0.0.1` on a
  random port and path, and only while the login waits for it. Only
  `127.0.0.1` works: hCaptcha refuses these challenges on `localhost`.
- If no browser opens, the login screen shows the address to open by
  hand.
- Solve it in the browser; the login carries on in the terminal.
- You have 5 minutes and 3 tries. A rejected answer brings a new
  challenge; after the third, the login fails with a suggestion to use
  QR.
- `esc` cancels the captcha and the login.

## When a login is retried

A login is sent once. If the connection itself fails, or Discord
answers 502, 503 or 504 (its servers are busy), amitty retries **once**
after a 5 second countdown, shown on screen. Any other answer is final:
a wrong password, a wrong code or a rate limit is shown, never retried,
so amitty never hammers Discord with credentials.

<!-- nav -->

---

[Previous: Install and run](install.md) | [Contents](README.md) | [Next: Token storage](token.md)
