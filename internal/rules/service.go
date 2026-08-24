package rules

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"singbox-web/internal/audit"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

var (
	ErrNotFound     = errors.New("规则资源不存在")
	ErrManualUpdate = errors.New("手工规则不需要网络更新")
)

// Service owns resource metadata and the fixed local-file lifecycle.
type Service struct {
	store       *Store
	layout      paths.Layout
	profiles    ProfileReferences
	clock       func() time.Time
	random      io.Reader
	locks       ResourceLocker
	downloader  Downloader
	audit       *audit.Logger
	mu          sync.Mutex
	reconcileMu sync.Mutex
}

// SetResourceLocker is intended for deterministic lock-order tests.
func (s *Service) SetResourceLocker(locker ResourceLocker) {
	if locker != nil {
		s.locks = locker
	}
}

func NewService(store *Store, layout paths.Layout, profiles ProfileReferences, clock func() time.Time, randomSource io.Reader) (*Service, error) {
	if store == nil || profiles == nil {
		return nil, fmt.Errorf("规则资源服务依赖不能为空")
	}
	if clock == nil {
		clock = time.Now
	}
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &Service{store: store, layout: layout, profiles: profiles, clock: clock, random: randomSource, locks: newResourceLocks(), downloader: NewDownloader()}, nil
}

func (s *Service) SetDownloader(downloader Downloader) {
	if downloader != nil {
		s.downloader = downloader
	}
}
func (s *Service) SetAuditLogger(logger *audit.Logger) { s.audit = logger }

