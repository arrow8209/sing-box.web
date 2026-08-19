package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/atomicfile"
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

func TestRepositoryPublicationMetadataOperations(t *testing.T) {
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 2}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 2}); err != nil {
		t.Fatal(err)
	}
	checked := time.Date(2026, 8, 18, 1, 2, 3, 0, time.UTC)
	validation := ValidationSummary{Revision: 2, Status: "invalid", Stage: "schema", Message: "1 个问题", CheckedAt: checked}
	staleValidation := validation
	staleValidation.Revision = 1
	if _, err := repo.RecordValidation(context.Background(), item.ID, 1, staleValidation); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale validation error = %v", err)
	}
	got, err := repo.RecordValidation(context.Background(), item.ID, 2, validation)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastValidation == nil || *got.LastValidation != validation || got.PublishedRevision != nil {
		t.Fatalf("validation profile = %#v", got)
	}

	state := PublicationState{Revision: 2, Digest: "digest", PublishedAt: checked, Validation: ValidationSummary{Revision: 2, Status: "valid", Stage: "singbox", Message: "sing-box check 通过", CheckedAt: checked}}
	staleState := state
	staleState.Revision, staleState.Validation.Revision = 1, 1
	if _, err := repo.RecordPublication(context.Background(), item.ID, 1, staleState); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale publication error = %v", err)
	}
	got, err = repo.RecordPublication(context.Background(), item.ID, 2, state)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublishedRevision == nil || *got.PublishedRevision != 2 || got.PublishedDigest != state.Digest || got.PublishedAt == nil || *got.PublishedAt != checked || got.LastValidation == nil || *got.LastValidation != state.Validation {
		t.Fatalf("publication profile = %#v", got)
	}

	recovery := PublicationState{Revision: 1, Digest: "recovered", PublishedAt: checked.Add(time.Hour), Validation: ValidationSummary{Revision: 1, Status: "valid", Stage: "recovery", Message: "由磁盘发布状态恢复", CheckedAt: checked.Add(time.Hour)}}
	got, err = repo.RepairPublication(context.Background(), item.ID, &recovery)
	if err != nil {
		t.Fatal(err)
	}
	if got.DraftRevision != 2 || got.PublishedRevision == nil || *got.PublishedRevision != 1 || got.LastValidation == nil || *got.LastValidation != recovery.Validation {
		t.Fatalf("repaired profile = %#v", got)
	}
	got, err = repo.RepairPublication(context.Background(), item.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.DraftRevision != 2 || got.PublishedRevision != nil || got.PublishedDigest != "" || got.PublishedAt != nil || got.LastValidation == nil || *got.LastValidation != recovery.Validation {
		t.Fatalf("cleared profile = %#v", got)
	}
}

