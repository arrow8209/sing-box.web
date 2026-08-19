package publication

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"singbox-web/internal/configdoc"
	"singbox-web/internal/profile"
	"singbox-web/internal/singbox"
	"singbox-web/internal/validation"
)

var (
	ErrExternalDrift    = errors.New("published configuration has external drift")
	ErrPathOccupied     = errors.New("stable publication path is occupied")
	ErrRecoveryRequired = errors.New("publication recovery is required")
	ErrNotPublished     = errors.New("profile is not published")
	ErrValidationFailed = errors.New("configuration validation failed")
)

type Checker interface {
	Version(context.Context) (string, error)
	Check(context.Context, string) (singbox.CheckResult, error)
}

type Options struct {
	ConfigRoot string
	Profiles   *profile.Repository
	Preflight  *validation.Service
	Checker    Checker
	Now        func() time.Time
	Random     io.Reader
	FileMode   fs.FileMode
	DirMode    fs.FileMode
	GroupID    *int
}

type Request struct {
	ProfileID                string `json:"-"`
	ExpectedRevision         uint64 `json:"-"`
	ConfirmExternalOverwrite bool   `json:"confirmExternalOverwrite"`
}

type Receipt struct {
	ProfileID   string            `json:"profileId"`
	Revision    uint64            `json:"revision"`
	Digest      string            `json:"digest"`
	Directory   string            `json:"directory"`
	PublishedAt time.Time         `json:"publishedAt"`
	Validation  validation.Result `json:"validation"`
}

type Service struct {
	options          Options
	operationMu      sync.RWMutex
	locksMu          sync.Mutex
	locks            map[string]*sync.Mutex
	randomMu         sync.Mutex
	stateMu          sync.RWMutex
	recoveryRequired bool
	failpoint        func(stage string) error
	syncDirectory    func(string) error
}

func NewService(options Options) *Service {
	if options.FileMode == 0 {
		options.FileMode = 0o640
	}
	if options.DirMode == 0 {
		options.DirMode = 0o750
	}
	return &Service{options: options, locks: make(map[string]*sync.Mutex), syncDirectory: syncDirectory}
}

type CheckerError struct{ classification error }

func (err *CheckerError) Error() string { return "sing-box check 失败" }
func (err *CheckerError) Unwrap() error { return err.classification }

func safeCheckerError(cause error) error {
	classification := error(singbox.ErrCheckFailed)
	for _, trusted := range []error{singbox.ErrUnavailable, singbox.ErrTimeout, singbox.ErrCheckFailed} {
		if errors.Is(cause, trusted) {
			classification = trusted
			break
		}
	}
	return &CheckerError{classification: classification}
}

