package publication

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

type checkOnlyErrorChecker struct{ cause error }

func (checker checkOnlyErrorChecker) Version(context.Context) (string, error) { return "v", nil }
func (checker checkOnlyErrorChecker) Check(context.Context, string) (singbox.CheckResult, error) {
	return singbox.CheckResult{}, checker.cause
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

func TestCheckErrorIsClassifiableWithoutLeakingText(t *testing.T) {
	cause := fmt.Errorf("secret-token in /absolute/private/path: %w", singbox.ErrCheckFailed)
	fixture := newPublicationFixture(t, checkOnlyErrorChecker{cause: cause})
	_, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1})
	if err == nil || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "/absolute/private/path") {
		t.Fatalf("unsafe error = %v", err)
	}
	if !errors.Is(err, singbox.ErrCheckFailed) || errors.Is(err, cause) {
		t.Fatalf("error lost classification: %v", err)
	}
	if unwrapped := errors.Unwrap(err); unwrapped == nil || strings.Contains(unwrapped.Error(), "secret-token") {
		t.Fatalf("unsafe unwrap = %v", unwrapped)
	}
	unknown := errors.New("unknown-secret /private/unknown")
	fixture = newPublicationFixture(t, checkOnlyErrorChecker{cause: unknown})
	_, err = fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1})
	if !errors.Is(err, singbox.ErrCheckFailed) || errors.Is(err, unknown) || strings.Contains(err.Error(), "unknown-secret") || strings.Contains(errors.Unwrap(err).Error(), "unknown-secret") {
		t.Fatalf("unsafe unknown error = %v, unwrap=%v", err, errors.Unwrap(err))
	}
}

func TestDirectoryCreationSyncOrderAndFailure(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	var calls []string
	fixture.service.syncDirectory = func(path string) error { calls = append(calls, path); return nil }
	if err := fixture.service.prepareRoot(); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.prepareProfileRoot(fixture.item.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{fixture.configRoot, filepath.Join(fixture.configRoot, ".generations"), fixture.configRoot, fixture.service.profileGenerationRoot(fixture.item.ID), filepath.Join(fixture.configRoot, ".generations")}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("sync calls = %#v, want %#v", calls, want)
	}

	failed := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	wantErr := errors.New("sync failed")
	failed.service.syncDirectory = func(path string) error {
		if strings.HasSuffix(path, ".generations") {
			return wantErr
		}
		return nil
	}
	if err := failed.service.prepareRoot(); !errors.Is(err, wantErr) {
		t.Fatalf("prepareRoot error = %v", err)
	}
}

func TestDeleteRejectsManagedContentDriftWithoutTouchingState(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(fixture.configRoot, fixture.item.Slug)
	target, err := os.Readlink(stable)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(fixture.configRoot, target))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.configRoot, target, entries[0].Name()), []byte("{}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); !errors.Is(err, ErrExternalDrift) {
		t.Fatalf("Delete error = %v", err)
	}
	if got, err := os.Readlink(stable); err != nil || got != target {
		t.Fatalf("stable changed: %q, %v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.configRoot, target)); err != nil {
		t.Fatalf("generation removed: %v", err)
	}
	if _, err := fixture.repo.Get(context.Background(), fixture.item.ID); err != nil {
		t.Fatalf("metadata removed: %v", err)
	}
	if _, err := os.Lstat(fixture.repo.DraftPath(fixture.item.ID)); err != nil {
		t.Fatalf("draft removed: %v", err)
	}
}

func TestDeleteDetectsReplacedPathRaceAndRestoresReplacement(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(fixture.configRoot, fixture.item.Slug)
	fixture.service.failpoint = func(stage string) error {
		if stage == "before_delete_exchange" {
			if err := os.Remove(stable); err != nil {
				return err
			}
			return os.WriteFile(stable, []byte("replacement"), 0o600)
		}
		return nil
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); !errors.Is(err, ErrExternalDrift) {
		t.Fatalf("Delete error = %v", err)
	}
	content, err := os.ReadFile(stable)
	if err != nil || string(content) != "replacement" {
		t.Fatalf("replacement changed: %q, %v", content, err)
	}
	if _, err := fixture.repo.Get(context.Background(), fixture.item.ID); err != nil {
		t.Fatalf("metadata removed: %v", err)
	}
}

