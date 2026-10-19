package rules

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

func fixedClock(now time.Time) func() time.Time { return func() time.Time { return now } }

func TestEnsureTemplateResourcesAssignsRandomStableIDs(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(1, 0)))
	if err != nil {
		t.Fatal(err)
	}
	templates := []ResourceTemplate{{TemplateKey: "template-cn", Name: "GeoSite-CN", URL: "https://example.test/cn.srs", Format: FormatBinary}}
	first, err := store.EnsureTemplateResources(context.Background(), templates, bytes.NewReader(bytes.Repeat([]byte{0x11}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(2, 0)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := reopened.EnsureTemplateResources(context.Background(), templates, bytes.NewReader(bytes.Repeat([]byte{0x22}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if first["template-cn"] == "" || first["template-cn"] != second["template-cn"] {
		t.Fatalf("ids first=%v second=%v", first, second)
	}
	persisted, err := storage.ReadJSON[document](layout.RulesFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Resources) != 1 || persisted.Resources[0].LocalPath != "data/rulesets/"+first["template-cn"]+".srs" {
		t.Fatalf("persisted=%+v", persisted)
	}
}

func TestOpenStoreRebasesLegacyPathsAndPreservesMetadata(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	oldDir := t.TempDir()
	now := time.Unix(10, 0).UTC()
	count := 1
	legacy := document{Resources: []Resource{
		{ID: strings.Repeat("a", 32), TemplateKey: "template-cn", CatalogKey: "geosite:cn", SourceType: SourceRemote, Name: "remote", URL: "https://example.test/cn.srs", Format: FormatBinary, UpdateInterval: "24h", LastAttemptAt: &now, LastSuccessAt: &now, LastDigest: strings.Repeat("c", 64), LastError: "previous error", NextUpdateAt: &now, RuleCount: &count},
		{ID: strings.Repeat("b", 32), SourceType: SourceManual, Name: "manual", Format: FormatSource, Disabled: true, LastAttemptAt: &now, LastSuccessAt: &now, LastDigest: strings.Repeat("d", 64), RuleCount: &count},
	}}
	for index := range legacy.Resources {
		resource := &legacy.Resources[index]
		resource.LocalPath = filepath.Join(oldDir, "data", "rulesets", resource.ID+extension(resource.Format))
		if err := storage.WriteFile(resource.LocalPath, []byte("old location"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := storage.WriteJSON(layout.RulesFile, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(layout.RulesFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateManagedState(layout.RulesFile, layout); err != nil {
		t.Fatal(err)
	}
	afterGate, err := os.ReadFile(layout.RulesFile)
	if err != nil || !bytes.Equal(afterGate, original) {
		t.Fatalf("read-only gate changed metadata: %q err=%v", afterGate, err)
	}
	assertPathMigrationBackups(t, layout.RulesFile, 0)
	store, err := OpenStore(layout.RulesFile, layout, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	resources, err := store.list()
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := storage.ReadJSON[document](layout.RulesFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != len(legacy.Resources) || len(persisted.Resources) != len(legacy.Resources) {
		t.Fatalf("resources=%+v persisted=%+v", resources, persisted)
	}
	for index, before := range legacy.Resources {
		absolute, relative, err := layout.RuleSetPath(before.ID, extension(before.Format))
		if err != nil {
			t.Fatal(err)
		}
		want := before
		want.LocalPath = absolute
		if !reflect.DeepEqual(resources[index], want) {
			t.Fatalf("runtime resource=%+v want=%+v", resources[index], want)
		}
		want.LocalPath = relative
		if !reflect.DeepEqual(persisted.Resources[index], want) {
			t.Fatalf("persisted resource=%+v want=%+v", persisted.Resources[index], want)
		}
		old, err := os.ReadFile(before.LocalPath)
		if err != nil || string(old) != "old location" {
			t.Fatalf("old file changed: %q err=%v", old, err)
		}
	}
	backups := assertPathMigrationBackups(t, layout.RulesFile, 1)
	backup, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup does not preserve original bytes: %q err=%v", backup, err)
	}
	info, err := os.Stat(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("backup permissions=%v", info.Mode().Perm())
	}
	if _, err := OpenStore(layout.RulesFile, layout, fixedClock(now)); err != nil {
		t.Fatal(err)
	}
	assertPathMigrationBackups(t, layout.RulesFile, 1)
}

func TestOpenStoreRejectsForgedPathsWithoutRewriting(t *testing.T) {
	id := strings.Repeat("a", 32)
	relative := "data/rulesets/" + id + ".srs"
	for _, path := range []string{
		"../" + relative,
		"./" + relative,
		"data/rulesets/../rulesets/" + id + ".srs",
		"/old/../old/" + relative,
		"/old//" + relative,
		"/old/not-data/rulesets/" + id + ".srs",
		"/old/data/rulesets/" + strings.Repeat("b", 32) + ".srs",
		"/old/data/rulesets/" + id + ".json",
		"/old/\x00/" + relative,
		"",
	} {
		t.Run(path, func(t *testing.T) {
			layout, err := paths.Resolve(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			data := document{Resources: []Resource{{ID: id, Name: "forged", URL: "https://example.test/x.srs", Format: FormatBinary, LocalPath: path}}}
			if err := storage.WriteJSON(layout.RulesFile, data, 0o600); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(layout.RulesFile)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := OpenStore(layout.RulesFile, layout, nil); err == nil || !strings.Contains(err.Error(), layout.RulesFile) {
				t.Fatalf("forged path accepted or file missing from error: %v", err)
			}
			after, err := os.ReadFile(layout.RulesFile)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatalf("invalid metadata rewritten: %q err=%v", after, err)
			}
			assertPathMigrationBackups(t, layout.RulesFile, 0)
		})
	}
}

func assertPathMigrationBackups(t *testing.T, file string, want int) []string {
	t.Helper()
	backups, err := filepath.Glob(file + ".before-relative-paths-*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != want {
		t.Fatalf("migration backups=%v want %d", backups, want)
	}
	return backups
}

func TestOpenStoreMigratesRemoteSourceToBinaryAndDeletesPayload(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("c", 32)
	sourcePath, _, err := layout.RuleSetPath(id, ".json")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(10, 0)
	count := 2
	oldSource := filepath.Join(t.TempDir(), "data", "rulesets", id+".json")
	legacy := document{Resources: []Resource{{ID: id, SourceType: SourceRemote, Name: "legacy", URL: "https://example.test/legacy.srs", Format: FormatSource, UpdateInterval: "24h", LocalPath: oldSource, LastAttemptAt: &now, LastSuccessAt: &now, LastDigest: strings.Repeat("a", 64), LastError: "old", NextUpdateAt: &now, RuleCount: &count}}}
	if err := storage.WriteJSON(layout.RulesFile, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(oldSource, []byte("old directory must remain untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.RuleSetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte(`{"version":3,"rules":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(layout.RulesFile, layout, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	resources, err := store.list()
	if err != nil || len(resources) != 1 {
		t.Fatalf("resources=%+v err=%v", resources, err)
	}
	resource := resources[0]
	if resource.Format != FormatBinary || resource.LocalPath != filepath.Join(layout.RuleSetsDir, id+".srs") || resource.LastAttemptAt != nil || resource.LastSuccessAt != nil || resource.LastDigest != "" || resource.LastError != "" || resource.NextUpdateAt != nil || resource.RuleCount != nil {
		t.Fatalf("resource=%+v", resource)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("old source remains: %v", err)
	}
	if old, err := os.ReadFile(oldSource); err != nil || string(old) != "old directory must remain untouched" {
		t.Fatalf("old directory source changed: %q err=%v", old, err)
	}
	persisted, err := storage.ReadJSON[document](layout.RulesFile)
	if err != nil || persisted.Resources[0].LocalPath != "data/rulesets/"+id+".srs" {
		t.Fatalf("persisted=%+v err=%v", persisted, err)
	}
}

func TestValidateManagedStateRejectsInvalidIDsDuplicatesAndDerivedPaths(t *testing.T) {
	for _, test := range []struct {
		name string
		body func(paths.Layout) string
	}{
		{name: "invalid id", body: func(layout paths.Layout) string {
			return `{"resources":[{"id":"../escape","name":"x","url":"https://example.test/x","format":"binary","updateInterval":"","disabled":false,"localPath":"` + filepath.Join(layout.RuleSetsDir, "escape.srs") + `"}]}`
		}},
		{name: "duplicate", body: func(layout paths.Layout) string {
			id := strings.Repeat("a", 32)
			path := filepath.Join(layout.RuleSetsDir, id+".srs")
			return `{"resources":[{"id":"` + id + `","name":"x","url":"https://example.test/x","format":"binary","updateInterval":"","disabled":false,"localPath":"` + path + `"},{"id":"` + id + `","name":"y","url":"https://example.test/y","format":"binary","updateInterval":"","disabled":false,"localPath":"` + path + `"}]}`
		}},
		{name: "forged path", body: func(layout paths.Layout) string {
			id := strings.Repeat("b", 32)
			return `{"resources":[{"id":"` + id + `","name":"x","url":"https://example.test/x","format":"binary","updateInterval":"","disabled":false,"localPath":"` + filepath.Join(layout.DataDir, "elsewhere.srs") + `"}]}`
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			layout, err := paths.Resolve(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(layout.RulesFile), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(layout.RulesFile, []byte(test.body(layout)), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ValidateManagedState(layout.RulesFile, layout); err == nil || !strings.Contains(err.Error(), layout.RulesFile) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