func (service *Service) Publish(ctx context.Context, request Request) (Receipt, error) {
	service.operationMu.RLock()
	defer service.operationMu.RUnlock()
	if service.requiresRecovery() {
		return Receipt{}, ErrRecoveryRequired
	}
	lock := service.profileLock(request.ProfileID)
	lock.Lock()
	defer lock.Unlock()
	if service.requiresRecovery() {
		return Receipt{}, ErrRecoveryRequired
	}
	if err := service.valid(); err != nil {
		return Receipt{}, err
	}
	item, err := service.options.Profiles.Get(ctx, request.ProfileID)
	if err != nil {
		return Receipt{}, err
	}
	if item.DraftRevision != request.ExpectedRevision {
		return Receipt{}, &profile.RevisionConflict{Current: item.DraftRevision}
	}
	draft, err := service.loadDraft(ctx, item, request.ExpectedRevision)
	if err != nil {
		return Receipt{}, err
	}
	preflight := service.options.Preflight.ValidateDraft(draft)
	if !preflight.Valid || !profile.IsPublishable(draft) {
		stage := "json"
		if len(preflight.Issues) > 0 {
			stage = preflight.Issues[0].Stage
		}
		summary := profile.ValidationSummary{Revision: request.ExpectedRevision, Status: "invalid", Stage: stage, Message: fmt.Sprintf("发现 %d 个校验问题", len(preflight.Issues)), CheckedAt: preflight.CheckedAt}
		if _, recordErr := service.options.Profiles.RecordValidation(ctx, item.ID, request.ExpectedRevision, summary); recordErr != nil {
			return Receipt{}, recordErr
		}
		return Receipt{ProfileID: item.ID, Revision: request.ExpectedRevision, Validation: preflight}, ErrValidationFailed
	}
	files, err := configdoc.Split(draft.Content.Config)
	if err != nil {
		return Receipt{}, err
	}
	digest := configdoc.Digest(files)
	oldTarget, exists, managed, drift, err := service.inspectStable(item)
	if err != nil {
		return Receipt{}, err
	}
	if !managed && exists {
		if item.PublishedRevision == nil {
			return Receipt{}, ErrPathOccupied
		}
		return Receipt{}, ErrExternalDrift
	}
	if item.PublishedRevision != nil && !exists {
		return Receipt{}, ErrExternalDrift
	}
	if item.PublishedRevision == nil && exists {
		return Receipt{}, ErrPathOccupied
	}
	if drift && !request.ConfirmExternalOverwrite {
		return Receipt{}, ErrExternalDrift
	}
	installedVersion, err := service.options.Checker.Version(ctx)
	if err != nil {
		return Receipt{}, err
	}
	if err := service.prepareRoot(); err != nil {
		return Receipt{}, err
	}
	parent := service.profileGenerationRoot(item.ID)
	if err := service.prepareProfileRoot(item.ID); err != nil {
		return Receipt{}, err
	}
	stagingName, err := service.randomName(".staging-")
	if err != nil {
		return Receipt{}, err
	}
	staging := filepath.Join(parent, stagingName)
	if err := os.Mkdir(staging, service.options.DirMode); err != nil {
		return Receipt{}, err
	}
	if err := os.Chmod(staging, service.options.DirMode); err != nil {
		service.removeAndSync(staging, parent)
		return Receipt{}, err
	}
	if service.options.GroupID != nil {
		if err := os.Chown(staging, -1, *service.options.GroupID); err != nil {
			service.removeAndSync(staging, parent)
			return Receipt{}, err
		}
	}
	if err := writeModules(staging, files, service.options.FileMode, service.options.GroupID); err != nil {
		service.removeAndSync(staging, parent)
		return Receipt{}, err
	}
	check, checkErr := service.options.Checker.Check(ctx, staging)
	if checkErr != nil || !check.Valid {
		service.removeAndSync(staging, parent)
		summary := profile.ValidationSummary{Revision: request.ExpectedRevision, Status: "invalid", Stage: "singbox", Message: "sing-box check 失败", CheckedAt: service.options.Now().UTC()}
		if _, recordErr := service.options.Profiles.RecordValidation(ctx, item.ID, request.ExpectedRevision, summary); recordErr != nil {
			return Receipt{}, recordErr
		}
		if checkErr != nil {
			return Receipt{}, safeCheckerError(checkErr)
		}
		return Receipt{}, ErrValidationFailed
	}
	currentTarget, currentExists, currentManaged, currentDrift, inspectErr := service.inspectStable(item)
	if inspectErr != nil {
		service.removeAndSync(staging, parent)
		return Receipt{}, inspectErr
	}
	if currentExists != exists || currentTarget != oldTarget || currentManaged != managed {
		service.removeAndSync(staging, parent)
		if item.PublishedRevision == nil {
			return Receipt{}, ErrPathOccupied
		}
		return Receipt{}, ErrExternalDrift
	}
	if currentDrift && !request.ConfirmExternalOverwrite {
		service.removeAndSync(staging, parent)
		return Receipt{}, ErrExternalDrift
	}
	drift = currentDrift
	publishedAt := service.options.Now().UTC()
	if err := os.Chtimes(staging, publishedAt, publishedAt); err != nil {
		service.removeAndSync(staging, parent)
		return Receipt{}, err
	}
	if err := syncDirectory(staging); err != nil {
		service.removeAndSync(staging, parent)
		return Receipt{}, err
	}
	publicationID := fmt.Sprintf("%d-%s", request.ExpectedRevision, digest[:12])
	generation := filepath.Join(parent, publicationID)
	exchanged := false
	if _, statErr := os.Lstat(generation); statErr == nil {
		if !drift || !request.ConfirmExternalOverwrite || oldTarget != filepath.Join(".generations", item.ID, publicationID) {
			service.removeAndSync(staging, parent)
			return Receipt{}, ErrExternalDrift
		}
		if err := unix.Renameat2(unix.AT_FDCWD, staging, unix.AT_FDCWD, generation, unix.RENAME_EXCHANGE); err != nil {
			service.removeAndSync(staging, parent)
			return Receipt{}, fmt.Errorf("exchange drifted generation: %w", err)
		}
		exchanged = true
	} else if !errors.Is(statErr, os.ErrNotExist) {
		service.removeAndSync(staging, parent)
		return Receipt{}, statErr
	} else if err := os.Rename(staging, generation); err != nil {
		service.removeAndSync(staging, parent)
		return Receipt{}, err
	}
	if err := syncDirectory(parent); err != nil {
		return Receipt{}, err
	}
	if err := service.callFailpoint("after_generation_rename"); err != nil {
		return Receipt{}, err
	}
	relativeTarget := filepath.Join(".generations", item.ID, publicationID)
	if err := service.swapLink(item.Slug, relativeTarget, oldTarget, exists); err != nil {
		return Receipt{}, err
	}
	if err := service.callFailpoint("after_link_swap"); err != nil {
		return Receipt{}, err
	}
	preflight.InstalledVersion = installedVersion
	validationSummary := profile.ValidationSummary{Revision: request.ExpectedRevision, Status: "valid", Stage: "singbox", Message: "sing-box check 通过", CheckedAt: publishedAt}
	receipt := Receipt{ProfileID: item.ID, Revision: request.ExpectedRevision, Digest: digest, Directory: relativeTarget, PublishedAt: publishedAt, Validation: preflight}
	metadataErr := service.callFailpoint("before_metadata")
	if metadataErr == nil {
		_, metadataErr = service.options.Profiles.RecordPublication(ctx, item.ID, request.ExpectedRevision, profile.PublicationState{Revision: request.ExpectedRevision, Digest: digest, PublishedAt: publishedAt, Validation: validationSummary})
	}
	if metadataErr != nil {
		if errors.Is(metadataErr, profile.ErrIndexDurabilityUncertain) {
			if exchanged {
				service.removeAndSync(staging, parent)
			}
			return receipt, metadataErr
		}
		if exchanged {
			if exchangeErr := unix.Renameat2(unix.AT_FDCWD, staging, unix.AT_FDCWD, generation, unix.RENAME_EXCHANGE); exchangeErr != nil {
				service.setRecoveryRequired(true)
				return Receipt{}, fmt.Errorf("%w: metadata: %v; generation rollback: %v", ErrRecoveryRequired, metadataErr, exchangeErr)
			}
			service.removeAndSync(staging, parent)
		}
		if rollbackErr := service.restoreLink(item.Slug, oldTarget, relativeTarget, exists); rollbackErr != nil {
			service.setRecoveryRequired(true)
			return Receipt{}, fmt.Errorf("%w: metadata: %v; link rollback: %v", ErrRecoveryRequired, metadataErr, rollbackErr)
		}
		if !exchanged {
			service.removeAndSync(generation, parent)
		}
		return Receipt{}, metadataErr
	}
	if err := service.callFailpoint("after_metadata"); err != nil {
		return Receipt{}, err
	}
	if exchanged {
		service.removeAndSync(staging, parent)
	}
	if exists && oldTarget != relativeTarget {
		if err := service.removeOldGeneration(item.ID, oldTarget, parent); err != nil {
			return receipt, err
		}
	}
	return receipt, nil
}

