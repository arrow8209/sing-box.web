package core

import (
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/paths"
)

func writeVersionProgram(t *testing.T, path, version string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	program := "#!/bin/sh\nif [ \"$1\" = version ]; then echo 'sing-box version " + version + "'; exit 0; fi\nexit 2\n"
	if err := os.WriteFile(path, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverInstallationRestoresBackupAndCalibrates(t *testing.T) {
	work := t.TempDir()
	layout, err := paths.Resolve(work)
	if err != nil {
		t.Fatal(err)
	}
	writeVersionProgram(t, layout.CoreBackup, "1.2.3")
	store := NewStore(layout.CoreFile)
	state, err := RecoverInstallation(layout, store)
	if err != nil {
		t.Fatal(err)
	}
	if state.InstalledVersion != "1.2.3" || state.InstalledDigest == "" {
		t.Fatalf("state=%+v", state)
	}
	if _, err := os.Stat(layout.CoreBinary); err != nil {
		t.Fatalf("backup not restored: %v", err)
	}
	if _, err := os.Stat(layout.CoreBackup); !os.IsNotExist(err) {
		t.Fatalf("backup remains: %v", err)
	}
}

func TestRecoverInstallationRefusesUnknownPair(t *testing.T) {
	work := t.TempDir()
	layout, err := paths.Resolve(work)
	if err != nil {
		t.Fatal(err)
	}
	writeVersionProgram(t, layout.CoreBinary, "1.2.3")
	writeVersionProgram(t, layout.CoreBackup, "1.2.4")
	store := NewStore(layout.CoreFile)
	if err := store.RecordInstalled("1.0.0", "0000"); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverInstallation(layout, store); err == nil {
		t.Fatal("expected ambiguity refusal")
	}
}
