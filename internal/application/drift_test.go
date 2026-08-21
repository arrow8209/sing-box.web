package application_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/application"
)

func TestCheckDriftRequiresExactCurrentDigestConfirmation(t *testing.T) {
	dir := writeConfigDirectory(t, map[string]string{"10-log.json": "{\"log\":{\"level\":\"info\"}}\n"})
	expected, err := application.DigestDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "10-log.json"), []byte("{\"log\":{\"level\":\"debug\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	current, err := application.DigestDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	result, err := application.CheckDrift(expected, "", current)
	if !errors.Is(err, application.ErrExternalDrift) || result.ConfirmationDigest != current {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := application.CheckDrift(expected, current, current); err != nil {
		t.Fatalf("confirmation rejected: %v", err)
	}
}

func TestDigestDirectoryDoesNotFollowSymlinks(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}
	first, err := application.DigestDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := application.DigestDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("symlink target influenced digest: %s != %s", first, second)
	}
}

func TestDigestDirectoryMissingIsEmpty(t *testing.T) {
	digest, err := application.DigestDirectory(filepath.Join(t.TempDir(), "missing"))
	if err != nil || digest != "" {
		t.Fatalf("digest=%q err=%v", digest, err)
	}
}

func writeConfigDirectory(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