func (service *Service) ReadPublished(ctx context.Context, profileID string) (json.RawMessage, error) {
	service.operationMu.RLock()
	defer service.operationMu.RUnlock()
	if err := service.valid(); err != nil {
		return nil, err
	}
	item, err := service.options.Profiles.Get(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if item.PublishedRevision == nil {
		return nil, ErrNotPublished
	}
	_, exists, managed, drift, err := service.inspectStable(item)
	if err != nil {
		return nil, err
	}
	if !exists || !managed || drift {
		return nil, ErrExternalDrift
	}
	target, _ := os.Readlink(filepath.Join(service.options.ConfigRoot, item.Slug))
	files, err := readModules(filepath.Join(service.options.ConfigRoot, target))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExternalDrift, err)
	}
	if configdoc.Digest(files) != item.PublishedDigest {
		return nil, ErrExternalDrift
	}
	return configdoc.Merge(files)
}

func (service *Service) Recover(ctx context.Context) error {
	service.operationMu.Lock()
	defer service.operationMu.Unlock()
	if err := service.valid(); err != nil {
		return err
	}
	items, err := service.options.Profiles.List(ctx)
	if err != nil {
		return err
	}
	pending, err := service.options.Profiles.PendingDeletions(ctx)
	if err != nil {
		return err
	}
	intentByProfile := make(map[string]profile.DeletionIntent, len(pending))
	for _, intent := range pending {
		intentByProfile[intent.ProfileID] = intent
	}
	for _, item := range items {
		markers, err := service.deletionMarkers(item.ID)
		if err != nil {
			return err
		}
		if intent, exists := intentByProfile[item.ID]; exists {
			if err := service.recoverDeletion(ctx, item, intent, markers); err != nil {
				return err
			}
			continue
		}
		if len(markers) != 0 {
			return ErrExternalDrift
		}
		stable := filepath.Join(service.options.ConfigRoot, item.Slug)
		info, lstatErr := os.Lstat(stable)
		if errors.Is(lstatErr, os.ErrNotExist) {
			if item.PublishedRevision != nil {
				if _, err := service.options.Profiles.RepairPublication(ctx, item.ID, nil); err != nil {
					return err
				}
			}
			if err := service.cleanupGenerations(item.ID, ""); err != nil {
				return err
			}
			continue
		}
		if lstatErr != nil {
			return lstatErr
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return ErrExternalDrift
		}
		target, err := os.Readlink(stable)
		if err != nil {
			return err
		}
		publicationID, generation, err := service.ownedTarget(item.ID, target)
		if err != nil {
			return ErrExternalDrift
		}
		revision, prefix, err := parsePublicationID(publicationID)
		if err != nil {
			return ErrExternalDrift
		}
		files, err := readModules(generation)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrExternalDrift, err)
		}
		digest := configdoc.Digest(files)
		if !strings.HasPrefix(digest, prefix) {
			return ErrExternalDrift
		}
		generationInfo, err := os.Lstat(generation)
		if err != nil {
			return err
		}
		publishedAt := generationInfo.ModTime().UTC()
		validationSummary := profile.ValidationSummary{Revision: revision, Status: "valid", Stage: "recovery", Message: "由磁盘发布状态恢复", CheckedAt: publishedAt}
		state := profile.PublicationState{Revision: revision, Digest: digest, PublishedAt: publishedAt, Validation: validationSummary}
		if !publicationMatches(item, state) {
			if _, err := service.options.Profiles.RepairPublication(ctx, item.ID, &state); err != nil {
				return err
			}
		}
		if err := service.cleanupGenerations(item.ID, publicationID); err != nil {
			return err
		}
	}
	service.setRecoveryRequired(false)
	return nil
}

