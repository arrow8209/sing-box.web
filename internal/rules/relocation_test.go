package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type relocationTransport struct{}

func (relocationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	payload := []byte("binary-rule-data")
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request}, nil
}

func TestRuleOperationsAfterWorkingDirectoryMove(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "relative paths"
		if legacy {
			name = "legacy absolute paths"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			service, layout := newRuleService(t, nil)
			downloader := NewDownloader(DownloadPolicyForTest(relocationTransport{}))
			service.SetDownloader(downloader)
			remote, err := service.Create(ctx, CreateInput{Name: "remote", URL: "https://example.test/rule.srs", Format: FormatBinary})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.UpdateNow(ctx, remote.ID); err != nil {
				t.Fatal(err)
			}
			service.random = bytes.NewReader(bytes.Repeat([]byte{0x22}, 16))
			manual, err := service.Create(ctx, CreateInput{SourceType: SourceManual, Name: "manual", Content: json.RawMessage(`{"version":3,"rules":[{"domain_suffix":["before.example"]}]}`)})
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				resources, err := service.store.list()
				if err != nil {
					t.Fatal(err)
				}
				// Encode the old on-disk representation without the new writer.
				if err := storage.WriteJSON(layout.RulesFile, document{Resources: resources}, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			oldDir := layout.WorkDir
			movedDir := filepath.Join(t.TempDir(), "moved")
			if err := os.Rename(oldDir, movedDir); err != nil {
				t.Fatal(err)
			}
			layout, err = paths.Resolve(movedDir)
			if err != nil {
				t.Fatal(err)
			}
			store, err := OpenStore(layout.RulesFile, layout, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := RecoverRuleTransactions(store, layout); err != nil {
				t.Fatal(err)
			}
			service, err = NewService(store, layout, referenceSource{}, time.Now, nil)
			if err != nil {
				t.Fatal(err)
			}
			service.SetDownloader(downloader)
			listed, err := service.List(ctx)
			if err != nil || len(listed) != 2 {
				t.Fatalf("list=%+v err=%v", listed, err)
			}
			for _, resource := range listed {
				if resource.SizeBytes == nil || *resource.SizeBytes == 0 {
					t.Fatalf("moved resource file not found: %+v", resource)
				}
			}
			detail, err := service.Get(ctx, manual.ID)
			if err != nil || !bytes.Contains(detail.Content, []byte("before.example")) {
				t.Fatalf("manual detail=%+v err=%v", detail, err)
			}
			snapshot, release, err := service.SnapshotForBindings(ctx, []RuleBinding{{ResourceID: remote.ID, Enabled: true}, {ResourceID: manual.ID, Enabled: true}})
			if err != nil {
				t.Fatal(err)
			}
			release()
			for id, resource := range snapshot.Resources {
				want, _, err := layout.RuleSetPath(id, extension(resource.Format))
				if err != nil || resource.LocalPath != want || snapshot.Digests[id] == "" {
					t.Fatalf("snapshot resource=%+v want=%q err=%v", resource, want, err)
				}
			}
			if _, err := service.UpdateNow(ctx, remote.ID); err != nil {
				t.Fatal(err)
			}
			content := json.RawMessage(`{"version":3,"rules":[{"domain_suffix":["after.example"]}]}`)
			if _, err := service.Patch(ctx, manual.ID, PatchInput{Content: &content}); err != nil {
				t.Fatal(err)
			}
			detail, err = service.Get(ctx, manual.ID)
			if err != nil || !bytes.Contains(detail.Content, []byte("after.example")) {
				t.Fatalf("updated manual detail=%+v err=%v", detail, err)
			}
			persisted, err := storage.ReadJSON[document](layout.RulesFile)
			if err != nil {
				t.Fatal(err)
			}
			for _, resource := range persisted.Resources {
				_, want, err := layout.RuleSetPath(resource.ID, extension(resource.Format))
				if err != nil || resource.LocalPath != want {
					t.Fatalf("persisted resource=%+v want=%q err=%v", resource, want, err)
				}
			}
			for _, id := range []string{remote.ID, manual.ID} {
				if err := service.Delete(ctx, id); err != nil {
					t.Fatal(err)
				}
			}
			files, err := os.ReadDir(layout.RuleSetsDir)
			if err != nil || len(files) != 0 {
				t.Fatalf("moved rule files not deleted: %v err=%v", files, err)
			}
			if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
				t.Fatalf("old directory was recreated: %v", err)
			}
			backups := 0
			if legacy {
				backups = 1
			}
			assertPathMigrationBackups(t, layout.RulesFile, backups)
		})
	}
}
