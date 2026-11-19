package instance

import (
	"path/filepath"
	"testing"
)

func TestAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "amitty", FileName)
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	// A second open file description conflicts with the first, as a second process would.
	if _, err := Acquire(path); !IsRunning(err) {
		t.Fatalf("second Acquire = %v, want it running", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	again, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire after Release = %v", err)
	}
	if err := again.Release(); err != nil {
		t.Fatal(err)
	}
	if err := (Lock{}).Release(); err != nil {
		t.Fatalf("zero Release = %v", err)
	}
}
