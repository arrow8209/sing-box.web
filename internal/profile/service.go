package profile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"singbox-web/internal/paths"
	"singbox-web/internal/templates"
)

const (
	maxModuleBytes = 1 << 20
	maxDraftBytes  = 4 << 20
)

type Service struct {
	templates       *templates.Factory
	defaultBindings DefaultBindingSource
	store           *store
	clock           func() time.Time
	random          io.Reader
	guard           DeleteGuard
	resourceDigests map[string]string
	resourceKnown   bool
	mu              sync.Mutex
}

func Open(layout paths.Layout, factory *templates.Factory, source DefaultBindingSource, clock func() time.Time, randomSource io.Reader) (*Service, error) {
	if factory == nil {
		return nil, fmt.Errorf("档案模板不能为空")
	}
	if source == nil {
		return nil, fmt.Errorf("默认规则绑定来源不能为空")
	}
	if clock == nil {
		clock = time.Now
	}
	if randomSource == nil {
		randomSource = rand.Reader
	}
	service := &Service{templates: factory, defaultBindings: source, store: newStore(layout), clock: clock, random: randomSource, guard: conservativeDeleteGuard{}, resourceDigests: map[string]string{}}
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, err := service.refreshMetas(); err != nil {
		return nil, err
	}
	return service, nil
}

// ValidateManagedState performs the read-only profiles gate used before any
// startup recovery or initialization may write managed state.
func ValidateManagedState(layout paths.Layout) error {
	_, err := newStore(layout).loadMetas()
	return err
}

// RecoverDeletingDrafts resolves interrupted deletions using the already
// validated profiles index as the commit record. NewServer calls this while it
// owns the exclusive application lock.
func RecoverDeletingDrafts(layout paths.Layout) error {
	store := newStore(layout)
	metas, err := store.loadMetas()
	if err != nil {
		return err
	}
	present := make(map[string]struct{}, len(metas))
	for _, meta := range metas {
		present[meta.ID] = struct{}{}
	}
	entries, err := os.ReadDir(layout.DraftsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	changed := false
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json.deleting") {
			continue
		}
		profileID := strings.TrimSuffix(name, ".json.deleting")
		if !profileIDPattern.MatchString(profileID) {
			continue
		}
		staged := filepath.Join(layout.DraftsDir, name)
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("检查待恢复草稿失败 %s: %w", staged, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("待恢复草稿不是普通文件: %s", staged)
		}
		if _, exists := present[profileID]; exists {
			canonical := store.draftPath(profileID)
			if _, err := os.Lstat(canonical); err == nil {
				return fmt.Errorf("草稿与待恢复草稿同时存在: %s", staged)
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(staged, canonical); err != nil {
				return fmt.Errorf("恢复草稿失败 %s: %w", staged, err)
			}
		} else if err := store.removeFile(staged); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理已删除档案草稿失败 %s: %w", staged, err)
		}
		changed = true
	}
	if changed {
		return syncDirectory(layout.DraftsDir)
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (s *Service) SetDeleteGuard(guard DeleteGuard) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if guard == nil {
		s.guard = conservativeDeleteGuard{}
		return
	}
	s.guard = guard
}

// conservativeDeleteGuard is used before the core operations layer can
// reliably identify the active profile. Refusing every deletion is safer than
// accidentally deleting the active configuration; Core Operations replaces it
// with a guard that can allow non-current profiles.
type conservativeDeleteGuard struct{}

func (conservativeDeleteGuard) GuardDelete(context.Context, string, bool) error {
	return ErrCurrentProfileRequiresCoreDecision
}

