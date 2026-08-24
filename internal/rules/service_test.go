package rules

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
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

type controlledRuleDownloader struct {
	started  chan<- string
	release  map[string]<-chan struct{}
	failures map[string]error
}

func (d controlledRuleDownloader) Fetch(ctx context.Context, resource Resource) (DownloadedFile, error) {
	d.started <- resource.ID
	select {
	case <-ctx.Done():
		return DownloadedFile{}, ctx.Err()
	case <-d.release[resource.ID]:
	}
	if err := d.failures[resource.ID]; err != nil {
		return DownloadedFile{}, err
	}
	payload := []byte("binary-" + resource.ID)
	path, err := stageRulePayload(resource.LocalPath, payload)
	if err != nil {
		return DownloadedFile{}, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	return DownloadedFile{Path: path, Digest: digest, Payload: payload}, nil
}

func TestConcurrentUpdatesMergeLatestResourceStates(t *testing.T) {
	for _, test := range []struct {
		name          string
		secondFailure error
	}{
		{name: "both updates succeed"},
		{name: "later update fails", secondFailure: errors.New("download failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, layout := newRuleService(t, nil)
			first, err := service.Create(context.Background(), CreateInput{Name: "first", URL: "https://example.test/first.srs", Format: FormatBinary})
			if err != nil {
				t.Fatal(err)
			}
			service.random = bytes.NewReader(bytes.Repeat([]byte{0x22}, 16))
			second, err := service.Create(context.Background(), CreateInput{Name: "second", URL: "https://example.test/second.srs", Format: FormatBinary})
			if err != nil {
				t.Fatal(err)
			}
			started := make(chan string, 2)
			firstRelease, secondRelease := make(chan struct{}), make(chan struct{})
			service.SetDownloader(controlledRuleDownloader{
				started:  started,
				release:  map[string]<-chan struct{}{first.ID: firstRelease, second.ID: secondRelease},
				failures: map[string]error{second.ID: test.secondFailure},
			})
			type outcome struct {
				id  string
				err error
			}
			completed := make(chan outcome, 2)
			for _, id := range []string{first.ID, second.ID} {
				go func() {
					_, updateErr := service.UpdateNow(context.Background(), id)
					completed <- outcome{id: id, err: updateErr}
				}()
			}
			for range 2 {
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("concurrent updates did not both reach the downloader")
				}
			}
			close(firstRelease)
			select {
			case result := <-completed:
				if result.id != first.ID || result.err != nil {
					t.Fatalf("first update result=%+v", result)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("first update did not complete")
			}
			close(secondRelease)
			select {
			case result := <-completed:
				if result.id != second.ID || !errors.Is(result.err, test.secondFailure) {
					t.Fatalf("second update result=%+v", result)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("second update did not complete")
			}

			resources, err := service.store.list()
			if err != nil {
				t.Fatal(err)
			}
			firstIndex, secondIndex := indexOf(resources, first.ID), indexOf(resources, second.ID)
			if firstIndex < 0 || resources[firstIndex].LastSuccessAt == nil || resources[firstIndex].LastDigest == "" {
				t.Fatalf("first resource lost its successful update state: %+v", resources)
			}
			if test.secondFailure == nil && (secondIndex < 0 || resources[secondIndex].LastSuccessAt == nil || resources[secondIndex].LastDigest == "") {
				t.Fatalf("second resource lost its successful update state: %+v", resources)
			}
			if test.secondFailure != nil && (secondIndex < 0 || resources[secondIndex].LastError == "") {
				t.Fatalf("second resource lost its failed update state: %+v", resources)
			}
			if err := RecoverRuleTransactions(service.store, layout); err != nil {
				t.Fatalf("concurrent updates left an unrecoverable state: %v", err)
			}
		})
	}
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
	return newRuleServiceWithProfiles(t, referenceSource{references})
}

func newRuleServiceWithProfiles(t *testing.T, profiles ProfileReferences) (*Service, paths.Layout) {
	t.Helper()
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(1, 0)))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, layout, profiles, fixedClock(time.Unix(1, 0)), bytes.NewReader(bytes.Repeat([]byte{0x11}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	return service, layout
}

type changeTrackingReferences struct{ changed []string }

func (*changeTrackingReferences) ReferencesForResource(context.Context, string) ([]Reference, error) {
	return nil, nil
}

func (s *changeTrackingReferences) MarkResourcesChanged(_ context.Context, ids []string) error {
	s.changed = append(s.changed, ids...)
	return nil
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

func TestDeleteLastResourcePersistsEmptyArray(t *testing.T) {
	service, _ := newRuleService(t, nil)
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(context.Background(), resource.ID); err != nil {
		t.Fatal(err)
	}
	resources, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resources == nil || len(resources) != 0 {
		t.Fatalf("resources=%#v", resources)
	}
	payload, err := os.ReadFile(service.store.file)
	if err != nil || !bytes.Contains(payload, []byte(`"resources": []`)) {
		t.Fatalf("state=%s err=%v", payload, err)
	}
}

func TestNetworkResourceRejectsSourceFormat(t *testing.T) {
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
	if _, err := service.Patch(context.Background(), resource.ID, PatchInput{Format: &format}); err == nil {
		t.Fatal("patched network resource to source")
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("binary file was changed: %v", err)
	}
}

func TestPatchURLInvalidatesDownloadedContent(t *testing.T) {
	service, layout := newRuleService(t, nil)
	service.SetDownloader(catalogRuleDownloader{})
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/old.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateNow(context.Background(), resource.ID); err != nil {
		t.Fatal(err)
	}
	target, _, err := layout.RuleSetPath(resource.ID, ".srs")
	if err != nil {
		t.Fatal(err)
	}
	newURL := "https://example.test/new.srs"
	updated, err := service.Patch(context.Background(), resource.ID, PatchInput{URL: &newURL})
	if err != nil {
		t.Fatal(err)
	}
	if updated.URL != newURL || updated.LastDigest != "" || updated.LastSuccessAt != nil || updated.SizeBytes != nil {
		t.Fatalf("updated=%+v", updated)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("old downloaded file remains: %v", err)
	}
}

func TestPatchDisabledMarksReferencedProfilesChanged(t *testing.T) {
	profiles := &changeTrackingReferences{}
	service, _ := newRuleServiceWithProfiles(t, profiles)
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	disabled := true
	updated, err := service.Patch(context.Background(), resource.ID, PatchInput{Disabled: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Disabled || len(profiles.changed) != 1 || profiles.changed[0] != resource.ID {
		t.Fatalf("updated=%+v changed=%v", updated, profiles.changed)
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
	if _, err := service.Create(context.Background(), CreateInput{Name: "source", URL: "https://example.test/a.json", Format: FormatSource}); err == nil {
		t.Fatal("accepted source network resource")
	}
	if _, err := service.Create(context.Background(), CreateInput{SourceType: SourceManual, Name: "manual", Format: FormatBinary, Content: json.RawMessage(`{"version":3,"rules":[{"domain_suffix":["example.com"]}]}`)}); err == nil {
		t.Fatal("accepted binary manual resource")
	}
	resource, err := service.Create(context.Background(), CreateInput{Name: "binary", URL: "https://example.test/a.srs"})
	if err != nil || resource.Format != FormatBinary {
		t.Fatalf("default resource=%+v err=%v", resource, err)
	}
}

func TestManualResourcePersistsValidatedContentAndSkipsNetworkUpdate(t *testing.T) {
	service, layout := newRuleService(t, nil)
	resource, err := service.Create(context.Background(), CreateInput{
		SourceType: SourceManual,
		Name:       "私有网络",
		Content:    []byte(`{"version":3,"rules":[{"ip_cidr":["10.0.0.0/8","192.168.0.0/16"]}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resource.SourceType != SourceManual || resource.Format != FormatSource || resource.RuleCount == nil || *resource.RuleCount != 2 || resource.LastDigest == "" || resource.SizeBytes == nil {
		t.Fatalf("resource=%+v", resource)
	}
	path, _, err := layout.RuleSetPath(resource.ID, ".json")
	if err != nil {
		t.Fatal(err)
	}
	if payload, err := os.ReadFile(path); err != nil || !bytes.Contains(payload, []byte("192.168.0.0/16")) {
		t.Fatalf("payload=%s err=%v", payload, err)
	}
	detail, err := service.Get(context.Background(), resource.ID)
	if err != nil || !bytes.Contains(detail.Content, []byte("10.0.0.0/8")) {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	if _, err := service.UpdateNow(context.Background(), resource.ID); !errors.Is(err, ErrManualUpdate) {
		t.Fatalf("err=%v", err)
	}
}

func TestResourceCanConvertBetweenRemoteAndManualWithoutChangingID(t *testing.T) {
	service, _ := newRuleService(t, nil)
	resource, err := service.Create(context.Background(), CreateInput{Name: "CN", URL: "https://example.test/cn.srs", Format: FormatBinary})
	if err != nil {
		t.Fatal(err)
	}
	manual := SourceManual
	content := json.RawMessage(`{"version":3,"rules":[{"domain_suffix":["example.com"]}]}`)
	converted, err := service.Patch(context.Background(), resource.ID, PatchInput{SourceType: &manual, Content: &content})
	if err != nil {
		t.Fatal(err)
	}
	if converted.ID != resource.ID || converted.SourceType != SourceManual || converted.Format != FormatSource || converted.LastDigest == "" {
		t.Fatalf("manual=%+v", converted)
	}
	remote := SourceRemote
	url := "https://example.test/cn.srs"
	format := FormatBinary
	converted, err = service.Patch(context.Background(), resource.ID, PatchInput{SourceType: &remote, URL: &url, Format: &format})
	if err != nil {
		t.Fatal(err)
	}
	if converted.SourceType != SourceRemote || converted.URL != url || converted.Format != FormatBinary || converted.LastDigest != "" || converted.SizeBytes != nil {
		t.Fatalf("remote=%+v", converted)
	}
}
