package publication

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"singbox-web/internal/profile"
	"singbox-web/internal/singbox"
	"singbox-web/internal/validation"
)

type fakeChecker struct {
	version string
	result  singbox.CheckResult
	err     error
}

func (checker fakeChecker) Version(context.Context) (string, error) {
	return checker.version, checker.err
}
func (checker fakeChecker) Check(context.Context, string) (singbox.CheckResult, error) {
	return checker.result, checker.err
}

func TestPublishReadDriftAndDelete(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "sing-box version latest", result: singbox.CheckResult{Valid: true}})
	receipt, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.item.Slug))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(target, receipt.Digest[:12]) || receipt.Revision != 1 {
		t.Fatalf("target=%q receipt=%#v", target, receipt)
	}
	raw, err := fixture.service.ReadPublished(context.Background(), fixture.item.ID)
	if err != nil || !bytes.Contains(raw, []byte(`"log"`)) {
		t.Fatalf("ReadPublished = %s, %v", raw, err)
	}
	entries, _ := os.ReadDir(filepath.Join(fixture.configRoot, target))
	if err := os.WriteFile(filepath.Join(fixture.configRoot, target, entries[0].Name()), []byte("{}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); !errors.Is(err, ErrExternalDrift) {
		t.Fatalf("drift error = %v", err)
	}
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1, ConfirmExternalOverwrite: true}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, "Primary "); err == nil {
		t.Fatal("Delete accepted mismatched confirmation")
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); err != nil {
		t.Fatal(err)
	}
}

func TestPreflightFailureRecordsCountOnlyAndDoesNotTouchConfigRoot(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	profiles := profile.NewService(fixture.repo, nil, nil)
	draft, err := profiles.GetDraft(context.Background(), fixture.item.ID)
	if err != nil {
		t.Fatal(err)
	}
	draft.Content.EditorBuffers["log"] = profile.EditorBuffer{Text: `{"secret":"do-not-persist"`}
	if _, err := profiles.SaveDraft(context.Background(), fixture.item.ID, 1, draft.Content); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 2}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("Publish error = %v", err)
	}
	if _, err := os.Lstat(fixture.configRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config root was touched: %v", err)
	}
	got, err := fixture.repo.Get(context.Background(), fixture.item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastValidation == nil || got.LastValidation.Revision != 2 || got.LastValidation.Status != "invalid" || strings.Contains(got.LastValidation.Message, "secret") {
		t.Fatalf("validation summary = %#v", got.LastValidation)
	}
}

func TestFailedSecondCheckPreservesPublishedFields(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.repo.Get(context.Background(), fixture.item.ID)
	if err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.item.Slug))
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.options.Checker = fakeChecker{version: "v", result: singbox.CheckResult{Valid: false, Stdout: "secret", Stderr: "private"}}
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("Publish error = %v", err)
	}
	after, err := fixture.repo.Get(context.Background(), fixture.item.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterTarget, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.item.Slug))
	if err != nil {
		t.Fatal(err)
	}
	if target != afterTarget || after.PublishedRevision == nil || before.PublishedRevision == nil || *after.PublishedRevision != *before.PublishedRevision || after.PublishedDigest != before.PublishedDigest || !after.PublishedAt.Equal(*before.PublishedAt) {
		t.Fatalf("publication changed: before=%#v after=%#v", before, after)
	}
	if after.LastValidation == nil || after.LastValidation.Message != "sing-box check 失败" || strings.Contains(after.LastValidation.Message, "secret") {
		t.Fatalf("validation leaked checker output: %#v", after.LastValidation)
	}
}

func TestFailedCheckDoesNotPublish(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: false}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err == nil {
		t.Fatal("Publish succeeded")
	}
	if _, err := os.Lstat(filepath.Join(fixture.configRoot, fixture.item.Slug)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stable path exists: %v", err)
	}
	got, err := fixture.repo.Get(context.Background(), fixture.item.ID)
	if err != nil || got.PublishedRevision != nil || got.LastValidation == nil || got.LastValidation.Stage != "singbox" {
		t.Fatalf("profile = %#v, %v", got, err)
	}
}

func TestPublishRejectsUnmanagedStablePaths(t *testing.T) {
	for _, makePath := range []func(string) error{
		func(path string) error { return os.WriteFile(path, []byte("occupied"), 0o600) },
		func(path string) error { return os.Symlink("../outside", path) },
		func(path string) error {
			return os.Symlink(filepath.Join(string(filepath.Separator), "tmp", "outside"), path)
		},
	} {
		fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
		if err := os.MkdirAll(fixture.configRoot, 0o750); err != nil {
			t.Fatal(err)
		}
		stable := filepath.Join(fixture.configRoot, fixture.item.Slug)
		if err := makePath(stable); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1, ConfirmExternalOverwrite: true}); !errors.Is(err, ErrPathOccupied) {
			t.Fatalf("occupied error = %v", err)
		}
		if _, err := os.Lstat(stable); err != nil {
			t.Fatalf("unmanaged path removed: %v", err)
		}
	}
}