func TestRepositoryDeletionIntentPersistsCancelsAndIsRemovedAtomically(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	intent := DeletionIntent{ProfileID: item.ID, Target: ".generations/" + item.ID + "/1-010101010101", Digest: strings.Repeat("01", 32), TxID: strings.Repeat("02", 16), Phase: DeletionPhasePrepared}
	if err := repo.BeginDeletion(context.Background(), item.ID, intent); err != nil {
		t.Fatal(err)
	}
	repo, err = OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	intents, err := repo.PendingDeletions(context.Background())
	if err != nil || !reflect.DeepEqual(intents, []DeletionIntent{intent}) {
		t.Fatalf("intents = %#v, %v", intents, err)
	}
	if err := repo.CancelDeletion(context.Background(), item.ID, intent.TxID); err != nil {
		t.Fatal(err)
	}
	if intents, _ := repo.PendingDeletions(context.Background()); len(intents) != 0 {
		t.Fatalf("cancelled intents = %#v", intents)
	}
	if err := repo.BeginDeletion(context.Background(), item.ID, intent); err != nil {
		t.Fatal(err)
	}
	if err := repo.TransitionDeletionPhase(context.Background(), item.ID, intent.TxID, DeletionPhasePrepared, DeletionPhaseDiskRemovalAuthorized); err != nil {
		t.Fatal(err)
	}
	repo, err = OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	intents, err = repo.PendingDeletions(context.Background())
	if err != nil || len(intents) != 1 || intents[0].Phase != DeletionPhaseDiskRemovalAuthorized {
		t.Fatalf("transitioned intents = %#v, %v", intents, err)
	}
	if err := repo.RemoveMetadata(context.Background(), item.ID); err != nil {
		t.Fatal(err)
	}
	if intents, _ := repo.PendingDeletions(context.Background()); len(intents) != 0 {
		t.Fatalf("removed profile intents = %#v", intents)
	}
	if _, err := os.Lstat(repo.DraftPath(item.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("draft remains: %v", err)
	}
}

func TestRepositoryDeletionIntentDurabilityUncertainUpdatesMemory(t *testing.T) {
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	intent := DeletionIntent{ProfileID: item.ID, Target: ".generations/" + item.ID + "/1-010101010101", Digest: strings.Repeat("01", 32), TxID: strings.Repeat("02", 16), Phase: DeletionPhasePrepared}
	wantErr := errors.New("sync failed")
	repo.writeIndexFile = func(path string, disk indexFile) error {
		if err := atomicfile.WriteJSON(path, 0o600, disk); err != nil {
			return err
		}
		return wantErr
	}
	if err := repo.BeginDeletion(context.Background(), item.ID, intent); !errors.Is(err, ErrIndexDurabilityUncertain) {
		t.Fatalf("BeginDeletion error = %v", err)
	}
	intents, err := repo.PendingDeletions(context.Background())
	if err != nil || len(intents) != 1 || intents[0] != intent {
		t.Fatalf("intents = %#v, %v", intents, err)
	}
}

func TestDeletionIntentRejectsInvalidTargetAndPhase(t *testing.T) {
	id := "01010101010101010101010101010101"
	valid := DeletionIntent{ProfileID: id, Target: ".generations/" + id + "/1-010101010101", Digest: strings.Repeat("01", 32), TxID: strings.Repeat("02", 16), Phase: DeletionPhasePrepared}
	for name, mutate := range map[string]func(*DeletionIntent){
		"absolute target": func(intent *DeletionIntent) { intent.Target = "/.generations/" + id + "/1-010101010101" },
		"unclean target":  func(intent *DeletionIntent) { intent.Target = ".generations/../.generations/" + id + "/1-010101010101" },
		"other profile": func(intent *DeletionIntent) {
			intent.Target = ".generations/" + strings.Repeat("03", 16) + "/1-010101010101"
		},
		"zero revision": func(intent *DeletionIntent) { intent.Target = ".generations/" + id + "/0-010101010101" },
		"signed revision": func(intent *DeletionIntent) {
			intent.Target = ".generations/" + id + "/+1-010101010101"
		},
		"wrong prefix": func(intent *DeletionIntent) { intent.Target = ".generations/" + id + "/1-020202020202" },
		"uppercase prefix": func(intent *DeletionIntent) {
			intent.Digest = strings.Repeat("ab", 32)
			intent.Target = ".generations/" + id + "/1-ABABABABABAB"
		},
		"invalid phase": func(intent *DeletionIntent) { intent.Phase = "removing" },
	} {
		t.Run(name, func(t *testing.T) {
			intent := valid
			mutate(&intent)
			if err := validateDeletionIntent(intent); err == nil {
				t.Fatalf("validateDeletionIntent(%#v) succeeded", intent)
			}
		})
	}
}

func TestRepositoryOrdinaryCommitRetainsDeletionIntent(t *testing.T) {
	repo, err := OpenRepository(t.TempDir(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); err != nil {
		t.Fatal(err)
	}
	intent := DeletionIntent{ProfileID: item.ID, Target: ".generations/" + item.ID + "/1-010101010101", Digest: strings.Repeat("01", 32), TxID: strings.Repeat("02", 16), Phase: DeletionPhasePrepared}
	if err := repo.BeginDeletion(context.Background(), item.ID, intent); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Rename(context.Background(), item.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	intents, err := repo.PendingDeletions(context.Background())
	if err != nil || !reflect.DeepEqual(intents, []DeletionIntent{intent}) {
		t.Fatalf("intents after rename = %#v, %v", intents, err)
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
	wantErr := errors.New("rename failed")
	repo.writeIndexFile = func(string, indexFile) error { return wantErr }
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

func TestRepositoryInsertPreservesDraftAndSynchronizesMemoryWhenIndexWasReplaced(t *testing.T) {
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
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	draft := Draft{ProfileID: item.ID, Revision: 1}
	if err := repo.Insert(context.Background(), item, draft); !errors.Is(err, wantErr) {
		t.Fatalf("Insert error = %v, want %v", err, wantErr)
	}
	if _, err := os.Stat(repo.DraftPath(item.ID)); err != nil {
		t.Fatalf("draft was removed after index replacement: %v", err)
	}
	items, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, []Profile{item}) {
		t.Fatalf("in-memory List = %#v", items)
	}

	reopened, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	items, err = reopened.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, []Profile{item}) {
		t.Fatalf("reopened List = %#v", items)
	}
}

func TestRepositoryInsertPreservesDraftAndRequiresReopenWhenIndexStateIsUncertain(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
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
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1}); !errors.Is(err, wantErr) {
		t.Fatalf("Insert error = %v, want %v", err, wantErr)
	}
	draftPath := repo.DraftPath(item.ID)
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("draft was removed while index state was uncertain: %v", err)
	}
	if _, err := repo.List(context.Background()); err == nil {
		t.Fatal("List succeeded while repository requires recovery")
	}

	if err := os.Remove(repo.indexPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backupPath, repo.indexPath); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reopened.List(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("recovered List = %#v, %v", items, err)
	}
	if _, err := os.Stat(draftPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan draft remains after recovery: %v", err)
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

func TestCorruptDraftIsReadOnlyWithoutBlockingOtherProfiles(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	items := []Profile{
		{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1},
		{ID: "02020202020202020202020202020202", Name: "Two", Slug: "two", DraftRevision: 1},
	}
	for _, item := range items {
		if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1, Content: DraftContent{Config: json.RawMessage(`{}`)}}); err != nil {
			t.Fatal(err)
		}
	}
	corruptPath := repo.DraftPath(items[0].ID)
	if err := os.WriteFile(corruptPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err = OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repo, nil, nil)
	listed, err := service.List(context.Background())
	if err != nil || len(listed) != 2 {
		t.Fatalf("List = %#v, %v", listed, err)
	}
	status := service.DraftStatus(context.Background(), items[0].ID)
	if !status.ReadOnly || status.Message != "草稿 JSON 损坏，需要管理员在服务器修复" {
		t.Fatalf("status = %#v", status)
	}
	if _, err := service.GetDraft(context.Background(), items[0].ID); !errors.Is(err, ErrDraftCorrupt) {
		t.Fatalf("GetDraft error = %v", err)
	}
	if _, err := service.SaveDraft(context.Background(), items[0].ID, 1, DraftContent{Config: json.RawMessage(`{}`)}); !errors.Is(err, ErrDraftCorrupt) {
		t.Fatalf("SaveDraft error = %v", err)
	}
	raw, err := os.ReadFile(corruptPath)
	if err != nil || string(raw) != "{" {
		t.Fatalf("corrupt bytes = %q, %v", raw, err)
	}
	if _, err := service.SaveDraft(context.Background(), items[1].ID, 1, DraftContent{Config: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRepairsOneStepDraftCommit(t *testing.T) {
	dir := t.TempDir()
	repo, err := OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: 1}
	if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: 1, Content: DraftContent{Config: json.RawMessage(`{"n":1}`)}}); err != nil {
		t.Fatal(err)
	}
	draft := Draft{ProfileID: item.ID, Revision: 2, Content: DraftContent{Config: json.RawMessage(`{"n":9007199254740993}`)}, SavedAt: time.Now()}
	if err := atomicfile.WriteJSON(repo.DraftPath(item.ID), 0o600, draft); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(repo.DraftPath(item.ID))
	repo, err = OpenRepository(dir, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(context.Background(), item.ID)
	if err != nil || got.DraftRevision != 2 {
		t.Fatalf("profile = %#v, %v", got, err)
	}
	after, _ := os.ReadFile(repo.DraftPath(item.ID))
	if !bytes.Equal(before, after) {
		t.Fatal("repair changed draft bytes")
	}
}

func TestOpenIsolatesUnsafeDraftRevisionMismatch(t *testing.T) {
	tests := []struct {
		name     string
		profile  uint64
		draftID  string
		draftRev uint64
		remove   bool
	}{
		{name: "missing", profile: 1, draftRev: 1, remove: true},
		{name: "profile ahead", profile: 2, draftRev: 1},
		{name: "gap exceeds one", profile: 1, draftRev: 3},
		{name: "ID differs", profile: 1, draftID: "02020202020202020202020202020202", draftRev: 1},
		{name: "zero", profile: 1, draftRev: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			repo, err := OpenRepository(dir, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			item := Profile{ID: "01010101010101010101010101010101", Name: "One", Slug: "one", DraftRevision: test.profile}
			if err := repo.Insert(context.Background(), item, Draft{ProfileID: item.ID, Revision: test.profile, Content: DraftContent{Config: json.RawMessage(`{}`)}}); err != nil {
				t.Fatal(err)
			}
			path := repo.DraftPath(item.ID)
			if test.remove {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				id := test.draftID
				if id == "" {
					id = item.ID
				}
				if err := atomicfile.WriteJSON(path, 0o600, Draft{ProfileID: id, Revision: test.draftRev, Content: DraftContent{Config: json.RawMessage(`{}`)}}); err != nil {
					t.Fatal(err)
				}
			}
			repo, err = OpenRepository(dir, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			if status := NewService(repo, nil, nil).DraftStatus(context.Background(), item.ID); !status.ReadOnly {
				t.Fatalf("status = %#v", status)
			}
		})
	}
}

func readTestJSON(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