func TestDeleteRechecksDigestAfterRenameAndRollsBack(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(fixture.configRoot, fixture.item.Slug)
	target, err := os.Readlink(stable)
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.failpoint = func(stage string) error {
		if stage == "before_delete_digest" {
			entries, err := os.ReadDir(filepath.Join(fixture.configRoot, target))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(fixture.configRoot, target, entries[0].Name()), []byte("{}\n"), 0o640)
		}
		return nil
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); !errors.Is(err, ErrExternalDrift) {
		t.Fatalf("Delete error = %v", err)
	}
	if got, err := os.Readlink(stable); err != nil || got != target {
		t.Fatalf("stable not restored: %q, %v", got, err)
	}
	if markers, err := fixture.service.deletionMarkers(fixture.item.ID); err != nil || len(markers) != 0 {
		t.Fatalf("tombstones remain: %#v, %v", markers, err)
	}
	if _, err := fixture.repo.Get(context.Background(), fixture.item.ID); err != nil {
		t.Fatalf("metadata removed: %v", err)
	}
	if _, err := os.Lstat(fixture.repo.DraftPath(fixture.item.ID)); err != nil {
		t.Fatalf("draft removed: %v", err)
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
			if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
				t.Fatal(err)
			}
			profiles := profile.NewService(fixture.repo, nil, nil)
			draft, err := profiles.GetDraft(context.Background(), fixture.item.ID)
			if err != nil {
				t.Fatal(err)
			}
			draft.Content.Config = []byte(`{"log":{"level":"debug"},"dns":{"strategy":"ipv4_only"}}`)
			if _, err := profiles.SaveDraft(context.Background(), fixture.item.ID, 1, draft.Content); err != nil {
				t.Fatal(err)
			}
			injected := errors.New("interrupt")
			fixture.service.failpoint = func(got string) error {
				if got == stage {
					return injected
				}
				return nil
			}
			if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 2}); !errors.Is(err, injected) {
				t.Fatalf("Publish error = %v", err)
			}
			fixture = reopenPublicationFixture(t, fixture)
			if err := fixture.service.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			got, err := fixture.repo.Get(context.Background(), fixture.item.ID)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := fixture.service.ReadPublished(context.Background(), fixture.item.ID)
			if err != nil {
				t.Fatalf("recovered config = %s, %v", raw, err)
			}
			wantRevision := uint64(2)
			wantLevel, wantStrategy := "debug", "ipv4_only"
			if stage == "after_generation_rename" {
				wantRevision, wantLevel, wantStrategy = 1, "info", "prefer_ipv4"
			}
			if got.PublishedRevision == nil || *got.PublishedRevision != wantRevision || !bytes.Contains(raw, []byte(`"level":"`+wantLevel+`"`)) || !bytes.Contains(raw, []byte(`"strategy":"`+wantStrategy+`"`)) {
				t.Fatalf("mixed recovery: profile=%#v config=%s", got, raw)
			}
			target, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.item.Slug))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(fixture.configRoot, target)); err != nil {
				t.Fatalf("referenced generation removed: %v", err)
			}
		})
	}
}

