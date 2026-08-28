package application_test

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/application"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

func TestSwapRestoreReturnsOldConfig(t *testing.T) {
	layout := newLayoutWithConfig(t, "old")
	staging := newStaging(t, layout, "new")
	expected, err := application.DigestDirectory(layout.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := application.BeginDirectorySwap(layout, staging, expected)
	if err != nil {
		t.Fatal(err)
	}
	if got := readMarker(t, layout.ConfigDir); got != "new" {
		t.Fatalf("after swap=%q", got)
	}
	if err := transaction.Restore(); err != nil {
		t.Fatal(err)
	}
	if got := readMarker(t, layout.ConfigDir); got != "old" {
		t.Fatalf("after restore=%q", got)
	}
	transaction.Cleanup()
	assertNoTransactionDirs(t, layout.WorkDir)
}

func TestSwapCommitKeepsNewConfigAndRemovesRollback(t *testing.T) {
	layout := newLayoutWithConfig(t, "old")
	staging := newStaging(t, layout, "new")
	expected, err := application.DigestDirectory(layout.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := application.BeginDirectorySwap(layout, staging, expected)
	if err != nil {
		t.Fatal(err)
	}
	transaction.Commit()
	transaction.Cleanup()
	if got := readMarker(t, layout.ConfigDir); got != "new" {
		t.Fatalf("after commit=%q", got)
	}
	assertNoTransactionDirs(t, layout.WorkDir)
}

func TestSwapRejectsChangedConfigWithoutReplacingIt(t *testing.T) {
	layout := newLayoutWithConfig(t, "old")
	staging := newStaging(t, layout, "new")
	expected, err := application.DigestDirectory(layout.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "marker"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := application.BeginDirectorySwap(layout, staging, expected); err == nil {
		t.Fatal("changed config was accepted")
	}
	if got := readMarker(t, layout.ConfigDir); got != "changed" {
		t.Fatalf("config overwritten: %q", got)
	}
}

func TestRestoreRetainsNewExternalDrift(t *testing.T) {
	layout := newLayoutWithConfig(t, "old")
	staging := newStaging(t, layout, "new")
	expected, err := application.DigestDirectory(layout.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := application.BeginDirectorySwap(layout, staging, expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "marker"), []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Restore(); err == nil {
		t.Fatal("external drift was overwritten")
	}
	transaction.Cleanup()
	if got := readMarker(t, layout.ConfigDir); got != "external" {
		t.Fatalf("config overwritten: %q", got)
	}
}

func newLayoutWithConfig(t *testing.T, marker string) paths.Layout {
	t.Helper()
	root := t.TempDir()
	layout, err := paths.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "marker"), []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
	return layout
}

func newStaging(t *testing.T, layout paths.Layout, marker string) string {
	t.Helper()
	staging, err := layout.StagingConfigDir("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "marker"), []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
	return staging
}

func readMarker(t *testing.T, directory string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(directory, "marker"))
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func assertNoTransactionDirs(t *testing.T, workDir string) {
	t.Helper()
	directories, err := storage.ListTransactionDirs(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(directories) != 0 {
		t.Fatalf("transaction dirs=%v", directories)
	}
}