func (s *Service) List(ctx context.Context) ([]Meta, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshMetas()
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Meta, Draft, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return Meta{}, Draft{}, err
	}
	if err := ctx.Err(); err != nil {
		return Meta{}, Draft{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bindings, err := s.defaultBindings.DefaultBindings(ctx)
	if err != nil {
		return Meta{}, Draft{}, err
	}
	seed, err := s.templates.NewProfileSeed(s.clock().UTC())
	if err != nil {
		return Meta{}, Draft{}, err
	}
	id, err := randomHex(s.random, 16)
	if err != nil {
		return Meta{}, Draft{}, err
	}
	draft := Draft{ProfileID: id, Revision: 1, Config: seed.Config, Buffers: seed.Buffers, Bindings: bindings, SavedAt: seed.CreatedAt.UTC()}
	if err := validateDraftEnvelope(draft, maxModuleBytes, maxDraftBytes); err != nil {
		return Meta{}, Draft{}, err
	}
	meta := Meta{ID: id, Name: name, DraftRevision: 1, AppliedResourceDigests: map[string]string{}, NeedsApply: true}
	if err := s.store.create(meta, draft); err != nil {
		return Meta{}, Draft{}, err
	}
	return cloneMeta(meta), cloneDraft(draft), nil
}

func (s *Service) Rename(ctx context.Context, profileID string, input RenameInput) (Meta, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return Meta{}, err
	}
	if err := ctx.Err(); err != nil {
		return Meta{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metas, err := s.refreshMetas()
	if err != nil {
		return Meta{}, err
	}
	index := findMeta(metas, profileID)
	if index < 0 {
		return Meta{}, ErrNotFound
	}
	metas[index].Name = name
	if err := s.store.writeMetas(metas); err != nil {
		return Meta{}, err
	}
	return cloneMeta(metas[index]), nil
}

func (s *Service) Delete(ctx context.Context, profileID string, input DeleteInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metas, err := s.refreshMetas()
	if err != nil {
		return err
	}
	index := findMeta(metas, profileID)
	if index < 0 {
		return ErrNotFound
	}
	if metas[index].Name != input.ConfirmName {
		return ErrDeleteNameMismatch
	}
	if s.guard != nil {
		if err := s.guard.GuardDelete(ctx, profileID, input.StopCurrent); err != nil {
			return err
		}
	}
	next := append(append([]Meta(nil), metas[:index]...), metas[index+1:]...)
	stagedDraft := s.store.draftPath(profileID) + ".deleting"
	if _, err := os.Lstat(stagedDraft); err == nil {
		return fmt.Errorf("存在未完成的草稿删除: %s", profileID)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(s.store.draftPath(profileID), stagedDraft); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := syncDirectory(s.store.draftsDir); err != nil {
		return err
	}
	if err := s.store.writeMetas(next); err != nil {
		if restoreErr := os.Rename(stagedDraft, s.store.draftPath(profileID)); restoreErr != nil && !os.IsNotExist(restoreErr) {
			return fmt.Errorf("删除档案元数据失败: %w（恢复草稿失败: %v）", err, restoreErr)
		}
		if restoreErr := syncDirectory(s.store.draftsDir); restoreErr != nil {
			return fmt.Errorf("删除档案元数据失败: %w（同步恢复草稿失败: %v）", err, restoreErr)
		}
		return err
	}
	// profiles.json is the deletion commit point. Cleanup is best effort so the
	// API cannot report failure after the profile has already disappeared;
	// startup recovery removes any staged draft left behind.
	if err := s.store.removeFile(stagedDraft); err == nil {
		_ = syncDirectory(s.store.draftsDir)
	}
	return nil
}

func (s *Service) GetDraft(ctx context.Context, profileID string) (Draft, string, error) {
	if err := ctx.Err(); err != nil {
		return Draft{}, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, draft, err := s.loadUsableProfile(profileID)
	if err != nil {
		return Draft{}, "", err
	}
	if meta.DraftRevision != draft.Revision {
		return Draft{}, "", fmt.Errorf("草稿修订与档案元数据不一致")
	}
	return cloneDraft(draft), RevisionETag(draft.Revision), nil
}

func (s *Service) SaveDraft(ctx context.Context, profileID, ifMatch string, input DraftInput) (Draft, string, error) {
	if err := ctx.Err(); err != nil {
		return Draft{}, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, draft, err := s.loadUsableProfile(profileID)
	if err != nil {
		return Draft{}, "", err
	}
	expected, err := ParseRevisionETag(ifMatch)
	if err != nil {
		return Draft{}, "", ErrPreconditionRequired
	}
	if expected != draft.Revision {
		return Draft{}, RevisionETag(draft.Revision), RevisionConflict{LatestRevision: draft.Revision}
	}
	nextConfig, err := reconcileBuffers(draft.Config, input.Buffers)
	if err != nil {
		return Draft{}, "", err
	}
	nextBuffers := cloneBuffers(draft.Buffers)
	for module, buffer := range input.Buffers {
		nextBuffers[module] = buffer
	}
	next := Draft{ProfileID: profileID, Revision: draft.Revision + 1, Config: nextConfig, Buffers: nextBuffers, Bindings: append([]RuleBinding(nil), input.Bindings...), SavedAt: s.clock().UTC()}
	if err := validateDraftEnvelope(next, maxModuleBytes, maxDraftBytes); err != nil {
		return Draft{}, "", err
	}
	if err := s.store.writeDraft(next); err != nil {
		return Draft{}, "", err
	}
	metas, err := s.store.loadMetas()
	if err != nil {
		return Draft{}, "", err
	}
	index := findMeta(metas, profileID)
	if index < 0 {
		return Draft{}, "", ErrNotFound
	}
	metas[index].DraftRevision = next.Revision
	metas[index].NeedsApply = metas[index].AppliedRevision == nil || *metas[index].AppliedRevision != next.Revision
	if err := s.store.writeMetas(metas); err != nil {
		return Draft{}, "", err
	}
	return cloneDraft(next), RevisionETag(next.Revision), nil
}

func (s *Service) RecordValidation(ctx context.Context, profileID string, revision uint64, result ValidationSummary) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metas, err := s.refreshMetas()
	if err != nil {
		return err
	}
	index := findMeta(metas, profileID)
	if index < 0 {
		return ErrNotFound
	}
	if metas[index].DraftRevision != revision || metas[index].QuarantinedError != "" {
		return nil
	}
	result.Revision = revision
	result.CheckedAt = result.CheckedAt.UTC()
	metas[index].LastValidation = &result
	return s.store.writeMetas(metas)
}

func (s *Service) MarkResourcesChanged(ctx context.Context, resourceIDs []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wanted := make(map[string]struct{}, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		wanted[resourceID] = struct{}{}
	}
	if len(wanted) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metas, err := s.refreshMetas()
	if err != nil {
		return err
	}
	changed := false
	for index := range metas {
		if metas[index].QuarantinedError != "" {
			continue
		}
		draft, err := s.store.readDraft(metas[index].ID)
		if err != nil {
			continue
		}
		for _, binding := range draft.Bindings {
			if binding.Enabled {
				_, ok := wanted[binding.ResourceID]
				if !ok {
					continue
				}
				if !metas[index].NeedsApply {
					metas[index].NeedsApply = true
					changed = true
				}
				break
			}
		}
	}
	if !changed {
		return nil
	}
	return s.store.writeMetas(metas)
}

// MarkResourceNeedsApply is the narrow cross-service hook used when a rule
// resource changes format and its previously downloaded local file is no
// longer usable.
func (s *Service) MarkResourceNeedsApply(ctx context.Context, resourceID string) error {
	return s.MarkResourcesChanged(ctx, []string{resourceID})
}

func (s *Service) MarkApplied(ctx context.Context, result AppliedResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metas, err := s.refreshMetas()
	if err != nil {
		return err
	}
	index := findMeta(metas, result.ProfileID)
	if index < 0 {
		return ErrNotFound
	}
	if metas[index].QuarantinedError != "" {
		return ErrQuarantined
	}
	if metas[index].DraftRevision != result.Revision {
		return RevisionConflict{LatestRevision: metas[index].DraftRevision}
	}
	revision := result.Revision
	appliedAt := result.AppliedAt.UTC()
	validation := result.Validation
	validation.Revision = result.Revision
	validation.CheckedAt = validation.CheckedAt.UTC()
	metas[index].AppliedRevision = &revision
	metas[index].AppliedDigest = result.Digest
	metas[index].AppliedResourceDigests = cloneStringMap(result.ResourceDigests)
	metas[index].AppliedAt = &appliedAt
	// If resource reconciliation has seen a newer/missing file, an apply based
	// on an older snapshot must not accidentally clear its dirty marker.
	if !s.resourceKnown || s.enabledBindingsMatchCurrent(result.ProfileID, result.ResourceDigests) {
		metas[index].NeedsApply = false
	} else {
		metas[index].NeedsApply = true
	}
	metas[index].LastValidation = &validation
	return s.store.writeMetas(metas)
}

// ReconcileResourceDigests records the currently usable rule files and marks
// every profile whose enabled bindings no longer match its applied snapshot.
// It is invoked at startup and after a durable rule status write.
func (s *Service) ReconcileResourceDigests(ctx context.Context, current map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceDigests = cloneStringMap(current)
	s.resourceKnown = true
	metas, err := s.refreshMetas()
	if err != nil {
		return err
	}
	changed := false
	for index := range metas {
		if metas[index].QuarantinedError != "" {
			continue
		}
		draft, err := s.store.readDraft(metas[index].ID)
		if err != nil {
			continue
		}
		if !bindingsMatchDigests(draft.Bindings, metas[index].AppliedResourceDigests, current) && !metas[index].NeedsApply {
			metas[index].NeedsApply = true
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.store.writeMetas(metas)
}

func (s *Service) enabledBindingsMatchCurrent(profileID string, digests map[string]string) bool {
	draft, err := s.store.readDraft(profileID)
	if err != nil {
		return false
	}
	return bindingsMatchDigests(draft.Bindings, digests, s.resourceDigests)
}

func bindingsMatchDigests(bindings []RuleBinding, expected, current map[string]string) bool {
	for _, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		expectedDigest, expectedOK := expected[binding.ResourceID]
		currentDigest, currentOK := current[binding.ResourceID]
		if !expectedOK || !currentOK || expectedDigest != currentDigest {
			return false
		}
	}
	return true
}

func (s *Service) ReferencesForResource(ctx context.Context, resourceID string) ([]Reference, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	metas, err := s.refreshMetas()
	if err != nil {
		return nil, err
	}
	references := []Reference{}
	for _, meta := range metas {
		draft, err := s.store.readDraft(meta.ID)
		if err != nil {
			// A damaged draft must not turn into permission to delete a resource
			// whose binding can no longer be inspected.
			return nil, fmt.Errorf("读取档案规则绑定失败 %s: %w", meta.ID, err)
		}
		for _, binding := range draft.Bindings {
			if binding.ResourceID == resourceID {
				references = append(references, Reference{ID: meta.ID, Name: meta.Name})
				break
			}
		}
	}
	return references, nil
}

func (s *Service) loadUsableProfile(profileID string) (Meta, Draft, error) {
	metas, err := s.refreshMetas()
	if err != nil {
		return Meta{}, Draft{}, err
	}
	index := findMeta(metas, profileID)
	if index < 0 {
		return Meta{}, Draft{}, ErrNotFound
	}
	if metas[index].QuarantinedError != "" {
		return Meta{}, Draft{}, fmt.Errorf("%w: %s", ErrQuarantined, metas[index].QuarantinedError)
	}
	draft, err := s.store.readDraft(profileID)
	if err != nil {
		return Meta{}, Draft{}, err
	}
	return metas[index], draft, nil
}

func (s *Service) refreshMetas() ([]Meta, error) {
	metas, err := s.store.loadMetas()
	if err != nil {
		return nil, err
	}
	changed := false
	for index := range metas {
		draft, draftErr := s.store.readDraft(metas[index].ID)
		if draftErr != nil {
			message := draftErr.Error()
			if metas[index].QuarantinedError != message {
				metas[index].QuarantinedError = message
				changed = true
			}
			continue
		}
		if metas[index].QuarantinedError != "" {
			metas[index].QuarantinedError = ""
			changed = true
		}
		if draft.Revision > metas[index].DraftRevision {
			metas[index].DraftRevision = draft.Revision
			metas[index].NeedsApply = metas[index].AppliedRevision == nil || *metas[index].AppliedRevision != draft.Revision
			changed = true
		} else if draft.Revision < metas[index].DraftRevision {
			message := "草稿修订落后于档案元数据"
			if metas[index].QuarantinedError != message {
				metas[index].QuarantinedError = message
				changed = true
			}
		}
	}
	if changed {
		if err := s.store.writeMetas(metas); err != nil {
			return nil, err
		}
	}
	return metas, nil
}

func validateName(name string) (string, error) {
	normalized := strings.TrimSpace(name)
	length := utf8.RuneCountInString(normalized)
	if length < 1 || length > 80 {
		return "", fmt.Errorf("档案名称必须为 1 至 80 个字符")
	}
	return normalized, nil
}

func randomHex(source io.Reader, size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := io.ReadFull(source, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func reconcileBuffers(current map[string]json.RawMessage, buffers map[string]string) (map[string]json.RawMessage, error) {
	next := cloneConfig(current)
	for module, buffer := range buffers {
		if len(buffer) > maxModuleBytes {
			return nil, fmt.Errorf("草稿模块过大: %s", module)
		}
		if buffer == "" {
			delete(next, module)
			continue
		}
		if json.Valid([]byte(buffer)) {
			next[module] = append(json.RawMessage(nil), []byte(buffer)...)
		}
	}
	return next, nil
}

func validateDraftEnvelope(draft Draft, moduleLimit, totalLimit int) error {
	if draft.ProfileID == "" || draft.Revision == 0 || draft.Config == nil || draft.Buffers == nil {
		return fmt.Errorf("草稿包不完整")
	}
	for module, value := range draft.Config {
		if len(value) > moduleLimit {
			return fmt.Errorf("草稿模块过大: %s", module)
		}
		if !json.Valid(value) {
			return fmt.Errorf("草稿模块 JSON 无效: %s", module)
		}
	}
	for module, value := range draft.Buffers {
		if len(value) > moduleLimit {
			return fmt.Errorf("草稿模块过大: %s", module)
		}
	}
	encoded, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	if len(encoded) > totalLimit {
		return fmt.Errorf("草稿包过大")
	}
	return nil
}

func findMeta(metas []Meta, profileID string) int {
	for index := range metas {
		if metas[index].ID == profileID {
			return index
		}
	}
	return -1
}

func cloneConfig(source map[string]json.RawMessage) map[string]json.RawMessage {
	target := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		target[key] = append(json.RawMessage(nil), value...)
	}
	return target
}

func cloneBuffers(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneStringMap(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
