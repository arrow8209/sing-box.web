package rules

import (
	"context"
	"crypto/rand"
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

var ErrNotFound = errors.New("规则资源不存在")

// Service owns resource metadata and the fixed local-file lifecycle.
type Service struct {
	store      *Store
	layout     paths.Layout
	profiles   ProfileReferences
	clock      func() time.Time
	random     io.Reader
	locks      *resourceLocks
	downloader Downloader
	audit      *audit.Logger
	mu         sync.Mutex
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
			return s.updateLocked(ctx, resources, index, resource, started, &stage, &digest)
		}
	}
	s.mu.Unlock()
	return result, err
}

func (s *Service) updateLocked(ctx context.Context, resources []Resource, index int, resource Resource, now time.Time, stage, digest *string) (UpdateResult, error) {
	if resource.Disabled {
		return UpdateResult{}, errors.New("已禁用的规则资源不能更新")
	}
	*stage = "download"
	downloaded, err := s.downloader.Fetch(ctx, resource)
	if err != nil {
		return UpdateResult{}, s.recordUpdateFailure(resources, index, now, err)
	}
	defer os.Remove(downloaded.Path)
	*digest = downloaded.Digest
	*stage = "validate"
	if err := ValidateDownloaded(resource.Format, downloaded.Payload); err != nil {
		return UpdateResult{}, s.recordUpdateFailure(resources, index, now, err)
	}
	*stage = "install"
	replacement, err := storage.BeginFileReplacement(downloaded.Path, resource.LocalPath, 0o600)
	if err != nil {
		return UpdateResult{}, s.recordUpdateFailure(resources, index, now, err)
	}
	changed := resource.LastDigest != downloaded.Digest
	next := resource
	next.LastAttemptAt, next.LastSuccessAt = ptrTime(now), ptrTime(now)
	next.LastDigest, next.LastError = downloaded.Digest, ""
	resources[index] = next
	*stage = "save_status"
	s.mu.Lock()
	err = s.store.write(resources)
	s.mu.Unlock()
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
	*stage = "completed"
	return UpdateResult{Changed: changed, Digest: downloaded.Digest, Message: updateMessage(changed)}, nil
}

func (s *Service) recordUpdateFailure(resources []Resource, index int, now time.Time, cause error) error {
	resources[index].LastAttemptAt = ptrTime(now)
	resources[index].LastError = safeUpdateError(cause)
	s.mu.Lock()
	err := s.store.write(resources)
	s.mu.Unlock()
	if err != nil {
		return errors.Join(cause, err)
	}
	return cause
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
	if err := validateInput(input.Name, input.URL, input.Format, input.UpdateInterval); err != nil {
		return ResourceResponse{}, err
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
	resource := Resource{ID: id, Name: input.Name, URL: input.URL, Format: input.Format, UpdateInterval: input.UpdateInterval, Disabled: input.Disabled, LocalPath: absolute}
	resources = append(resources, resource)
	if err := s.store.write(resources); err != nil {
		return ResourceResponse{}, err
	}
	return s.response(resource), nil
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
	defer s.mu.Unlock()
	resources, err := s.store.list()
	if err != nil {
		return ResourceResponse{}, err
	}
	index := indexOf(resources, id)
	if index < 0 {
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
	if err := validateInput(next.Name, next.URL, next.Format, next.UpdateInterval); err != nil {
		return ResourceResponse{}, err
	}
	if next.Format != current.Format {
		newPath, _, err := s.layout.RuleSetPath(id, extension(next.Format))
		if err != nil {
			return ResourceResponse{}, err
		}
		staged, err := moveAside(current.LocalPath)
		if err != nil {
			return ResourceResponse{}, err
		}
		next.LocalPath, next.LastSuccessAt, next.LastDigest, next.LastError, next.LastAttemptAt, next.NextUpdateAt = newPath, nil, "", "", nil, nil
		resources[index] = next
		if err := s.store.write(resources); err != nil {
			_ = staged.restore()
			return ResourceResponse{}, err
		}
		if err := staged.delete(); err != nil {
			return ResourceResponse{}, err
		}
		if err := s.markReferencedNeedsApply(ctx, id); err != nil {
			return ResourceResponse{}, err
		}
		return s.response(next), nil
	}
	resources[index] = next
	if err := s.store.write(resources); err != nil {
		return ResourceResponse{}, err
	}
	return s.response(next), nil
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
	remaining := append(append([]Resource(nil), resources[:index]...), resources[index+1:]...)
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
	return ResourceResponse{ID: resource.ID, Name: resource.Name, URL: resource.URL, Format: resource.Format, UpdateInterval: resource.UpdateInterval, Disabled: resource.Disabled, LocalPath: relative, LastAttemptAt: resource.LastAttemptAt, LastSuccessAt: resource.LastSuccessAt, LastDigest: resource.LastDigest, LastError: resource.LastError, NextUpdateAt: resource.NextUpdateAt}
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

func validateInput(name, rawURL string, format Format, interval string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 {
		return fmt.Errorf("规则资源名称必须为 1–80 个字符")
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