func TestDeleteInterruptionAfterRenameCompletesOnRecovery(t *testing.T) {
	fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
	if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("interrupt")
	fixture.service.failpoint = func(stage string) error {
		if stage == "after_delete_rename" {
			return injected
		}
		return nil
	}
	if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); !errors.Is(err, injected) {
		t.Fatalf("Delete error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.configRoot, fixture.item.Slug)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stable exists: %v", err)
	}
	markers, err := fixture.service.deletionMarkers(fixture.item.ID)
	if err != nil || len(markers) != 1 {
		t.Fatalf("tombstone missing: %#v, %v", markers, err)
	}
	fixture = reopenPublicationFixture(t, fixture)
	if err := fixture.service.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repo.Get(context.Background(), fixture.item.ID); err == nil {
		t.Fatal("profile metadata remains")
	}
	if _, err := os.Lstat(fixture.repo.DraftPath(fixture.item.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("draft remains: %v", err)
	}
	if _, err := os.Lstat(markers[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tombstone remains: %v", err)
	}
	if _, err := os.Lstat(fixture.service.profileGenerationRoot(fixture.item.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generations remain: %v", err)
	}
}

func TestRecoverRejectsConflictingOrInvalidDeleteTombstone(t *testing.T) {
	for _, variant := range []string{"valid-without-intent", "stable-and-tombstone", "regular", "outside-link"} {
		t.Run(variant, func(t *testing.T) {
			fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
			if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
				t.Fatal(err)
			}
			stable := filepath.Join(fixture.configRoot, fixture.item.Slug)
			target, err := os.Readlink(stable)
			if err != nil {
				t.Fatal(err)
			}
			tombstone := fixture.service.deleteTombstone(fixture.item.ID, strings.Repeat("03", 16))
			if err := os.Rename(stable, tombstone); err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "valid-without-intent":
			case "stable-and-tombstone":
				if err := os.Symlink(target, stable); err != nil {
					t.Fatal(err)
				}
			case "regular":
				if err := os.Remove(tombstone); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(tombstone, []byte("occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "outside-link":
				if err := os.Remove(tombstone); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../outside", tombstone); err != nil {
					t.Fatal(err)
				}
			}
			if err := fixture.service.Recover(context.Background()); !errors.Is(err, ErrExternalDrift) {
				t.Fatalf("Recover error = %v", err)
			}
			if _, err := fixture.repo.Get(context.Background(), fixture.item.ID); err != nil {
				t.Fatalf("metadata removed: %v", err)
			}
			if _, err := os.Lstat(fixture.repo.DraftPath(fixture.item.ID)); err != nil {
				t.Fatalf("draft removed: %v", err)
			}
			if _, err := os.Lstat(tombstone); err != nil {
				t.Fatalf("tombstone removed: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(fixture.configRoot, target)); err != nil {
				t.Fatalf("generation removed: %v", err)
			}
		})
	}
}

func TestRecoverDeletionRejectsSymlinkedGenerationParentsWithoutDeleting(t *testing.T) {
	for _, parentKind := range []string{"generations", "profile"} {
		t.Run(parentKind, func(t *testing.T) {
			fixture := newPublicationFixture(t, fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}})
			if _, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.item.ID, ExpectedRevision: 1}); err != nil {
				t.Fatal(err)
			}
			injected := errors.New("interrupt")
			fixture.service.failpoint = func(stage string) error {
				if stage == "after_delete_rename" {
					return injected
				}
				return nil
			}
			if err := fixture.service.Delete(context.Background(), fixture.item.ID, fixture.item.Name); !errors.Is(err, injected) {
				t.Fatalf("Delete error = %v", err)
			}
			outside := t.TempDir()
			if err := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			parent := fixture.service.profileGenerationRoot(fixture.item.ID)
			if parentKind == "generations" {
				parent = filepath.Join(fixture.configRoot, ".generations")
			}
			backup := parent + ".backup"
			if err := os.Rename(parent, backup); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, parent); err != nil {
				t.Fatal(err)
			}
			fixture = reopenPublicationFixture(t, fixture)
			if err := fixture.service.Recover(context.Background()); !errors.Is(err, ErrExternalDrift) {
				t.Fatalf("Recover error = %v", err)
			}
			if _, err := fixture.repo.Get(context.Background(), fixture.item.ID); err != nil {
				t.Fatalf("profile removed: %v", err)
			}
			if _, err := os.Lstat(fixture.repo.DraftPath(fixture.item.ID)); err != nil {
				t.Fatalf("draft removed: %v", err)
			}
			if content, err := os.ReadFile(filepath.Join(outside, "sentinel")); err != nil || string(content) != "keep" {
				t.Fatalf("outside touched: %q, %v", content, err)
			}
			if markers, err := fixture.service.deletionMarkers(fixture.item.ID); err != nil || len(markers) != 1 {
				t.Fatalf("tombstone changed: %#v, %v", markers, err)
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
	dataDir    string
	now        func() time.Time
}

func newPublicationFixture(t *testing.T, checker Checker) publicationFixture {
	t.Helper()
	now := func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
	dataDir := t.TempDir()
	repo, err := profile.OpenRepository(dataDir, now)
	if err != nil {
		t.Fatal(err)
	}
	item := profile.Profile{ID: "01010101010101010101010101010101", Name: "Primary", Slug: "primary", DraftRevision: 1}
	draft := profile.Draft{ProfileID: item.ID, Revision: 1, Content: profile.DraftContent{Config: []byte(`{"log":{"level":"info"},"dns":{"strategy":"prefer_ipv4"}}`), EditorBuffers: map[string]profile.EditorBuffer{}}}
	if err := repo.Insert(context.Background(), item, draft); err != nil {
		t.Fatal(err)
	}
	schema, err := validation.NewSchemaValidator([]byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	configRoot := filepath.Join(t.TempDir(), "config")
	return publicationFixture{repo: repo, item: item, configRoot: configRoot, dataDir: dataDir, now: now, service: NewService(Options{ConfigRoot: configRoot, Profiles: repo, Preflight: validation.NewService(schema, "target", now), Checker: checker, Now: now, Random: bytes.NewReader(bytes.Repeat([]byte{1}, 1024)), FileMode: 0o640, DirMode: 0o750})}
}

func reopenPublicationFixture(t *testing.T, fixture publicationFixture) publicationFixture {
	t.Helper()
	repo, err := profile.OpenRepository(fixture.dataDir, fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := validation.NewSchemaValidator([]byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	fixture.repo = repo
	fixture.service = NewService(Options{ConfigRoot: fixture.configRoot, Profiles: repo, Preflight: validation.NewService(schema, "target", fixture.now), Checker: fakeChecker{version: "v", result: singbox.CheckResult{Valid: true}}, Now: fixture.now, Random: bytes.NewReader(bytes.Repeat([]byte{2}, 1024)), FileMode: 0o640, DirMode: 0o750})
	return fixture
}
