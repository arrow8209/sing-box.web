//go:build linux

package atomicfile

import (
	"path/filepath"
	"testing"
)

func TestAcquireRejectsSecondOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := Acquire(path); err == nil {
		second.Close()
		t.Fatal("second Acquire() succeeded")
	}
}
