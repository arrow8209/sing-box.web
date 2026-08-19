package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/configdoc"
)

func TestSaveDraftRejectsStaleRevision(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	content := DraftContent{Config: json.RawMessage(`{"log":{"level":"debug"}}`), EditorBuffers: map[string]EditorBuffer{}}
	saved, err := service.SaveDraft(context.Background(), item.ID, 1, content)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 {
		t.Fatalf("revision = %d, want 2", saved.Revision)
	}
	_, err = service.SaveDraft(context.Background(), item.ID, 1, content)
	var conflict *RevisionConflict
	if !errors.As(err, &conflict) || conflict.Current != 2 {
		t.Fatalf("error = %#v", err)
	}
}

func TestSaveDraftPersistsInvalidEditorBufferButBlocksPublish(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	content := DraftContent{
		Config:        json.RawMessage(`{"log":{"level":"info"}}`),
		EditorBuffers: map[string]EditorBuffer{"log": {Text: `{"log":`, Valid: true, Applied: true, Error: "client"}},
	}
	saved, err := service.SaveDraft(context.Background(), item.ID, 1, content)
	if err != nil {
		t.Fatal(err)
	}
	buffer := saved.Content.EditorBuffers["log"]
	if buffer.Valid || buffer.Applied || buffer.Error == "" || IsPublishable(saved) {
		t.Fatalf("invalid buffer was publishable: %#v", saved)
	}
	if !bytes.Contains([]byte(buffer.Text), []byte(`{"log":`)) || bytes.Contains([]byte(buffer.Error), []byte(buffer.Text)) {
		t.Fatalf("buffer/error = %#v", buffer)
	}
}

func TestSaveDraftDivergentBuffersBlockPublishAndSurviveReopen(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	content := DraftContent{
		Config: json.RawMessage(`{"log":{"level":"info"},"outbounds":[{"type":"direct"}]}`),
		EditorBuffers: map[string]EditorBuffer{
			"log":              {Text: `{"log":{"level":"debug"}}`, Valid: false, Applied: true},
			"outbounds/0/type": {Text: `"block"`, Valid: false, Applied: true},
			"route":            {Text: `{`, Valid: true, Applied: true, Error: "client"},
		},
	}
	saved, err := service.SaveDraft(context.Background(), item.ID, 1, content)
	if err != nil {
		t.Fatal(err)
	}
	for key, buffer := range saved.Content.EditorBuffers {
		if key == "route" {
			if buffer.Valid || buffer.Applied || buffer.Error == "" {
				t.Fatalf("buffer %q = %#v", key, buffer)
			}
			continue
		}
		if !buffer.Valid || buffer.Applied || buffer.Error != "缓冲与配置对象不同步" {
			t.Fatalf("buffer %q = %#v", key, buffer)
		}
	}
	if IsPublishable(saved) {
		t.Fatal("divergent draft is publishable")
	}

	reopened, err := OpenRepository(service.repo.indexPath[:len(service.repo.indexPath)-len("profiles.json")], time.Now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewService(reopened, service.template, service.random).GetDraft(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content.EditorBuffers["log"].Text != content.EditorBuffers["log"].Text || got.Content.EditorBuffers["log"].Applied {
		t.Fatalf("reopened draft = %#v", got)
	}
	if got.Content.EditorBuffers["route"].Text != "{" || got.Content.EditorBuffers["route"].Valid || got.Content.EditorBuffers["route"].Applied {
		t.Fatalf("reopened invalid buffer = %#v", got.Content.EditorBuffers["route"])
	}
}

func TestSaveDraftValidatesConfigAndBufferPaths(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	for _, config := range []string{`[]`, `null`, `{}`, `{} {}`} {
		content := DraftContent{Config: json.RawMessage(config), EditorBuffers: map[string]EditorBuffer{}}
		_, err := service.SaveDraft(context.Background(), item.ID, 1, content)
		if config == `{}` {
			if err != nil {
				t.Fatalf("valid config: %v", err)
			}
			item.DraftRevision++
			continue
		}
		if err == nil {
			t.Fatalf("accepted config %q", config)
		}
	}

	for _, key := range []string{"", "/log", "log/", "log//level", ".", "..", "__proto__", "log/prototype", "constructor/x"} {
		content := DraftContent{Config: json.RawMessage(`{"log":{}}`), EditorBuffers: map[string]EditorBuffer{key: {Text: `{}`}}}
		if _, err := service.SaveDraft(context.Background(), item.ID, item.DraftRevision, content); err == nil {
			t.Fatalf("accepted unsafe key %q", key)
		}
	}
}

func TestSaveDraftConcurrentExpectedRevisionHasAtMostOneSuccess(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	content := DraftContent{Config: json.RawMessage(`{"large":9007199254740993}`), EditorBuffers: map[string]EditorBuffer{"large": {Text: `{"large":9007199254740993}`}}}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.SaveDraft(context.Background(), item.ID, 1, content)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrRevisionConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d, want 1", successes)
	}
	saved, err := service.GetDraft(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if buffer := saved.Content.EditorBuffers["large"]; !buffer.Valid || !buffer.Applied {
		t.Fatalf("large exact number buffer = %#v", buffer)
	}
}

func TestSaveDraftComparesJSONNumbersExactly(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	content := DraftContent{
		Config: json.RawMessage(`{"holder":{"one":1.0,"huge":1e100000000000000000000}}`),
		EditorBuffers: map[string]EditorBuffer{
			"holder/one":  {Text: `1`},
			"holder/huge": {Text: `10e99999999999999999999`},
		},
	}
	saved, err := service.SaveDraft(context.Background(), item.ID, 1, content)
	if err != nil {
		t.Fatal(err)
	}
	if !IsPublishable(saved) {
		t.Fatalf("equivalent exact numbers are not publishable: %#v", saved.Content.EditorBuffers)
	}
}

func TestSaveDraftRestoresPreviousDraftWhenIndexIsNotReplaced(t *testing.T) {
	service, item := newTestServiceWithProfile(t)
	before, err := service.GetDraft(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("index rename failed")
	service.repo.writeIndexFile = func(string, indexFile) error { return wantErr }
	_, err = service.SaveDraft(context.Background(), item.ID, 1, DraftContent{Config: json.RawMessage(`{"changed":true}`)})
	if !errors.Is(err, wantErr) {
		t.Fatalf("SaveDraft error = %v", err)
	}
	after, err := service.GetDraft(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || !bytes.Equal(after.Content.Config, before.Content.Config) || !after.SavedAt.Equal(before.SavedAt) {
		t.Fatalf("after = %#v, before = %#v", after, before)
	}
}

func newTestServiceWithProfile(t *testing.T) (*Service, Profile) {
	t.Helper()
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, template, bytes.NewReader(bytes.Repeat([]byte{9}, 512)))
	item, _, err := service.Create(context.Background(), CreateInput{Name: "Primary", Slug: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	return service, item
}
