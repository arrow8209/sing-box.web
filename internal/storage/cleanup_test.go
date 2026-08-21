package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/storage"
)

func TestListTransactionDirsListsOnlyDirectMatchingDirectories(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"config.staging-0123456789abcdef0123456789abcdef", "config.rollback-fedcba9876543210fedcba9876543210", "config.staging-invalid!", "other"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "nested", "config.staging-0123456789abcdef0123456789abcdef"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := storage.ListTransactionDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != filepath.Join(root, "config.rollback-fedcba9876543210fedcba9876543210") || got[1] != filepath.Join(root, "config.staging-0123456789abcdef0123456789abcdef") {
		t.Fatalf("dirs = %v", got)
	}
}

func TestListTransactionDirsRejectsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "config.staging-0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ListTransactionDirs(root); err == nil {
		t.Fatal("symlinked transaction directory was accepted")
	}
}
