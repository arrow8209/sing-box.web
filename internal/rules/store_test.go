package rules

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/paths"
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
	legacy := &Store{file: layout.RulesFile, layout: layout, clock: fixedClock(now)}
	if err := legacy.write([]Resource{{ID: id, SourceType: SourceRemote, Name: "legacy", URL: "https://example.test/legacy.srs", Format: FormatSource, UpdateInterval: "24h", LocalPath: sourcePath, LastAttemptAt: &now, LastSuccessAt: &now, LastDigest: strings.Repeat("a", 64), LastError: "old", NextUpdateAt: &now, RuleCount: &count}}); err != nil {
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
