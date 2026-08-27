package application

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/paths"
)

func TestRecoverConfigTransactionsRestoresCommittedRollback(t *testing.T) {
	work := t.TempDir()
	layout, err := paths.Resolve(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "new.json"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	rollback := filepath.Join(work, "config.rollback-0123456789abcdef0123456789abcdef")
	if err := os.Mkdir(rollback, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rollback, "old.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := DigestDirectory(rollback)
	if err != nil {
		t.Fatal(err)
	}
	if err := RecoverConfigTransactions(layout, digest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(layout.ConfigDir, "old.json")); err != nil {
		t.Fatalf("rollback not restored: %v", err)
	}
}

func TestRecoverConfigTransactionsPreservesProfileStagingDirectories(t *testing.T) {
	work := t.TempDir()
	layout, err := paths.Resolve(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.staging-0123456789abcdef0123456789abcdef", "config.staging-fedcba9876543210fedcba9876543210"} {
		if err := os.Mkdir(filepath.Join(work, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := RecoverConfigTransactions(layout, ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.staging-0123456789abcdef0123456789abcdef", "config.staging-fedcba9876543210fedcba9876543210"} {
		if _, err := os.Stat(filepath.Join(work, name)); err != nil {
			t.Fatalf("persistent staging removed %s: %v", name, err)
		}
	}
}
