package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/auth"
	"singbox-web/internal/paths"
	"singbox-web/internal/rules"
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
	if server.http.Addr != "0.0.0.0:50022" {
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
		{name: "subscriptions null", target: func(layout paths.Layout) string { return layout.SubscriptionsFile }, contents: `{"version":1,"subscriptions":null}`},
		{name: "inbounds null", target: func(layout paths.Layout) string { return layout.InboundResourcesFile }, contents: `{"version":1,"inbounds":null}`},
		{name: "core syntax", target: func(layout paths.Layout) string { return layout.CoreFile }, contents: `{`},
		{name: "settings invalid proxy", target: func(layout paths.Layout) string { return layout.SettingsFile }, contents: `{"version":1,"proxyUrl":"socks5://127.0.0.1:1080"}`},
		{name: "settings invalid web port", target: func(layout paths.Layout) string { return layout.SettingsFile }, contents: `{"version":1,"proxyUrl":"","webPort":0}`},
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

func TestNewServerStartsAfterWorkingDirectoryMove(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "relative paths"
		if legacy {
			name = "legacy absolute paths"
		}
		t.Run(name, func(t *testing.T) {
			layout, err := paths.Resolve(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
				t.Fatal(err)
			}
			id := strings.Repeat("e", 32)
			absolute, relative, err := layout.RuleSetPath(id, ".json")
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte(`{"version":3,"rules":[{"domain_suffix":["example.test"]}]}`)
			if err := storage.WriteFile(absolute, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1, 0).UTC()
			resource := rules.Resource{ID: id, SourceType: rules.SourceManual, Name: "manual", Format: rules.FormatSource, LocalPath: relative, LastSuccessAt: &now, LastDigest: fmt.Sprintf("%x", sha256.Sum256(payload))}
			if legacy {
				resource.LocalPath = absolute
			}
			data := struct {
				Resources []rules.Resource `json:"resources"`
			}{Resources: []rules.Resource{resource}}
			if err := storage.WriteJSON(layout.RulesFile, data, 0o600); err != nil {
				t.Fatal(err)
			}
			movedDir := filepath.Join(t.TempDir(), "moved")
			if err := os.Rename(layout.WorkDir, movedDir); err != nil {
				t.Fatal(err)
			}
			server, err := NewServer(movedDir, Dependencies{ListenAddress: "127.0.0.1:0"})
			if err != nil {
				t.Fatalf("moved server did not start: %v", err)
			}
			defer server.Close(context.Background())
			persisted, err := storage.ReadJSON[struct {
				Resources []rules.Resource `json:"resources"`
			}](server.layout.RulesFile)
			if err != nil || len(persisted.Resources) != 1 || persisted.Resources[0].LocalPath != relative || persisted.Resources[0].LastDigest != resource.LastDigest {
				t.Fatalf("persisted=%+v err=%v", persisted, err)
			}
			movedPayload, err := os.ReadFile(filepath.Join(server.layout.RuleSetsDir, id+".json"))
			if err != nil || !bytes.Equal(movedPayload, payload) {
				t.Fatalf("manual rules changed: %q err=%v", movedPayload, err)
			}
		})
	}
}

func TestNewServerCompletesReadOnlyGatesBeforeRulePathMigration(t *testing.T) {
	for _, test := range []struct {
		name     string
		target   func(paths.Layout) string
		contents string
	}{
		{name: "core", target: func(layout paths.Layout) string { return layout.CoreFile }, contents: "{"},
		{name: "settings", target: func(layout paths.Layout) string { return layout.SettingsFile }, contents: `{"version":1,"proxyUrl":"socks5://127.0.0.1:1080"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout, err := paths.Resolve(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
				t.Fatal(err)
			}
			id := strings.Repeat("f", 32)
			legacy := []byte(`{"resources":[{"id":"` + id + `","name":"legacy","url":"https://example.test/cn.srs","format":"binary","updateInterval":"","disabled":false,"localPath":"/old/workdir/data/rulesets/` + id + `.srs"}]}`)
			if err := storage.WriteFile(layout.RulesFile, legacy, 0o600); err != nil {
				t.Fatal(err)
			}
			target := test.target(layout)
			if err := storage.WriteFile(target, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = NewServer(layout.WorkDir, Dependencies{ListenAddress: "127.0.0.1:0"})
			if err == nil || !strings.Contains(err.Error(), target) {
				t.Fatalf("err=%v want corrupt file %q", err, target)
			}
			after, err := os.ReadFile(layout.RulesFile)
			if err != nil || !bytes.Equal(after, legacy) {
				t.Fatalf("rules changed before gates completed: %q err=%v", after, err)
			}
			backups, err := filepath.Glob(layout.RulesFile + ".before-relative-paths-*.bak")
			if err != nil || len(backups) != 0 {
				t.Fatalf("backup created before gates completed: %v err=%v", backups, err)
			}
		})
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
