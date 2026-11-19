package tokenstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newStore is a store in a temporary directory with the given commands.
func newStore(t *testing.T, encrypt, decrypt string) Store {
	t.Helper()
	store, err := New(filepath.Join(t.TempDir(), "state", "amitty", FileName), encrypt, decrypt)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

// evidence: witnesses
func TestNew(t *testing.T) {
	for _, tt := range []struct {
		name, encrypt, decrypt string
		wantErr                bool
	}{
		{"gpg as configured", "gpg --encrypt --default-recipient-self", "gpg --decrypt --quiet", false},
		{"quoted arguments", `sh -c "tr a-z n-za-m"`, "cat", false},
		{"empty encrypt", "", "cat", true},
		{"empty decrypt", "cat", "", true},
		{"only spaces", "   ", "cat", true},
		{"unbalanced quote", `gpg "--encrypt`, "cat", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New("token.gpg", tt.encrypt, tt.decrypt); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// rot13 stands in for gpg: what is on disk differs from the token, and decrypting undoes it.
	const rot13 = "tr A-Za-z N-ZA-Mn-za-m"
	store := newStore(t, rot13, rot13)

	if err := store.Save(t.Context(), "Secret.Token"); err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(onDisk), "Secret.Token") {
		t.Fatalf("token stored in clear: %q", onDisk)
	}
	token, ok, err := store.Load(t.Context())
	if err != nil || !ok || token != "Secret.Token" {
		t.Fatalf("Load = %q, %v, %v", token, ok, err)
	}

	t.Run("the file and its directory are private", func(t *testing.T) {
		info, err := os.Stat(store.Path())
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("file mode = %o, want 600", perm)
		}
		dir, err := os.Stat(filepath.Dir(store.Path()))
		if err != nil {
			t.Fatal(err)
		}
		if perm := dir.Mode().Perm(); perm != 0o700 {
			t.Fatalf("dir mode = %o, want 700", perm)
		}
	})

	t.Run("a save replaces the token", func(t *testing.T) {
		if err := store.Save(t.Context(), "Other"); err != nil {
			t.Fatal(err)
		}
		if token, _, _ := store.Load(t.Context()); token != "Other" {
			t.Fatalf("token = %q", token)
		}
	})

	t.Run("no temporary files are left", func(t *testing.T) {
		entries, err := os.ReadDir(filepath.Dir(store.Path()))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("entries = %v, want the token file alone", entries)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("no file is no token, without an error", func(t *testing.T) {
		if token, ok, err := newStore(t, "cat", "cat").Load(t.Context()); err != nil || ok || token != "" {
			t.Fatalf("Load = %q, %v, %v", token, ok, err)
		}
	})

	write := func(t *testing.T, store Store, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.Path(), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		store := newStore(t, "cat", "cat")
		write(t, store, "  tok\n")
		if token, ok, err := store.Load(t.Context()); err != nil || !ok || token != "tok" {
			t.Fatalf("Load = %q, %v, %v", token, ok, err)
		}
	})

	for _, tt := range []struct{ name, decrypt, content string }{
		{"a failing decrypt", "false", "x"},
		{"a missing program", "amitty-no-such-program", "x"},
		{"an empty token", "cat", " \n"},
	} {
		t.Run(tt.name+" is an error, and the file is kept", func(t *testing.T) {
			store := newStore(t, "cat", tt.decrypt)
			write(t, store, tt.content)
			if _, ok, err := store.Load(t.Context()); err == nil || ok {
				t.Fatalf("ok = %v, err = %v", ok, err)
			}
			if _, err := os.Stat(store.Path()); err != nil {
				t.Fatalf("file gone: %v", err)
			}
		})
	}

	t.Run("a canceled context stops the command", func(t *testing.T) {
		store := newStore(t, "cat", "sleep 60")
		write(t, store, "x")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, _, err := store.Load(ctx); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("an unreadable path is an error", func(t *testing.T) {
		store := newStore(t, "cat", "cat")
		// A directory in place of the file opens but cannot be read as one.
		if err := os.MkdirAll(store.Path(), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, _, err := store.Load(t.Context()); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("the zero store is an error", func(t *testing.T) {
		if _, _, err := (Store{}).Load(t.Context()); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestSave(t *testing.T) {
	for _, tt := range []struct{ name, encrypt, token string }{
		{"a failing encrypt", "false", "t"},
		{"an encrypt that writes nothing", "true", "t"},
		{"an empty token", "cat", ""},
	} {
		t.Run(tt.name+" keeps the old token", func(t *testing.T) {
			store := newStore(t, "cat", "cat")
			if err := store.Save(t.Context(), "old"); err != nil {
				t.Fatal(err)
			}
			broken, err := New(store.Path(), tt.encrypt, "cat")
			if err != nil {
				t.Fatal(err)
			}
			if err := broken.Save(t.Context(), tt.token); err == nil {
				t.Fatal("want an error")
			}
			if token, _, _ := store.Load(t.Context()); token != "old" {
				t.Fatalf("token = %q, want the old one", token)
			}
		})
	}

	t.Run("the encrypt command's stderr is in the error", func(t *testing.T) {
		store := newStore(t, `sh -c "echo no public key >&2; exit 2"`, "cat")
		if err := store.Save(t.Context(), "t"); err == nil || !strings.Contains(err.Error(), "no public key") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("the zero store is an error", func(t *testing.T) {
		if err := (Store{}).Save(t.Context(), "t"); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestDelete(t *testing.T) {
	store := newStore(t, "cat", "cat")
	if err := store.Save(t.Context(), "t"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file kept: %v", err)
	}
	if err := store.Delete(); err != nil {
		t.Fatalf("deleting again: %v", err)
	}
}
