package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/auth"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

func TestNewServerRefusesMissingPasswordAndReleasesLock(t *testing.T) {
	workDir := t.TempDir()
	_, err := NewServer(workDir, Dependencies{})
	if !errors.Is(err, auth.ErrNotInitialized) || !strings.Contains(err.Error(), "singbox-web admin set-password") {
		t.Fatalf("err=%v", err)
	}
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := storage.AcquireInstanceLock(layout.AppLock)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestServerCloseReleasesExclusiveLock(t *testing.T) {
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(workDir, Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if server.http.Addr != defaultListenAddress {
		t.Fatalf("addr=%q", server.http.Addr)
	}
	if err := server.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	lock, err := storage.AcquireInstanceLock(layout.AppLock)
	if err != nil {
		t.Fatalf("lock remains held: %v", err)
	}
	defer lock.Close()
}

func TestNewServerReportsCorruptProfilePath(t *testing.T) {
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(layout.ProfilesFile, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = NewServer(workDir, Dependencies{})
	if err == nil || !strings.Contains(err.Error(), filepath.Clean(layout.ProfilesFile)) {
		t.Fatalf("err=%v", err)
	}
}

func TestNewServerReportsCorruptRulesPath(t *testing.T) {
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(layout.RulesFile, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = NewServer(workDir, Dependencies{})
	if err == nil || !strings.Contains(err.Error(), filepath.Clean(layout.RulesFile)) {
		t.Fatalf("err=%v", err)
	}
}
