// Package tokenstore keeps the account token in one file, encrypted and decrypted by commands from the config, such as gpg.
package tokenstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
)

// FileName is the file in the state directory that holds the encrypted token.
const FileName = "token.gpg"

// Store is a token file and the commands that encrypt and decrypt it. Both commands read stdin and write stdout.
type Store struct {
	path             string
	encrypt, decrypt []string
}

// New returns the store at path, with the encrypt and decrypt commands parsed as shell words.
func New(path, encryptCmd, decryptCmd string) (Store, error) {
	encrypt, err := parseCommand("token-encrypt-cmd", encryptCmd)
	if err != nil {
		return Store{}, err
	}
	decrypt, err := parseCommand("token-decrypt-cmd", decryptCmd)
	if err != nil {
		return Store{}, err
	}
	return Store{path: path, encrypt: encrypt, decrypt: decrypt}, nil
}

func parseCommand(key, command string) ([]string, error) {
	args, err := shellwords.Parse(command)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("%s is empty", key)
	}
	return args, nil
}

// Path returns the token file.
func (s Store) Path() string { return s.path }

// Load decrypts the stored token. ok is false, with no error, when no token is stored. The command's stderr goes to stderr, so a pinentry or gpg message reaches the user.
func (s Store) Load(ctx context.Context) (token string, ok bool, err error) {
	if len(s.decrypt) == 0 {
		return "", false, errors.New("token store has no decrypt command")
	}
	file, err := os.Open(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("open token: %w", err)
	}
	defer file.Close() //nolint:errcheck

	plain, err := run(ctx, s.decrypt, file, os.Stderr)
	if err != nil {
		return "", false, fmt.Errorf("decrypt token: %w", err)
	}
	token = strings.TrimSpace(string(plain))
	if token == "" {
		return "", false, errors.New("decrypted token is empty")
	}
	return token, true, nil
}

// Save encrypts token and replaces the stored one in a single rename, so a failed save leaves the old token in place. The command's stderr is returned in the error, as a terminal UI owns the screen while it runs.
func (s Store) Save(ctx context.Context, token string) error {
	if len(s.encrypt) == 0 {
		return errors.New("token store has no encrypt command")
	}
	if token == "" {
		return errors.New("refusing to store an empty token")
	}
	var stderr bytes.Buffer
	sealed, err := run(ctx, s.encrypt, strings.NewReader(token), &stderr)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = fmt.Errorf("%w: %s", err, msg)
		}
		return fmt.Errorf("encrypt token: %w", err)
	}
	if len(sealed) == 0 {
		return errors.New("encrypt token: the command wrote nothing")
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".token-*")
	if err != nil {
		return fmt.Errorf("create token file: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck
	// CreateTemp makes the file 0600, readable by its owner alone.
	if _, err := tmp.Write(sealed); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write token: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write token: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write token: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("replace token: %w", err)
	}
	return nil
}

// Delete removes the stored token. Deleting a missing token is not an error.
func (s Store) Delete() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete token: %w", err)
	}
	return nil
}

// run runs args with stdin, returning its stdout and sending its stderr to stderr.
func run(ctx context.Context, args []string, stdin io.Reader, stderr io.Writer) ([]byte, error) {
	var stdout bytes.Buffer
	command := exec.CommandContext(ctx, args[0], args[1:]...)
	command.Stdin, command.Stdout, command.Stderr = stdin, &stdout, stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("run %s: %w", args[0], err)
	}
	return stdout.Bytes(), nil
}
