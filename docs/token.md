# Token storage

Discord identifies a logged-in session by a **token**. Whoever holds
it can act as you, so amitty never keeps it in plain text: it is
encrypted by a command you choose and stored in one file.

## Where and how

| | |
|-|-|
| File | `$XDG_CONFIG_HOME/amitty/credentials/token.gpg`, or `~/.config/amitty/credentials/token.gpg` |
| Encrypt | `token-encrypt-cmd = "gpg --encrypt --default-recipient-self"` |
| Decrypt | `token-decrypt-cmd = "gpg --decrypt --quiet"` |

- **Saving**: after a login, the token goes to the encrypt command on
  stdin; what it writes to stdout is the file. The file is written
  beside the old one and renamed over it, readable by you alone, so a
  failed save keeps the old token.
- **Loading**: at startup, before the screen opens, the decrypt
  command reads the file on stdin and writes the token to stdout.
  Because the screen is not open yet, gpg's pinentry can ask for your
  passphrase in the terminal.

amitty uses no keyring and reads no token from the environment.

## Using another tool

Any command that reads stdin and writes stdout works. With
[age](https://age-encryption.org):

```toml
token-encrypt-cmd = "age --encrypt --recipient age1yourpublickey..."
token-decrypt-cmd = "age --decrypt --identity /home/you/.config/age/key.txt"
```

The commands are split into words as a shell would, but no shell runs
them: no pipes, no variables and no `~`, so paths are written in full.
The file keeps its `.gpg` name whatever the tool.

## What changes the file

| Event | Effect |
|-------|--------|
| A login | The token is saved. |
| Discord refreshes the token while connected | The new one is saved over the old. |
| `logout` (`ctrl+d`) | The file is deleted. |
| Discord rejects the token (gateway close 4004) | The file is deleted and the login is shown. |
| The file exists but does not decrypt | amitty stops, naming the file. It is **not** deleted, since your key or passphrase may come back. Delete it yourself to log in again. |

<!-- nav -->

---

[Previous: Login](login.md) | [Contents](README.md) | [Next: Using amitty](usage.md)
