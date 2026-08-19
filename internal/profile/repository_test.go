package profile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRepositoryPersistsOrderAndDraftsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	nowValue := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return nowValue }
	repo, err := OpenRepository(dir, now)
	if err != nil {
		t.Fatal(err)
	}

	items := []Profile{
		{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1},
		{ID: "02020202020202020202020202020202", Name: "Two", Slug: "two", DraftRevision: 1},
	}
	for _, item := range items {
		draft := Draft{ProfileID: item.ID, Revision: 1, Content: DraftContent{Config: json.RawMessage(`{"log":{}}`), EditorBuffers: map[string]EditorBuffer{}}}
		if err := repo.Insert(context.Background(), item, draft); err != nil {
			t.Fatal(err)
		}
	}

	repo, err = OpenRepository(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("List() = %#v, want %#v", got, items)
	}
	for _, item := range items {
		var draft Draft
		if err := readTestJSON(repo.DraftPath(item.ID), &draft); err != nil {
			t.Fatal(err)
		}
		if draft.ProfileID != item.ID || draft.SavedAt != nowValue {
			t.Fatalf("draft = %#v", draft)
		}
	}
}

func TestRepositoryRejectsDuplicateIDAndSlug(t *testing.T) {
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	base := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	draft := Draft{ProfileID: base.ID, Revision: 1, Content: DraftContent{Config: json.RawMessage(`{}`)}}
	if err := repo.Insert(context.Background(), base, draft); err != nil {
		t.Fatal(err)
	}
	for _, item := range []Profile{
		{ID: base.ID, Name: "Duplicate ID", Slug: "different", DraftRevision: 1},
		{ID: "02020202020202020202020202020202", Name: "Duplicate slug", Slug: base.Slug, DraftRevision: 1},
	} {
		if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); err == nil {
			t.Fatalf("Insert(%#v) succeeded", item)
		}
	}
}

func TestRepositoryRenameChangesOnlyName(t *testing.T) {
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	want := Profile{ID: "01010101010101010101010101010101", Name: "Before", Slug: "stable", DraftRevision: 7, PublishedDigest: "digest"}
	if err := repo.Insert(context.Background(), want, Draft{ProfileID: want.ID, Revision: 7}); err != nil {
		t.Fatal(err)
	}
	want.Name = "After"
	got, err := repo.Rename(context.Background(), want.ID, "After")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Rename() = %#v, want %#v", got, want)
	}
}

func TestSlugValidation(t *testing.T) {
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{"", "Upper", "-start", "has_underscore", "has space", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"}
	for i, slug := range invalid {
		item := Profile{ID: string(rune('a'+i)) + "1010101010101010101010101010101", Name: "Name", Slug: slug, DraftRevision: 1}
		if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); err == nil {
			t.Errorf("Insert accepted invalid slug %q", slug)
		}
	}
}

func TestRepositoryOpenInitializesFilesAndRejectsInvalidIndex(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "drafts")); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("drafts mode = %v, err = %v", info.Mode().Perm(), err)
	}
	if _, err := os.Stat(filepath.Join(dir, "profiles.json")); err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{
		`{"version":2,"profiles":[]}`,
		`{"version":1,"profiles":[{"id":"bad","name":"Bad","slug":"valid","draftRevision":1,"publishedRevision":null,"publishedDigest":"","publishedAt":null,"lastValidation":null}]}`,
		`{"version":1,"profiles":[{"id":"01010101010101010101010101010101","name":"Bad","slug":"INVALID","draftRevision":1,"publishedRevision":null,"publishedDigest":"","publishedAt":null,"lastValidation":null}]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "profiles.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenRepository(dir, time.Now); err == nil {
			t.Fatalf("OpenRepository accepted %s", raw)
		}
	}
	_ = repo
}

func TestRepositoryInsertRollsBackDraftWhenIndexWriteFails(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	draft := Draft{ProfileID: item.ID, Revision: 1}
	blocked := filepath.Join(dir, "blocked-index")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	repo.indexPath = blocked
	if err := repo.Insert(context.Background(), item, draft); err == nil {
		t.Fatal("Insert succeeded when index could not be committed")
	}
	if _, err := os.Stat(repo.DraftPath(item.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("draft was not rolled back: %v", err)
	}
	items, err := repo.List(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("List = %#v, %v", items, err)
	}
}

func TestRepositoryRemoveCommitsIndexBeforeRemovingDraft(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	draftPath := repo.DraftPath(item.ID)
	if err := os.Remove(draftPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(draftPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(draftPath, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveMetadata(context.Background(), item.ID); err == nil {
		t.Fatal("RemoveMetadata succeeded despite draft removal failure")
	}

	reopened, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reopened.List(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("persisted List = %#v, %v", items, err)
	}
}

func TestRepositoryOpenRemovesOrphanDraft(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	id := "01010101010101010101010101010101"
	path := repo.DraftPath(id)
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenRepository(dir, time.Now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan draft remains: %v", err)
	}
}

func readTestJSON(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
