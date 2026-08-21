package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/storage"
)

func TestListTransactionDirsListsOnlyDirectMatchingDirectories(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"config.staging-deadbeef", "config.rollback-cafe", "config.staging-invalid!", "other"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "nested", "config.staging-deadbeef"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := storage.ListTransactionDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != filepath.Join(root, "config.rollback-cafe") || got[1] != filepath.Join(root, "config.staging-deadbeef") {
		t.Fatalf("dirs = %v", got)
	}
}

func TestListTransactionDirsRejectsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "config.staging-deadbeef")); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ListTransactionDirs(root); err == nil {
		t.Fatal("symlinked transaction directory was accepted")
	}
}