func (service *Service) Delete(ctx context.Context, profileID, confirmName string) error {
	service.operationMu.RLock()
	defer service.operationMu.RUnlock()
	if service.requiresRecovery() {
		return ErrRecoveryRequired
	}
	lock := service.profileLock(profileID)
	lock.Lock()
	defer lock.Unlock()
	if service.requiresRecovery() {
		return ErrRecoveryRequired
	}
	if err := service.valid(); err != nil {
		return err
	}
	item, err := service.options.Profiles.Get(ctx, profileID)
	if err != nil {
		return err
	}
	if confirmName == "" || confirmName != item.Name {
		return fmt.Errorf("profile name confirmation does not match")
	}
	target, exists, managed, drift, err := service.inspectStable(item)
	if err != nil {
		return err
	}
	if exists && !managed {
		return ErrExternalDrift
	}
	if drift {
		return ErrExternalDrift
	}
	if item.PublishedRevision != nil && !exists {
		return ErrExternalDrift
	}
	if exists {
		stableInfo, err := os.Lstat(filepath.Join(service.options.ConfigRoot, item.Slug))
		if err != nil || stableInfo.Mode()&os.ModeSymlink == 0 {
			return ErrExternalDrift
		}
		stableTarget, err := os.Readlink(filepath.Join(service.options.ConfigRoot, item.Slug))
		if err != nil || stableTarget != target {
			return ErrExternalDrift
		}
		txID, err := service.randomTransactionID()
		if err != nil {
			return err
		}
		intent := profile.DeletionIntent{ProfileID: item.ID, Target: target, Digest: item.PublishedDigest, TxID: txID, Phase: profile.DeletionPhasePrepared}
		deletionTombstone := service.deleteTombstone(item.ID, txID)
		if err := service.options.Profiles.BeginDeletion(ctx, item.ID, intent); err != nil {
			if errors.Is(err, profile.ErrIndexDurabilityUncertain) {
				service.setRecoveryRequired(true)
			}
			return err
		}
		if err := service.callFailpoint("after_delete_intent"); err != nil {
			return err
		}
		moved, err := service.deleteStable(item, intent, stableInfo)
		if err != nil {
			if !moved {
				if cancelErr := service.options.Profiles.CancelDeletion(ctx, item.ID, txID); cancelErr != nil {
					service.setRecoveryRequired(true)
					return fmt.Errorf("%w: cancel deletion intent failed", ErrRecoveryRequired)
				}
			}
			return err
		}
		if err := service.options.Profiles.TransitionDeletionPhase(ctx, item.ID, txID, profile.DeletionPhasePrepared, profile.DeletionPhaseDiskRemovalAuthorized); err != nil {
			service.setRecoveryRequired(true)
			return fmt.Errorf("%w: authorize deletion: %v", ErrRecoveryRequired, err)
		}
		intent.Phase = profile.DeletionPhaseDiskRemovalAuthorized
		if err := service.callFailpoint("after_delete_link"); err != nil {
			return err
		}
		return service.completeAuthorizedDeletion(ctx, item, deletionTombstone)
	}
	if err := service.removeProfileGenerations(item.ID); err != nil {
		return err
	}
	if err := service.callFailpoint("after_delete_generations"); err != nil {
		return err
	}
	if err := service.callFailpoint("before_delete_metadata"); err != nil {
		return err
	}
	return service.options.Profiles.RemoveMetadata(ctx, profileID)
}

