//go:build linux

package storage_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/storage"
)

func TestAcquireInstanceLockRejectsSecondOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "app.lock")
	first, err := storage.AcquireInstanceLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := storage.AcquireInstanceLock(path); !errors.Is(err, storage.ErrAlreadyLocked) {
		t.Fatalf("second lock error = %v", err)
	}
}

func TestAcquireInstanceLockRejectsSymlinkedLockFile(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "app.lock"), filepath.Join(root, "data", "app.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.AcquireInstanceLock(filepath.Join(root, "data", "app.lock")); err == nil {
		t.Fatal("symlinked lock file was accepted")
	}
}
