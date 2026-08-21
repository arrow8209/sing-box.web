package rules

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"singbox-web/internal/paths"
)

type failingRuleDownloader struct{ err error }

func (d failingRuleDownloader) Fetch(context.Context, Resource) (DownloadedFile, error) {
	return DownloadedFile{}, d.err
}

func TestFailedUpdatePreservesCurrentFile(t *testing.T) {
	service, layout := newRuleService(t, nil)
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	target, _, err := layout.RuleSetPath(resource.ID, ".srs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.RuleSetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := []byte("old-binary")
	if err := os.WriteFile(target, old, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(old)
	items, err := service.store.list()
	if err != nil {
		t.Fatal(err)
	}
	items[0].LastSuccessAt = ptrTime(time.Unix(1, 0))
	items[0].LastDigest = fmt.Sprintf("%x", sum)
	if err := service.store.write(items); err != nil {
		t.Fatal(err)
	}
	service.SetDownloader(failingRuleDownloader{errors.New("response exceeded 64 MiB")})
	if _, err := service.UpdateNow(context.Background(), resource.ID); err == nil {
		t.Fatal("update succeeded")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(old) {
		t.Fatalf("current file=%q err=%v", got, err)
	}
	updated, err := service.store.list()
	if err != nil {
		t.Fatal(err)
	}
	if updated[0].LastError == "" || updated[0].LastSuccessAt == nil {
		t.Fatalf("status=%+v", updated[0])
	}
}

type referenceSource struct{ references map[string][]Reference }

func (s referenceSource) ReferencesForResource(_ context.Context, id string) ([]Reference, error) {
	return append([]Reference(nil), s.references[id]...), nil
}

func newRuleService(t *testing.T, references map[string][]Reference) (*Service, paths.Layout) {
	t.Helper()
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(1, 0)))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, layout, referenceSource{references}, fixedClock(time.Unix(1, 0)), bytes.NewReader(bytes.Repeat([]byte{0x11}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	return service, layout
}

func TestDeleteReferencedResourceListsProfiles(t *testing.T) {
	service, _ := newRuleService(t, map[string][]Reference{"11111111111111111111111111111111": {{ID: "profile-a", Name: "家庭出口"}}})
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	// The deterministic reader generates 11..., which makes the fixture above
	// bind the resource created by this test.
	err = service.Delete(context.Background(), resource.ID)
	var referenced *ReferencedError
	if !errors.As(err, &referenced) || len(referenced.Profiles) != 1 || referenced.Profiles[0].Name != "家庭出口" {
		t.Fatalf("err=%v", err)
	}
}

func TestPatchFormatMovesFixedPathAndResetsDownloadState(t *testing.T) {
	service, layout := newRuleService(t, nil)
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	old, _, err := layout.RuleSetPath(resource.ID, ".srs")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.RuleSetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	format := FormatSource
	updated, err := service.Patch(context.Background(), resource.ID, PatchInput{Format: &format})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LocalPath != "data/rulesets/"+resource.ID+".json" || updated.LastSuccessAt != nil || updated.LastDigest != "" {
		t.Fatalf("updated=%+v", updated)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old remains: %v", err)
	}
}

func TestCreateRejectsInvalidFields(t *testing.T) {
	service, _ := newRuleService(t, nil)
	if _, err := service.Create(context.Background(), CreateInput{Name: "", URL: "https://example.test/a", Format: FormatBinary}); err == nil {
		t.Fatal("accepted empty name")
	}
	if _, err := service.Create(context.Background(), CreateInput{Name: "a", URL: "file:///etc/passwd", Format: FormatBinary}); err == nil {
		t.Fatal("accepted file URL")
	}
}