func TestPublishModesGroupAndConcurrentSerialization(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if runtime.GOOS != "windows" {
		groupID := os.Getgid()
		fixture.service.options.GroupID = &groupID
	}
	errCh := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1})
			errCh <- err
		}()
	}
	wait.Wait()
	close(errCh)
	successes := 0
	for err := range errCh {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent publishes = %d", successes)
	}
	target, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.item.Slug))
	if err != nil {
		t.Fatal(err)
	}
	generation := filepath.Join(fixture.configRoot, target)
	entries, err := os.ReadDir(generation)
	if err != nil {
		t.Fatal(err)
	}
	paths := append([]string{fixture.configRoot, filepath.Join(fixture.configRoot, ".generations"), fixture.service.profileGenerationRoot(fixture.item.ID), generation}, func() []string {
		result := make([]string, len(entries))
		for index, entry := range entries {
			result[index] = filepath.Join(generation, entry.Name())
		}
		return result
	}()...)
	for index, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		wantMode := os.FileMode(0o640)
		if index < 4 {
			wantMode = 0o750
		}
		if info.Mode().Perm() != wantMode {
			t.Fatalf("%s mode = %o", path, info.Mode().Perm())
		}
		if fixture.service.options.GroupID != nil {
			stat := info.Sys().(*syscall.Stat_t)
			if int(stat.Gid) != *fixture.service.options.GroupID || stat.Uid != uint32(os.Getuid()) {
				t.Fatalf("%s uid:gid = %d:%d", path, stat.Uid, stat.Gid)
			}
		}
	}
}

func TestRecoverAfterPublicationFailpoints(t *testing.T) {
	for _, stage := range []string{"after_generation_rename", "after_link_swap", "after_metadata"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
			injected := errors.New("interrupt")
			fixture.service.failpoint = func(got string) error {
				if got == stage {
					return injected
				}
				return nil
			}
			if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); !errors.Is(err, injected) {
				t.Fatalf("Publish error = %v", err)
			}
			fixture.service.failpoint = nil
			if err := fixture.service.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			got, err := fixture.repo.Get(context.Background(), fixture.item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stage == "after_generation_rename" {
				if got.PublishedRevision != nil {
					t.Fatalf("unexpected recovered publication = %#v", got)
				}
				return
			}
			raw, err := fixture.service.ReadPublished(context.Background(), fixture.item.ID)
			if err != nil || !bytes.Contains(raw, []byte(`"log"`)) {
				t.Fatalf("recovered config = %s, %v", raw, err)
			}
		})
	}
}

func TestMetadataAndRollbackDoubleFailureRequiresRecovery(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	metadataErr, rollbackErr := errors.New("metadata failed"), errors.New("rollback failed")
	fixture.service.failpoint = func(stage string) error {
		switch stage {
		case "before_metadata":
			return metadataErr
		case "before_link_rollback":
			return rollbackErr
		}
		return nil
	}
	_, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1})
	if !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("Publish error = %v", err)
	}
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("blocked Publish error = %v", err)
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("blocked Delete error = %v", err)
	}
	fixture.service.failpoint = nil
	if err := fixture.service.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.ReadPublished(context.Background(), fixture.item.ID); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmedDriftMetadataFailureRestoresPriorGeneration(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.item.Slug))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(fixture.configRoot, target))
	if err != nil {
		t.Fatal(err)
	}
	drifted := filepath.Join(fixture.configRoot, target, entries[0].Name())
	if err := os.WriteFile(drifted, []byte("{}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	metadataErr := errors.New("metadata failed")
	fixture.service.failpoint = func(stage string) error {
		if stage == "before_metadata" {
			return metadataErr
		}
		return nil
	}
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1, ConfirmExternalOverwrite: true}); !errors.Is(err, metadataErr) {
		t.Fatalf("Publish error = %v", err)
	}
	content, err := os.ReadFile(drifted)
	if err != nil || string(content) != "{}\n" {
		t.Fatalf("prior generation was not restored: %q, %v", content, err)
	}
	fixture.service.failpoint = nil
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1, ConfirmExternalOverwrite: true}); err != nil {
		t.Fatal(err)
	}
}

type publicationFixture struct {
	service    *Service
	repo       *profile.Repository
	item       profile.Profile
	configRoot string
}

func newPublicationFixture(t *testing.T, checker Checker) publicationFixture {
	t.Helper()
	now := func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
	repo, err := profile.OpenRepository(t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	item := profile.Profile{ID: "01010101010101010101010101010101", Name: "Primary", Slug: "primary", DraftRevision: 1}
	draft := profile.Draft{ProfileID: item.ID, Revision: 1, Content: profile.DraftContent{Config: []byte(`{"log":{"level":"info"}}`), EditorBuffers: map[string]profile.EditorBuffer{}}}
	if err := repo.Insert(context.Background(), item, draft); err != nil {
		t.Fatal(err)
	}
	schema, err := validation.NewSchemaValidator([]byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	configRoot := filepath.Join(t.TempDir(), "config")
	return publicationFixture{repo: repo, item: item, configRoot: configRoot, service: NewService(Options{ConfigRoot: configRoot, Profiles: repo, Preflight: validation.NewService(schema, "target", now), Checker: checker, Now: now, Random: bytes.NewReader(bytes.Repeat([]byte{1}, 1024)), FileMode: 0o640, DirMode: 0o750})}
}