// UpdateNow fetches, validates and atomically installs one resource.  It only
// makes profile drafts need applying when the content digest actually changed.
func (s *Service) UpdateNow(ctx context.Context, id string) (result UpdateResult, err error) {
	started := s.clock().UTC()
	stage, digest := "lookup", ""
	defer func() {
		if s.audit == nil {
			return
		}
		event := audit.Event{Operation: "update_rule_resource", InternalID: id, Digest: digest, Stage: stage, DurationMS: s.clock().UTC().Sub(started).Milliseconds(), Result: "success"}
		if err != nil {
			event.Digest, event.Result = "", "failed"
		}
		s.audit.Record(event)
	}()
	if !resourceIDPattern.MatchString(id) {
		return result, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	release, err := s.locks.Lock(ctx, id)
	if err != nil {
		return result, err
	}
	defer release()
	s.mu.Lock()
	resources, err := s.store.list()
	if err == nil {
		index := indexOf(resources, id)
		if index < 0 {
			err = ErrNotFound
		} else {
			resource := resources[index]
			s.mu.Unlock()
			return s.updateLocked(ctx, resource, started, &stage, &digest)
		}
	}
	s.mu.Unlock()
	return result, err
}

func (s *Service) updateLocked(ctx context.Context, resource Resource, now time.Time, stage, digest *string) (UpdateResult, error) {
	if resource.Disabled {
		return UpdateResult{}, errors.New("已禁用的规则资源不能更新")
	}
	if effectiveSourceType(resource.SourceType) == SourceManual {
		return UpdateResult{}, ErrManualUpdate
	}
	*stage = "download"
	downloaded, err := s.downloader.Fetch(ctx, resource)
	if err != nil {
		return UpdateResult{}, s.recordUpdateFailure(resource.ID, now, err)
	}
	defer os.Remove(downloaded.Path)
	*digest = downloaded.Digest
	*stage = "validate"
	if err := ValidateDownloaded(resource.Format, downloaded.Payload); err != nil {
		return UpdateResult{}, s.recordUpdateFailure(resource.ID, now, err)
	}
	*stage = "install"
	replacement, err := storage.BeginFileReplacement(downloaded.Path, resource.LocalPath, 0o600)
	if err != nil {
		return UpdateResult{}, s.recordUpdateFailure(resource.ID, now, err)
	}
	changed := resource.LastDigest != downloaded.Digest
	next := resource
	next.LastAttemptAt, next.LastSuccessAt = ptrTime(now), ptrTime(now)
	next.LastDigest, next.LastError = downloaded.Digest, ""
	if resource.Format == FormatSource {
		count, countErr := SourceRuleCount(downloaded.Payload)
		if countErr != nil {
			_ = replacement.Restore()
			return UpdateResult{}, s.recordUpdateFailure(resource.ID, now, countErr)
		}
		next.RuleCount = &count
	} else if resource.CatalogKey == "" {
		next.RuleCount = nil
	}
	*stage = "save_status"
	err = s.mutateResource(resource.ID, func(current *Resource) {
		current.LastAttemptAt = next.LastAttemptAt
		current.LastSuccessAt = next.LastSuccessAt
		current.LastDigest = next.LastDigest
		current.LastError = next.LastError
		current.RuleCount = next.RuleCount
	})
	if err != nil {
		if restoreErr := replacement.Restore(); restoreErr != nil {
			return UpdateResult{}, errors.Join(err, restoreErr)
		}
		return UpdateResult{}, err
	}
	if err := replacement.Commit(); err != nil {
		return UpdateResult{}, err
	}
	if changed {
		*stage = "mark_needs_apply"
		if marker, ok := s.profiles.(interface {
			MarkResourcesChanged(context.Context, []string) error
		}); ok {
			if err := marker.MarkResourcesChanged(ctx, []string{resource.ID}); err != nil {
				return UpdateResult{}, err
			}
		}
	}
	if err := s.reconcileCurrentResourceDigests(ctx); err != nil {
		return UpdateResult{}, err
	}
	*stage = "completed"
	return UpdateResult{Changed: changed, Digest: downloaded.Digest, Message: updateMessage(changed)}, nil
}

func (s *Service) recordUpdateFailure(id string, now time.Time, cause error) error {
	err := s.mutateResource(id, func(resource *Resource) {
		resource.LastAttemptAt = ptrTime(now)
		resource.LastError = safeUpdateError(cause)
	})
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// mutateResource reloads the managed document immediately before committing
// one resource's fields. Downloads for different resources run concurrently,
// so writing the snapshot captured before a download would lose newer states.
func (s *Service) mutateResource(id string, mutate func(*Resource)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return err
	}
	index := indexOf(resources, id)
	if index < 0 {
		return ErrNotFound
	}
	mutate(&resources[index])
	if err := s.store.write(resources); err != nil {
		return err
	}
	return nil
}
func ptrTime(value time.Time) *time.Time { value = value.UTC(); return &value }
func safeUpdateError(err error) string {
	switch {
	case errors.Is(err, ErrUnsupportedURL):
		return "规则地址不受支持"
	case errors.Is(err, ErrTooManyRedirects):
		return "规则下载重定向次数超过限制"
	case errors.Is(err, ErrResponseTooLarge):
		return "规则响应超过 64 MiB"
	case errors.Is(err, ErrEmptyRuleSet):
		return "规则文件不能为空"
	default:
		return "规则下载或校验失败"
	}
}
func updateMessage(changed bool) string {
	if changed {
		return "规则资源已更新，请点击“应用”使 sing-box 使用新规则。"
	}
	return "规则资源内容未变化。"
}

func (s *Service) List(ctx context.Context) ([]ResourceResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return nil, err
	}
	responses := make([]ResourceResponse, len(resources))
	for i, resource := range resources {
		responses[i] = s.response(resource)
	}
	return responses, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (ResourceResponse, error) {
	return s.create(ctx, input, "", nil)
}

func (s *Service) create(ctx context.Context, input CreateInput, catalogKey string, catalogRuleCount *int) (ResourceResponse, error) {
	sourceType := effectiveSourceType(input.SourceType)
	var manualPayload []byte
	var manualCount int
	switch sourceType {
	case SourceRemote:
		if len(input.Content) != 0 {
			return ResourceResponse{}, errors.New("网络规则不能提交手工内容")
		}
		if input.Format == "" {
			input.Format = FormatBinary
		}
		if input.Format != FormatBinary {
			return ResourceResponse{}, errors.New("网络规则只能使用 binary 格式")
		}
		if err := validateRemoteInput(input.Name, input.URL, input.Format, input.UpdateInterval); err != nil {
			return ResourceResponse{}, err
		}
	case SourceManual:
		if err := validateName(input.Name); err != nil {
			return ResourceResponse{}, err
		}
		if input.URL != "" || input.UpdateInterval != "" || (input.Format != "" && input.Format != FormatSource) {
			return ResourceResponse{}, errors.New("手工规则只能使用 source 格式，且不能包含 URL 或更新周期")
		}
		var err error
		manualPayload, manualCount, err = NormalizeManualSource(input.Content)
		if err != nil {
			return ResourceResponse{}, err
		}
		input.Format = FormatSource
		catalogKey = ""
	default:
		return ResourceResponse{}, errors.New("规则资源来源类型无效")
	}
	if err := ctx.Err(); err != nil {
		return ResourceResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return ResourceResponse{}, err
	}
	id, err := randomID(s.random)
	if err != nil {
		return ResourceResponse{}, err
	}
	absolute, _, err := s.layout.RuleSetPath(id, extension(input.Format))
	if err != nil {
		return ResourceResponse{}, err
	}
	resource := Resource{ID: id, CatalogKey: catalogKey, SourceType: sourceType, Name: input.Name, URL: input.URL, Format: input.Format, UpdateInterval: input.UpdateInterval, Disabled: input.Disabled, LocalPath: absolute, RuleCount: catalogRuleCount}
	if sourceType == SourceManual {
		now := s.clock().UTC()
		digest := fmt.Sprintf("%x", sha256.Sum256(manualPayload))
		resource.LastAttemptAt, resource.LastSuccessAt = ptrTime(now), ptrTime(now)
		resource.LastDigest, resource.RuleCount = digest, &manualCount
		temporary, err := stageRulePayload(absolute, manualPayload)
		if err != nil {
			return ResourceResponse{}, err
		}
		defer os.Remove(temporary)
		replacement, err := storage.BeginFileReplacement(temporary, absolute, 0o600)
		if err != nil {
			return ResourceResponse{}, err
		}
		resources = append(resources, resource)
		if err := s.store.write(resources); err != nil {
			_ = replacement.Restore()
			return ResourceResponse{}, err
		}
		if err := replacement.Commit(); err != nil {
			return ResourceResponse{}, err
		}
		return s.response(resource), nil
	}
	resources = append(resources, resource)
	if err := s.store.write(resources); err != nil {
		return ResourceResponse{}, err
	}
	return s.response(resource), nil
}

func (s *Service) Get(ctx context.Context, id string) (ResourceDetail, error) {
	if !resourceIDPattern.MatchString(id) {
		return ResourceDetail{}, ErrNotFound
	}
	release, err := s.locks.Lock(ctx, id)
	if err != nil {
		return ResourceDetail{}, err
	}
	defer release()
	s.mu.Lock()
	resources, err := s.store.list()
	s.mu.Unlock()
	if err != nil {
		return ResourceDetail{}, err
	}
	index := indexOf(resources, id)
	if index < 0 {
		return ResourceDetail{}, ErrNotFound
	}
	resource := resources[index]
	detail := ResourceDetail{ResourceResponse: s.response(resource)}
	if effectiveSourceType(resource.SourceType) != SourceManual {
		return detail, nil
	}
	payload, err := os.ReadFile(resource.LocalPath)
	if err != nil {
		return ResourceDetail{}, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	if digest != resource.LastDigest || !json.Valid(payload) {
		return ResourceDetail{}, ErrResourceFileDrift
	}
	detail.Content = json.RawMessage(payload)
	return detail, nil
}

func (s *Service) Patch(ctx context.Context, id string, input PatchInput) (ResourceResponse, error) {
	if !resourceIDPattern.MatchString(id) {
		return ResourceResponse{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return ResourceResponse{}, err
	}
	release, err := s.locks.Lock(ctx, id)
	if err != nil {
		return ResourceResponse{}, err
	}
	defer release()
	s.mu.Lock()
	resources, err := s.store.list()
	if err != nil {
		s.mu.Unlock()
		return ResourceResponse{}, err
	}
	index := indexOf(resources, id)
	if index < 0 {
		s.mu.Unlock()
		return ResourceResponse{}, ErrNotFound
	}
	current := resources[index]
	next := current
	if input.Name != nil {
		next.Name = *input.Name
	}
	if input.URL != nil {
		next.URL = *input.URL
	}
	if input.Format != nil {
		next.Format = *input.Format
	}
	if input.UpdateInterval != nil {
		next.UpdateInterval = *input.UpdateInterval
	}
	if input.Disabled != nil {
		next.Disabled = *input.Disabled
	}
	if input.SourceType != nil {
		next.SourceType = *input.SourceType
	}
	next.SourceType = effectiveSourceType(next.SourceType)
	disabledChanged := next.Disabled != current.Disabled
	if next.SourceType == SourceManual {
		if input.URL != nil && *input.URL != "" || input.UpdateInterval != nil && *input.UpdateInterval != "" || input.Format != nil && *input.Format != FormatSource {
			s.mu.Unlock()
			return ResourceResponse{}, errors.New("手工规则只能使用 source 格式，且不能包含 URL 或更新周期")
		}
		if err := validateName(next.Name); err != nil {
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		if input.Content == nil && effectiveSourceType(current.SourceType) != SourceManual {
			s.mu.Unlock()
			return ResourceResponse{}, errors.New("切换到手工规则时必须提交规则内容")
		}
		next.URL, next.UpdateInterval, next.Format, next.CatalogKey = "", "", FormatSource, ""
		next.NextUpdateAt = nil
		if input.Content == nil {
			resources[index] = next
			if err := s.store.write(resources); err != nil {
				s.mu.Unlock()
				return ResourceResponse{}, err
			}
			s.mu.Unlock()
			if disabledChanged {
				if err := s.markResourceChanged(ctx, id); err != nil {
					return ResourceResponse{}, err
				}
			}
			return s.response(next), nil
		}
		payload, count, err := NormalizeManualSource(*input.Content)
		if err != nil {
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		newPath, _, err := s.layout.RuleSetPath(id, ".json")
		if err != nil {
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		var old movedFile
		if filepath.Clean(current.LocalPath) != filepath.Clean(newPath) {
			old, err = moveAside(current.LocalPath)
			if err != nil {
				s.mu.Unlock()
				return ResourceResponse{}, err
			}
		}
		temporary, err := stageRulePayload(newPath, payload)
		if err != nil {
			_ = old.restore()
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		defer os.Remove(temporary)
		replacement, err := storage.BeginFileReplacement(temporary, newPath, 0o600)
		if err != nil {
			_ = old.restore()
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		now := s.clock().UTC()
		digest := fmt.Sprintf("%x", sha256.Sum256(payload))
		changed := effectiveSourceType(current.SourceType) != SourceManual || current.LastDigest != digest || disabledChanged
		next.LocalPath, next.LastAttemptAt, next.LastSuccessAt = newPath, ptrTime(now), ptrTime(now)
		next.LastDigest, next.LastError, next.RuleCount = digest, "", &count
		resources[index] = next
		if err := s.store.write(resources); err != nil {
			_ = replacement.Restore()
			_ = old.restore()
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		s.mu.Unlock()
		if err := replacement.Commit(); err != nil {
			return ResourceResponse{}, err
		}
		if err := old.delete(); err != nil {
			return ResourceResponse{}, err
		}
		if changed {
			if err := s.markResourceChanged(ctx, id); err != nil {
				return ResourceResponse{}, err
			}
		}
		return s.response(next), nil
	}
	if next.SourceType != SourceRemote {
		s.mu.Unlock()
		return ResourceResponse{}, errors.New("规则资源来源类型无效")
	}
	if input.Content != nil {
		s.mu.Unlock()
		return ResourceResponse{}, errors.New("网络规则不能提交手工内容")
	}
	if next.Format != FormatBinary {
		s.mu.Unlock()
		return ResourceResponse{}, errors.New("网络规则只能使用 binary 格式")
	}
	if err := validateRemoteInput(next.Name, next.URL, next.Format, next.UpdateInterval); err != nil {
		s.mu.Unlock()
		return ResourceResponse{}, err
	}
	definitionChanged := effectiveSourceType(current.SourceType) != SourceRemote || next.URL != current.URL || next.Format != current.Format
	if (input.URL != nil && next.URL != current.URL) || (input.Format != nil && next.Format != current.Format) {
		next.CatalogKey = ""
	}
	if definitionChanged {
		newPath, _, err := s.layout.RuleSetPath(id, extension(next.Format))
		if err != nil {
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		staged, err := moveAside(current.LocalPath)
		if err != nil {
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		next.LocalPath, next.LastSuccessAt, next.LastDigest, next.LastError, next.LastAttemptAt, next.NextUpdateAt, next.RuleCount = newPath, nil, "", "", nil, nil, nil
		resources[index] = next
		if err := s.store.write(resources); err != nil {
			_ = staged.restore()
			s.mu.Unlock()
			return ResourceResponse{}, err
		}
		s.mu.Unlock()
		if err := staged.delete(); err != nil {
			return ResourceResponse{}, err
		}
		if err := s.markResourceChanged(ctx, id); err != nil {
			return ResourceResponse{}, err
		}
		return s.response(next), nil
	}
	if next.Disabled || next.UpdateInterval == "" || input.Disabled != nil || input.UpdateInterval != nil {
		next.NextUpdateAt = nil
	}
	resources[index] = next
	if err := s.store.write(resources); err != nil {
		s.mu.Unlock()
		return ResourceResponse{}, err
	}
	s.mu.Unlock()
	if disabledChanged {
		if err := s.markResourceChanged(ctx, id); err != nil {
			return ResourceResponse{}, err
		}
	}
	return s.response(next), nil
}

func (s *Service) resourceDigests(resources []Resource) map[string]string {
	digests := make(map[string]string, len(resources))
	for _, resource := range resources {
		if resource.LastSuccessAt != nil && resource.LastDigest != "" && resourceFileDigest(resource) == resource.LastDigest {
			digests[resource.ID] = resource.LastDigest
		}
	}
	return digests
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if !resourceIDPattern.MatchString(id) {
		return ErrNotFound
	}
	refs, err := s.profiles.ReferencesForResource(ctx, id)
	if err != nil {
		return err
	}
	if len(refs) != 0 {
		return &ReferencedError{ResourceID: id, Profiles: refs}
	}
	release, err := s.locks.Lock(ctx, id)
	if err != nil {
		return err
	}
	defer release()
	s.mu.Lock()
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return err
	}
	index := indexOf(resources, id)
	if index < 0 {
		return ErrNotFound
	}
	staged, err := moveAside(resources[index].LocalPath)
	if err != nil {
		return err
	}
	remaining := make([]Resource, 0, len(resources)-1)
	remaining = append(remaining, resources[:index]...)
	remaining = append(remaining, resources[index+1:]...)
	if err := s.store.write(remaining); err != nil {
		_ = staged.restore()
		return err
	}
	return staged.delete()
}

func (s *Service) References(ctx context.Context, id string) ([]Reference, error) {
	if !resourceIDPattern.MatchString(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	resources, err := s.store.list()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if indexOf(resources, id) < 0 {
		return nil, ErrNotFound
	}
	return s.profiles.ReferencesForResource(ctx, id)
}

func (s *Service) response(resource Resource) ResourceResponse {
	_, relative, err := s.layout.RuleSetPath(resource.ID, extension(resource.Format))
	if err != nil {
		relative = ""
	}
	var sizeBytes *int64
	if info, statErr := os.Lstat(resource.LocalPath); statErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		size := info.Size()
		sizeBytes = &size
	}
	return ResourceResponse{ID: resource.ID, CatalogKey: resource.CatalogKey, SourceType: effectiveSourceType(resource.SourceType), Name: resource.Name, URL: resource.URL, Format: resource.Format, UpdateInterval: resource.UpdateInterval, Disabled: resource.Disabled, LocalPath: relative, LastAttemptAt: resource.LastAttemptAt, LastSuccessAt: resource.LastSuccessAt, LastDigest: resource.LastDigest, LastError: resource.LastError, NextUpdateAt: resource.NextUpdateAt, SizeBytes: sizeBytes, RuleCount: resource.RuleCount}
}

func (s *Service) markReferencedNeedsApply(ctx context.Context, id string) error {
	marker, ok := s.profiles.(interface {
		MarkResourceNeedsApply(context.Context, string) error
	})
	if !ok {
		return nil
	}
	return marker.MarkResourceNeedsApply(ctx, id)
}

func (s *Service) markResourceChanged(ctx context.Context, id string) error {
	if marker, ok := s.profiles.(interface {
		MarkResourcesChanged(context.Context, []string) error
	}); ok {
		if err := marker.MarkResourcesChanged(ctx, []string{id}); err != nil {
			return err
		}
	} else if err := s.markReferencedNeedsApply(ctx, id); err != nil {
		return err
	}
	return s.reconcileCurrentResourceDigests(ctx)
}

func (s *Service) reconcileCurrentResourceDigests(ctx context.Context) error {
	reconciler, ok := s.profiles.(interface {
		ReconcileResourceDigests(context.Context, map[string]string) error
	})
	if !ok {
		return nil
	}
	// Serialize the fresh read and reconciliation so a slower update cannot
	// publish a digest map captured before a newer update committed.
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	digests, err := s.CurrentDigests(ctx)
	if err != nil {
		return err
	}
	return reconciler.ReconcileResourceDigests(ctx, digests)
}

func validateName(name string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 {
		return fmt.Errorf("规则资源名称必须为 1–80 个字符")
	}
	return nil
}

func validateRemoteInput(name, rawURL string, format Format, interval string) error {
	if err := validateName(name); err != nil {
		return err
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("规则资源 URL 必须是无凭据、无片段的 HTTP 或 HTTPS 地址")
	}
	if format != FormatSource && format != FormatBinary {
		return fmt.Errorf("规则资源格式无效")
	}
	if interval == "" {
		return nil
	}
	duration, err := time.ParseDuration(interval)
	if err != nil || duration < 15*time.Minute || duration > 30*24*time.Hour {
		return fmt.Errorf("更新周期必须介于 15 分钟和 30 天之间")
	}
	return nil
}

func stageRulePayload(target string, payload []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".manual-")
	if err != nil {
		return "", err
	}
	path := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", err
	}
	if _, err := file.Write(payload); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	keep = true
	return path, nil
}

func indexOf(resources []Resource, id string) int {
	for i := range resources {
		if resources[i].ID == id {
			return i
		}
	}
	return -1
}

type movedFile struct{ from, temporary string }

func moveAside(path string) (movedFile, error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return movedFile{}, nil
	} else if err != nil {
		return movedFile{}, err
	}
	directory := filepath.Dir(path)
	temporary := filepath.Join(directory, "."+filepath.Base(path)+".changing")
	if _, err := os.Lstat(temporary); err == nil {
		return movedFile{}, fmt.Errorf("规则文件存在未完成的切换: %s", path)
	} else if !os.IsNotExist(err) {
		return movedFile{}, err
	}
	if err := os.Rename(path, temporary); err != nil {
		return movedFile{}, err
	}
	return movedFile{from: path, temporary: temporary}, nil
}
func (m movedFile) restore() error {
	if m.temporary == "" {
		return nil
	}
	return os.Rename(m.temporary, m.from)
}
func (m movedFile) delete() error {
	if m.temporary == "" {
		return nil
	}
	return os.Remove(m.temporary)
}
