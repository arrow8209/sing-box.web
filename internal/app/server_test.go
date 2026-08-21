package app

import (
	"bytes"
	"context"
	"errors"
	"os"
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
	if defaultListenAddress != "172.16.1.54:50010" {
		t.Fatalf("default listen address=%q", defaultListenAddress)
	}
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

func TestNewServerRejectsSemanticManagedJSONWithPath(t *testing.T) {
	tests := []struct {
		name     string
		target   func(paths.Layout) string
		contents string
	}{
		{name: "auth", target: func(layout paths.Layout) string { return layout.AuthFile }, contents: `{"version":1,"passwordHash":"invalid","sessionKey":"invalid","sessionGeneration":1}`},
		{name: "profiles null", target: func(layout paths.Layout) string { return layout.ProfilesFile }, contents: `{"version":1,"profiles":null}`},
		{name: "profiles invalid id", target: func(layout paths.Layout) string { return layout.ProfilesFile }, contents: `{"version":1,"profiles":[{"id":"../escape","name":"x","draftRevision":1,"appliedRevision":null,"appliedDigest":"","appliedResourceDigests":{},"appliedAt":null,"needsApply":true,"lastValidation":null}]}`},
		{name: "rules null", target: func(layout paths.Layout) string { return layout.RulesFile }, contents: `{"resources":null}`},
		{name: "core syntax", target: func(layout paths.Layout) string { return layout.CoreFile }, contents: `{`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workDir := t.TempDir()
			layout, err := paths.Resolve(workDir)
			if err != nil {
				t.Fatal(err)
			}
			if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
				t.Fatal(err)
			}
			target := test.target(layout)
			if err := storage.WriteFile(target, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = NewServer(workDir, Dependencies{})
			if err == nil || !strings.Contains(err.Error(), filepath.Clean(target)) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestNewServerCompletesReadOnlyGatesBeforeProfileRepair(t *testing.T) {
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 32)
	profiles := []byte(`{"version":1,"profiles":[{"id":"` + id + `","name":"x","draftRevision":1,"appliedRevision":null,"appliedDigest":"","appliedResourceDigests":{},"appliedAt":null,"needsApply":true,"lastValidation":null}]}`)
	draft := []byte(`{"profileId":"` + id + `","revision":2,"config":{},"buffers":{},"bindings":[],"savedAt":"1970-01-01T00:00:01Z"}`)
	if err := storage.WriteFile(layout.ProfilesFile, profiles, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(filepath.Join(layout.DraftsDir, id+".json"), draft, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(layout.CoreFile, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = NewServer(workDir, Dependencies{})
	if err == nil || !strings.Contains(err.Error(), layout.CoreFile) {
		t.Fatalf("err=%v", err)
	}
	after, err := os.ReadFile(layout.ProfilesFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, profiles) {
		t.Fatalf("profiles changed before core gate: %s", after)
	}
}

func TestNewServerRecoversDeletingDraftWhileHoldingApplicationLock(t *testing.T) {
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("c", 32)
	profiles := []byte(`{"version":1,"profiles":[{"id":"` + id + `","name":"恢复","draftRevision":1,"appliedRevision":null,"appliedDigest":"","appliedResourceDigests":{},"appliedAt":null,"needsApply":true,"lastValidation":null}]}`)
	draft := []byte(`{"profileId":"` + id + `","revision":1,"config":{},"buffers":{},"bindings":[],"savedAt":"1970-01-01T00:00:01Z"}`)
	if err := storage.WriteFile(layout.ProfilesFile, profiles, 0o600); err != nil {
		t.Fatal(err)
	}
	stagedPath := filepath.Join(layout.DraftsDir, id+".json.deleting")
	if err := storage.WriteFile(stagedPath, draft, 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(workDir, Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	if _, err := os.Stat(filepath.Join(layout.DraftsDir, id+".json")); err != nil {
		t.Fatalf("draft was not restored: %v", err)
	}
	if _, err := os.Stat(stagedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged draft remains: %v", err)
	}
}