func (service *Service) valid() error {
	if service == nil || service.options.Profiles == nil || service.options.Preflight == nil || service.options.Checker == nil || service.options.Now == nil || service.options.Random == nil || service.options.ConfigRoot == "" {
		return fmt.Errorf("invalid publication service options")
	}
	if service.options.FileMode&^fs.FileMode(0o777) != 0 || service.options.DirMode&^fs.FileMode(0o777) != 0 {
		return fmt.Errorf("invalid publication permissions")
	}
	return nil
}

func (service *Service) loadDraft(ctx context.Context, item profile.Profile, expected uint64) (profile.Draft, error) {
	draft, err := profile.NewService(service.options.Profiles, nil, nil).GetDraft(ctx, item.ID)
	if err != nil {
		return profile.Draft{}, err
	}
	if draft.ProfileID != item.ID || draft.Revision != expected {
		return profile.Draft{}, &profile.RevisionConflict{Current: draft.Revision}
	}
	return draft, nil
}

func (service *Service) prepareRoot() error {
	if _, err := ensureDirectory(service.options.ConfigRoot, service.options.DirMode, nil, service.syncDirectory); err != nil {
		return err
	}
	generations := filepath.Join(service.options.ConfigRoot, ".generations")
	created, err := ensureDirectory(generations, service.options.DirMode, service.options.GroupID, service.syncDirectory)
	if err != nil {
		return err
	}
	if created {
		return service.syncDirectory(service.options.ConfigRoot)
	}
	return nil
}

func (service *Service) profileGenerationRoot(id string) string {
	if id == "" {
		return filepath.Join(service.options.ConfigRoot, ".generations")
	}
	return filepath.Join(service.options.ConfigRoot, ".generations", id)
}

func (service *Service) prepareProfileRoot(id string) error {
	created, err := ensureDirectory(service.profileGenerationRoot(id), service.options.DirMode, service.options.GroupID, service.syncDirectory)
	if err != nil {
		return err
	}
	if created {
		return service.syncDirectory(filepath.Join(service.options.ConfigRoot, ".generations"))
	}
	return nil
}

func (service *Service) inspectStable(item profile.Profile) (string, bool, bool, bool, error) {
	stable := filepath.Join(service.options.ConfigRoot, item.Slug)
	info, err := os.Lstat(stable)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, false, false, nil
	}
	if err != nil {
		return "", false, false, false, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", true, false, false, nil
	}
	target, err := os.Readlink(stable)
	if err != nil {
		return "", true, false, false, err
	}
	publicationID, generation, err := service.ownedTarget(item.ID, target)
	if err != nil {
		return target, true, false, false, nil
	}
	revision, prefix, parseErr := parsePublicationID(publicationID)
	if parseErr != nil {
		return target, true, false, false, nil
	}
	drift := false
	files, readErr := readModules(generation)
	if readErr != nil {
		drift = true
	} else {
		digest := configdoc.Digest(files)
		if !strings.HasPrefix(digest, prefix) {
			drift = true
		}
		if item.PublishedDigest != digest {
			drift = true
		}
	}
	if item.PublishedRevision == nil || *item.PublishedRevision != revision || item.PublishedDigest == "" {
		drift = true
	}
	return target, true, true, drift, nil
}

func (service *Service) ownedTarget(profileID, target string) (string, string, error) {
	if filepath.IsAbs(target) || filepath.Clean(target) != target {
		return "", "", ErrExternalDrift
	}
	parts := strings.Split(filepath.ToSlash(target), "/")
	if len(parts) != 3 || parts[0] != ".generations" || parts[1] != profileID || parts[2] == "" || parts[2] == "." || parts[2] == ".." {
		return "", "", ErrExternalDrift
	}
	generation := filepath.Join(service.options.ConfigRoot, filepath.FromSlash(target))
	expectedParent := filepath.Clean(service.profileGenerationRoot(profileID))
	if filepath.Dir(filepath.Clean(generation)) != expectedParent {
		return "", "", ErrExternalDrift
	}
	if err := service.validateGenerationParents(profileID); err != nil {
		return "", "", err
	}
	return parts[2], generation, nil
}

func (service *Service) validateGenerationParents(profileID string) error {
	for _, path := range []string{service.options.ConfigRoot, filepath.Join(service.options.ConfigRoot, ".generations"), service.profileGenerationRoot(profileID)} {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrExternalDrift
		}
	}
	return nil
}

