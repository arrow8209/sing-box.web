//go:build linux

package storage_test

import (
	"errors"
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
