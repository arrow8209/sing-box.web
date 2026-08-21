package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/paths"
)

func TestResolveUsesOnlyFixedRelativePaths(t *testing.T) {
	root := t.TempDir()
	layout, err := paths.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	assertPath(t, layout.DataDir, filepath.Join(root, "data"))
	assertPath(t, layout.ConfigDir, filepath.Join(root, "config"))
	assertPath(t, layout.CoreBinary, filepath.Join(root, "data", "bin", "sing-box"))
	assertPath(t, layout.AppLock, filepath.Join(root, "data", "app.lock"))
}

func TestResolveRejectsManagedSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "data")); err != nil {
		t.Fatal(err)
	}
	if _, err := paths.Resolve(root); err == nil {
		t.Fatal("managed data symlink was accepted")
	}
}

func assertPath(t *testing.T, actual, expected string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("path = %q, want %q", actual, expected)
	}
}