func (service *Service) randomName(prefix string) (string, error) {
	service.randomMu.Lock()
	defer service.randomMu.Unlock()
	bytes := make([]byte, 8)
	if _, err := io.ReadFull(service.options.Random, bytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(bytes), nil
}

func (service *Service) randomTransactionID() (string, error) {
	service.randomMu.Lock()
	defer service.randomMu.Unlock()
	value := make([]byte, 16)
	if _, err := io.ReadFull(service.options.Random, value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (service *Service) swapLink(slug, target, expectedOldTarget string, existed bool) error {
	tempName, err := service.randomName(".link-")
	if err != nil {
		return err
	}
	temp := filepath.Join(service.options.ConfigRoot, tempName)
	if err := os.Symlink(target, temp); err != nil {
		return err
	}
	stable := filepath.Join(service.options.ConfigRoot, slug)
	if !existed {
		if err := unix.Renameat2(unix.AT_FDCWD, temp, unix.AT_FDCWD, stable, unix.RENAME_NOREPLACE); err != nil {
			_ = os.Remove(temp)
			if errors.Is(err, unix.EEXIST) {
				return ErrPathOccupied
			}
			return err
		}
		if err := syncDirectory(service.options.ConfigRoot); err != nil {
			if rollbackErr := os.Remove(stable); rollbackErr != nil {
				service.setRecoveryRequired(true)
				return fmt.Errorf("%w: link sync: %v; rollback: %v", ErrRecoveryRequired, err, rollbackErr)
			}
			_ = syncDirectory(service.options.ConfigRoot)
			return err
		}
		return nil
	}
	if err := unix.Renameat2(unix.AT_FDCWD, temp, unix.AT_FDCWD, stable, unix.RENAME_EXCHANGE); err != nil {
		_ = os.Remove(temp)
		return err
	}
	info, lstatErr := os.Lstat(temp)
	oldTarget, readErr := os.Readlink(temp)
	if lstatErr != nil || info.Mode()&os.ModeSymlink == 0 || readErr != nil || oldTarget != expectedOldTarget {
		rollbackErr := unix.Renameat2(unix.AT_FDCWD, temp, unix.AT_FDCWD, stable, unix.RENAME_EXCHANGE)
		if rollbackErr != nil {
			service.setRecoveryRequired(true)
			return fmt.Errorf("%w: stable path raced and rollback failed: %v", ErrRecoveryRequired, rollbackErr)
		}
		_ = os.Remove(temp)
		return ErrExternalDrift
	}
	if err := syncDirectory(service.options.ConfigRoot); err != nil {
		rollbackErr := unix.Renameat2(unix.AT_FDCWD, temp, unix.AT_FDCWD, stable, unix.RENAME_EXCHANGE)
		_ = os.Remove(temp)
		_ = syncDirectory(service.options.ConfigRoot)
		if rollbackErr != nil {
			service.setRecoveryRequired(true)
			return fmt.Errorf("%w: link sync: %v; rollback: %v", ErrRecoveryRequired, err, rollbackErr)
		}
		return err
	}
	if err := os.Remove(temp); err != nil {
		return err
	}
	return syncDirectory(service.options.ConfigRoot)
}

func (service *Service) deleteStable(item profile.Profile, intent profile.DeletionIntent, expectedInfo fs.FileInfo) (bool, error) {
	stable := filepath.Join(service.options.ConfigRoot, item.Slug)
	tombstone := service.deleteTombstone(item.ID, intent.TxID)
	if _, err := os.Lstat(tombstone); err == nil || !errors.Is(err, os.ErrNotExist) {
		return false, ErrExternalDrift
	}
	if err := service.callFailpoint("before_delete_exchange"); err != nil {
		return false, err
	}
	if err := unix.Renameat2(unix.AT_FDCWD, stable, unix.AT_FDCWD, tombstone, unix.RENAME_NOREPLACE); err != nil {
		return false, ErrExternalDrift
	}
	if err := service.syncDirectory(service.options.ConfigRoot); err != nil {
		return false, service.rollbackDelete(stable, tombstone, err)
	}
	if err := service.callFailpoint("after_delete_rename"); err != nil {
		return true, err
	}
	movedInfo, lstatErr := os.Lstat(tombstone)
	movedTarget, readErr := os.Readlink(tombstone)
	if lstatErr != nil || movedInfo.Mode()&os.ModeSymlink == 0 || readErr != nil || movedTarget != intent.Target || !os.SameFile(expectedInfo, movedInfo) {
		return false, service.rollbackDelete(stable, tombstone, ErrExternalDrift)
	}
	if err := service.callFailpoint("before_delete_digest"); err != nil {
		return false, service.rollbackDelete(stable, tombstone, err)
	}
	matches, err := service.intentTargetMatches(intent, false)
	if err != nil || !matches {
		return false, service.rollbackDelete(stable, tombstone, ErrExternalDrift)
	}
	return true, nil
}

func (service *Service) deleteTombstone(profileID, txID string) string {
	return filepath.Join(service.options.ConfigRoot, ".delete-"+profileID+"-"+txID)
}

func (service *Service) deletionMarkers(profileID string) ([]string, error) {
	entries, err := os.ReadDir(service.options.ConfigRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	prefix := ".delete-" + profileID
	var result []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			result = append(result, filepath.Join(service.options.ConfigRoot, entry.Name()))
		}
	}
	return result, nil
}

func (service *Service) recoverDeletion(ctx context.Context, item profile.Profile, intent profile.DeletionIntent, markers []string) error {
	stable := filepath.Join(service.options.ConfigRoot, item.Slug)
	_, stableErr := os.Lstat(stable)
	expected := service.deleteTombstone(item.ID, intent.TxID)
	switch intent.Phase {
	case profile.DeletionPhasePrepared:
		if stableErr == nil {
			if len(markers) != 0 {
				return ErrExternalDrift
			}
			target, err := os.Readlink(stable)
			if err != nil || target != intent.Target {
				return ErrExternalDrift
			}
			matches, err := service.intentTargetMatches(intent, false)
			if err != nil || !matches {
				return ErrExternalDrift
			}
			return service.options.Profiles.CancelDeletion(ctx, item.ID, intent.TxID)
		}
		if !errors.Is(stableErr, os.ErrNotExist) {
			return stableErr
		}
		if len(markers) == 0 {
			return ErrExternalDrift
		}
		if len(markers) != 1 || markers[0] != expected || !service.validDeletionMarker(intent, expected, false) {
			return ErrExternalDrift
		}
		if err := service.options.Profiles.TransitionDeletionPhase(ctx, item.ID, intent.TxID, profile.DeletionPhasePrepared, profile.DeletionPhaseDiskRemovalAuthorized); err != nil {
			service.setRecoveryRequired(true)
			return fmt.Errorf("%w: authorize recovered deletion: %v", ErrRecoveryRequired, err)
		}
		intent.Phase = profile.DeletionPhaseDiskRemovalAuthorized
		return service.completeAuthorizedDeletion(ctx, item, expected)
	case profile.DeletionPhaseDiskRemovalAuthorized:
		if stableErr == nil {
			return ErrExternalDrift
		}
		if !errors.Is(stableErr, os.ErrNotExist) {
			return stableErr
		}
		if len(markers) > 1 || (len(markers) == 1 && (markers[0] != expected || !service.validDeletionMarker(intent, expected, true))) {
			return ErrExternalDrift
		}
		marker := ""
		if len(markers) == 1 {
			marker = expected
		}
		return service.completeAuthorizedDeletion(ctx, item, marker)
	default:
		return ErrExternalDrift
	}
}

func (service *Service) validDeletionMarker(intent profile.DeletionIntent, marker string, allowMissingGeneration bool) bool {
	info, err := os.Lstat(marker)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(marker)
	if err != nil || target != intent.Target {
		return false
	}
	matches, err := service.intentTargetMatches(intent, allowMissingGeneration)
	return err == nil && matches
}

func (service *Service) completeAuthorizedDeletion(ctx context.Context, item profile.Profile, marker string) error {
	if marker != "" {
		if err := os.Remove(marker); err != nil {
			return err
		}
		if err := service.syncDirectory(service.options.ConfigRoot); err != nil {
			return err
		}
		if err := service.callFailpoint("after_delete_marker"); err != nil {
			return err
		}
	}
	if err := service.removeProfileGenerations(item.ID); err != nil {
		return err
	}
	if err := service.callFailpoint("after_delete_generations"); err != nil {
		return err
	}
	if err := service.callFailpoint("before_delete_metadata"); err != nil {
		return err
	}
	metadataErr := service.options.Profiles.RemoveMetadata(ctx, item.ID)
	if metadataErr != nil && !errors.Is(metadataErr, profile.ErrIndexDurabilityUncertain) {
		return metadataErr
	}
	return metadataErr
}

func (service *Service) rollbackDelete(stable, tombstone string, cause error) error {
	if _, err := os.Lstat(stable); !errors.Is(err, os.ErrNotExist) {
		service.setRecoveryRequired(true)
		return fmt.Errorf("%w: delete rollback destination occupied", ErrRecoveryRequired)
	}
	if err := unix.Renameat2(unix.AT_FDCWD, tombstone, unix.AT_FDCWD, stable, unix.RENAME_NOREPLACE); err != nil {
		service.setRecoveryRequired(true)
		return fmt.Errorf("%w: delete rollback failed", ErrRecoveryRequired)
	}
	if err := service.syncDirectory(service.options.ConfigRoot); err != nil {
		service.setRecoveryRequired(true)
		return fmt.Errorf("%w: delete rollback sync failed", ErrRecoveryRequired)
	}
	return cause
}

func (service *Service) intentTargetMatches(intent profile.DeletionIntent, allowMissingGeneration bool) (bool, error) {
	publicationID, generation, err := service.parseOwnedTarget(intent.ProfileID, intent.Target)
	if err != nil {
		return false, err
	}
	revision, prefix, err := parsePublicationID(publicationID)
	if err != nil {
		return false, err
	}
	if revision == 0 || !strings.HasPrefix(intent.Digest, prefix) {
		return false, nil
	}
	if err := service.validateGenerationParents(intent.ProfileID); err != nil {
		if allowMissingGeneration && errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	files, err := readModules(generation)
	if allowMissingGeneration && errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	digest := configdoc.Digest(files)
	return intent.Digest == digest, nil
}

func (service *Service) parseOwnedTarget(profileID, target string) (string, string, error) {
	if filepath.IsAbs(target) || filepath.Clean(target) != target {
		return "", "", ErrExternalDrift
	}
	parts := strings.Split(filepath.ToSlash(target), "/")
	if len(parts) != 3 || parts[0] != ".generations" || parts[1] != profileID || parts[2] == "" {
		return "", "", ErrExternalDrift
	}
	return parts[2], filepath.Join(service.options.ConfigRoot, filepath.FromSlash(target)), nil
}

func (service *Service) removeProfileGenerations(profileID string) error {
	root := service.profileGenerationRoot(profileID)
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := service.validateGenerationParents(profileID); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return service.syncDirectory(filepath.Join(service.options.ConfigRoot, ".generations"))
}

func (service *Service) restoreLink(slug, target, expectedCurrentTarget string, existed bool) error {
	if err := service.callFailpoint("before_link_rollback"); err != nil {
		return err
	}
	stable := filepath.Join(service.options.ConfigRoot, slug)
	if !existed {
		info, err := os.Lstat(stable)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return ErrExternalDrift
		}
		current, err := os.Readlink(stable)
		if err != nil || current != expectedCurrentTarget {
			return ErrExternalDrift
		}
		if err := os.Remove(stable); err != nil {
			return err
		}
		return syncDirectory(service.options.ConfigRoot)
	}
	return service.swapLink(slug, target, expectedCurrentTarget, true)
}

func (service *Service) removeOldGeneration(profileID, oldTarget, parent string) error {
	_, path, err := service.ownedTarget(profileID, oldTarget)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return syncDirectory(parent)
}

func (service *Service) removeAndSync(path, parent string) {
	_ = os.RemoveAll(path)
	_ = syncDirectory(parent)
}

func (service *Service) cleanupGenerations(profileID, referenced string) error {
	parent := service.profileGenerationRoot(profileID)
	info, lstatErr := os.Lstat(parent)
	if errors.Is(lstatErr, os.ErrNotExist) {
		return nil
	}
	if lstatErr != nil {
		return lstatErr
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrExternalDrift
	}
	if err := service.validateGenerationParents(profileID); err != nil {
		return err
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return err
	}
	changed := false
	for _, entry := range entries {
		name := entry.Name()
		remove := strings.HasPrefix(name, ".staging-")
		if !remove && name != referenced {
			_, _, parseErr := parsePublicationID(name)
			remove = parseErr == nil
		}
		if remove {
			if err := os.RemoveAll(filepath.Join(parent, name)); err != nil {
				return err
			}
			changed = true
		}
	}
	if changed {
		return syncDirectory(parent)
	}
	return nil
}

func (service *Service) profileLock(id string) *sync.Mutex {
	service.locksMu.Lock()
	defer service.locksMu.Unlock()
	if service.locks[id] == nil {
		service.locks[id] = &sync.Mutex{}
	}
	return service.locks[id]
}

func (service *Service) requiresRecovery() bool {
	service.stateMu.RLock()
	defer service.stateMu.RUnlock()
	return service.recoveryRequired
}
func (service *Service) setRecoveryRequired(value bool) {
	service.stateMu.Lock()
	service.recoveryRequired = value
	service.stateMu.Unlock()
}
func (service *Service) callFailpoint(stage string) error {
	if service.failpoint != nil {
		return service.failpoint(stage)
	}
	return nil
}

func publicationMatches(item profile.Profile, state profile.PublicationState) bool {
	return item.PublishedRevision != nil && *item.PublishedRevision == state.Revision && item.PublishedDigest == state.Digest && item.PublishedAt != nil && item.PublishedAt.Equal(state.PublishedAt)
}
