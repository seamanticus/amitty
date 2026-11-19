// Package instance makes sure one amitty runs at a time, by holding a lock on a file for as long as the process lives.
package instance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// FileName is the lock file in the config directory.
const FileName = "amitty.lock"

// errRunning reports that another amitty holds the lock.
var errRunning = errors.New("amitty is already running")

// IsRunning reports whether err is Acquire's for a lock another process holds.
func IsRunning(err error) bool { return errors.Is(err, errRunning) }

// Lock is the held lock. The kernel releases it when the process exits, however it exits, so a crash leaves no stale lock.
type Lock struct{ file *os.File }

// Acquire takes the lock on the file at path, creating it and its directory, or an error IsRunning reports if another process holds it.
func Acquire(path string) (Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Lock{}, fmt.Errorf("create lock dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return Lock{}, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return Lock{}, fmt.Errorf("%w (lock %s)", errRunning, path)
		}
		return Lock{}, fmt.Errorf("lock %s: %w", path, err)
	}
	return Lock{file: file}, nil
}

// Release gives the lock up. The file stays, as removing it would let a starting process lock a file another is about to open.
func (l Lock) Release() error {
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}
