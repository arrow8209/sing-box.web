package profile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/atomicfile"
	"singbox-web/internal/configdoc"
)

func TestServiceCreateUsesTemplateAndStableSlug(t *testing.T) {
	dir := t.TempDir()
	now := func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
	repo, err := OpenRepository(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, template, bytes.NewReader(bytes.Repeat([]byte{3}, 256)))
	item, draft, err := service.Create(context.Background(), CreateInput{Name: "主配置", Slug: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Slug != "primary" || item.DraftRevision != 1 || item.PublishedRevision != nil {
		t.Fatalf("profile = %#v", item)
	}
	if draft.Revision != 1 || draft.ProfileID != item.ID || len(draft.Content.Config) == 0 {
		t.Fatalf("draft = %#v", draft)
	}
	if item.ID != strings.Repeat("03", 16) {
		t.Fatalf("ID = %q", item.ID)
	}
	if draft.SavedAt != now() {
		t.Fatalf("SavedAt = %v", draft.SavedAt)
	}
	if _, err := os.Stat(filepath.Join(dir, "config-root")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config-root was touched: %v", err)
	}
}

func TestServiceValidatesTrimmedUnicodeName(t *testing.T) {
	service := newTestService(t, bytes.NewReader(bytes.Repeat([]byte{4}, 512)))
	item, _, err := service.Create(context.Background(), CreateInput{Name: "  主配置  ", Slug: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "主配置" {
		t.Fatalf("Name = %q", item.Name)
	}
	for _, name := range []string{"   ", strings.Repeat("界", 81)} {
		if _, _, err := service.Create(context.Background(), CreateInput{Name: name, Slug: "another"}); err == nil {
			t.Fatalf("Create accepted name with %d code points", len([]rune(strings.TrimSpace(name))))
		}
	}
}

func TestServiceRandomReadFailureDoesNotPersist(t *testing.T) {
	readers := map[string]io.Reader{
		"profile ID": failingReader{},
		"template":   io.MultiReader(bytes.NewReader(bytes.Repeat([]byte{6}, 16)), failingReader{}),
	}
	for name, random := range readers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			repo, err := OpenRepository(dir, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			template, err := configdoc.LoadTemplate(assets.DefaultConfig)
			if err != nil {
				t.Fatal(err)
			}
			service := NewService(repo, template, random)
			if _, _, err := service.Create(context.Background(), CreateInput{Name: "Primary", Slug: "primary"}); err == nil {
				t.Fatal("Create succeeded with failing random reader")
			}
			items, err := repo.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 0 {
				t.Fatalf("profiles = %#v", items)
			}
		})
	}
}

func TestServiceDelegatesMetadataOperations(t *testing.T) {
	service := newTestService(t, bytes.NewReader(bytes.Repeat([]byte{5}, 512)))
	created, _, err := service.Create(context.Background(), CreateInput{Name: "Before", Slug: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("List = %#v, %v", listed, err)
	}
	got, err := service.Get(context.Background(), created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("Get = %#v, %v", got, err)
	}
	renamed, err := service.Rename(context.Background(), created.ID, " After ")
	if err != nil || renamed.Name != "After" {
		t.Fatalf("Rename = %#v, %v", renamed, err)
	}
	if err := service.RemoveMetadata(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = service.List(context.Background())
	if err != nil || len(listed) != 0 {
		t.Fatalf("List after remove = %#v, %v", listed, err)
	}
}

func TestServiceConcurrentCreateIsRaceSafe(t *testing.T) {
	var randomData []byte
	for value := byte(1); value <= 8; value++ {
		randomData = append(randomData, bytes.Repeat([]byte{value}, 57)...)
	}
	service := newTestService(t, bytes.NewReader(randomData))
	errCh := make(chan error, 8)
	var wait sync.WaitGroup
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, _, err := service.Create(context.Background(), CreateInput{
				Name: "Profile",
				Slug: fmt.Sprintf("profile-%d", index),
			})
			errCh <- err
		}(index)
	}
	wait.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 8 {
		t.Fatalf("List length = %d", len(items))
	}
}

func TestServiceCreateReturnsCreatedValuesWithDurabilityError(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("directory sync failed")
	repo.writeIndexFile = func(path string, disk indexFile) error {
		if err := atomicfile.WriteJSON(path, 0o600, disk); err != nil {
			return err
		}
		return wantErr
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, template, bytes.NewReader(bytes.Repeat([]byte{7}, 256)))
	item, draft, err := service.Create(context.Background(), CreateInput{Name: "Primary", Slug: "primary"})
	if !errors.Is(err, ErrIndexDurabilityUncertain) || !errors.Is(err, wantErr) {
		t.Fatalf("Create error = %v", err)
	}
	if item.ID == "" || draft.ProfileID != item.ID || draft.Revision != 1 {
		t.Fatalf("Create values = %#v, %#v", item, draft)
	}
	items, listErr := service.List(context.Background())
	if listErr != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("List = %#v, %v", items, listErr)
	}
	got, getErr := service.Get(context.Background(), item.ID)
	if getErr != nil || got.ID != item.ID {
		t.Fatalf("Get = %#v, %v", got, getErr)
	}
	reopened, openErr := OpenRepository(dir, time.Now)
	if openErr != nil {
		t.Fatal(openErr)
	}
	got, getErr = reopened.Get(context.Background(), item.ID)
	if getErr != nil || got.ID != item.ID {
		t.Fatalf("reopened Get = %#v, %v", got, getErr)
	}
}

func TestServiceCreateReturnsZeroValuesWhenIndexWasNotCommitted(t *testing.T) {
	service, repo := newServiceWithRepository(t)
	wantErr := errors.New("rename failed")
	repo.writeIndexFile = func(string, indexFile) error { return wantErr }
	item, draft, err := service.Create(context.Background(), CreateInput{Name: "Primary", Slug: "primary"})
	if !errors.Is(err, wantErr) || errors.Is(err, ErrIndexDurabilityUncertain) {
		t.Fatalf("Create error = %v", err)
	}
	if item != (Profile{}) || draft.ProfileID != "" {
		t.Fatalf("Create values = %#v, %#v", item, draft)
	}
	items, err := repo.List(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("List = %#v, %v", items, err)
	}
}

func TestServiceCreateReturnsZeroValuesAndFreezesWhenIndexStateIsUncertain(t *testing.T) {
	service, repo := newServiceWithRepository(t)
	wantErr := errors.New("uncertain write failure")
	backupPath := repo.indexPath + ".backup"
	repo.writeIndexFile = func(path string, _ indexFile) error {
		if err := os.Rename(path, backupPath); err != nil {
			return err
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		return wantErr
	}
	item, draft, err := service.Create(context.Background(), CreateInput{Name: "Primary", Slug: "primary"})
	if !errors.Is(err, wantErr) || errors.Is(err, ErrIndexDurabilityUncertain) {
		t.Fatalf("Create error = %v", err)
	}
	if item != (Profile{}) || draft.ProfileID != "" {
		t.Fatalf("Create values = %#v, %#v", item, draft)
	}
	if _, err := repo.List(context.Background()); err == nil {
		t.Fatal("List succeeded while repository requires recovery")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func newTestService(t *testing.T, random io.Reader) *Service {
	t.Helper()
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(repo, template, random)
}

func newServiceWithRepository(t *testing.T) (*Service, *Repository) {
	t.Helper()
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, template, bytes.NewReader(bytes.Repeat([]byte{8}, 256)))
	return service, repo
}
