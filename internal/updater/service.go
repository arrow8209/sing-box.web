package updater

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"singbox-web/internal/application"
	"singbox-web/internal/configgen"
	"singbox-web/internal/core"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

var ErrUpdateInProgress = errors.New("core_update_in_progress")

type ReleaseLookup interface {
	LatestStable(context.Context) (Release, error)
}
type BinaryPreparer interface {
	Prepare(context.Context, Release) (PreparedBinary, func(), error)
}
type ProgressBinaryPreparer interface {
	PrepareWithProgress(context.Context, Release, func(DownloadProgress)) (PreparedBinary, func(), error)
}

// UpdateConfirmation is deliberately just a version: an administrator must
// first see a release, then explicitly confirm that exact version.
type UpdateConfirmation struct {
	ConfirmedVersion string `json:"confirmedVersion"`
}

type Status struct {
	OperationID     string     `json:"operationId"`
	TargetVersion   string     `json:"targetVersion,omitempty"`
	Stage           string     `json:"stage"`
	StartedAt       time.Time  `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	Message         string     `json:"message,omitempty"`
	DownloadedBytes int64      `json:"downloadedBytes,omitempty"`
	TotalBytes      int64      `json:"totalBytes,omitempty"`
	BytesPerSecond  int64      `json:"bytesPerSecond,omitempty"`
}

type Service struct {
	layout     paths.Layout
	releases   ReleaseLookup
	fetcher    BinaryPreparer
	core       *core.Manager
	store      *core.Store
	checker    *configgen.Checker
	operations *core.OperationLock
	random     io.Reader

	mu     sync.RWMutex
	status Status
	active bool
}

func NewService(layout paths.Layout, releases ReleaseLookup, fetcher BinaryPreparer, manager *core.Manager, store *core.Store, checker *configgen.Checker, operations *core.OperationLock) *Service {
	if releases == nil {
		releases = NewReleaseClient()
	}
	if fetcher == nil {
		fetcher = NewFetcher(layout)
	}
	if checker == nil {
		checker = configgen.NewChecker(layout.WorkDir, nil, 10*time.Second, 64<<10)
	}
	if operations == nil {
		operations = core.NewOperationLock()
	}
	return &Service{layout: layout, releases: releases, fetcher: fetcher, core: manager, store: store, checker: checker, operations: operations, random: rand.Reader}
}

// Start never changes the installed program synchronously. It returns an ID
// for polling and executes the transaction with its own bounded context so a
// disconnected HTTP client cannot interrupt a stop/restart sequence.
func (s *Service) Start(_ context.Context, confirmation UpdateConfirmation) (string, error) {
	if s == nil || s.core == nil || s.store == nil {
		return "", errors.New("核心更新服务不可用")
	}
	if strings.TrimSpace(confirmation.ConfirmedVersion) == "" {
		return "", fmt.Errorf("必须确认目标版本")
	}
	id, err := s.operationID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return "", ErrUpdateInProgress
	}
	s.active = true
	s.status = Status{OperationID: id, TargetVersion: confirmation.ConfirmedVersion, Stage: "checking_release", StartedAt: now}
	s.mu.Unlock()
	go s.run(id, confirmation)
	return id, nil
}

func (s *Service) Status() Status {
	if s == nil {
		return Status{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value := s.status
	if value.FinishedAt != nil {
		copied := *value.FinishedAt
		value.FinishedAt = &copied
	}
	return value
}

// Latest returns only the display-safe release facts; download URLs and
// checksums remain internal to the updater transaction.
func (s *Service) Latest(ctx context.Context) (Release, error) {
	if s == nil || s.releases == nil {
		return Release{}, errors.New("核心更新服务不可用")
	}
	return s.releases.LatestStable(ctx)
}

func (s *Service) run(id string, confirmation UpdateConfirmation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err := s.update(ctx, confirmation)
	now := time.Now().UTC()
	s.mu.Lock()
	if s.status.OperationID == id {
		s.active = false
		s.status.BytesPerSecond = 0
		s.status.FinishedAt = &now
		if err != nil {
			s.status.Stage, s.status.Message = "failed", safeMessage(err)
		} else {
			s.status.Stage, s.status.Message = "completed", "核心已安装"
		}
	}
	s.mu.Unlock()
}

func (s *Service) update(ctx context.Context, confirmation UpdateConfirmation) error {
	s.setStage("checking_release")
	release, err := s.releases.LatestStable(ctx)
	if err != nil {
		return err
	}
	if release.Version != confirmation.ConfirmedVersion {
		return fmt.Errorf("最新版本已变更，需重新确认")
	}
	s.setStage("downloading")
	var prepared PreparedBinary
	var cleanup func()
	if fetcher, ok := s.fetcher.(ProgressBinaryPreparer); ok {
		prepared, cleanup, err = fetcher.PrepareWithProgress(ctx, release, s.setDownloadProgress)
	} else {
		prepared, cleanup, err = s.fetcher.Prepare(ctx, release)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	s.setStage("verifying_checksum") // Fetcher has verified archive and binary digest.
	if prepared.Version != release.Version {
		return fmt.Errorf("准备的核心版本不匹配")
	}
	s.setStage("verifying_binary")
	if err := verifyVersion(ctx, prepared.Path, prepared.Version); err != nil {
		return err
	}
	configDigest, err := application.DigestDirectory(s.layout.ConfigDir)
	if err != nil {
		return err
	}
	if configDigest != "" {
		s.setStage("checking_config")
		if _, err := s.checker.Check(ctx, prepared.Path, s.layout.ConfigDir); err != nil {
			return fmt.Errorf("新核心配置预检失败: %w", err)
		}
	}
	return s.installPrepared(ctx, prepared, configDigest)
}

func (s *Service) installPrepared(ctx context.Context, prepared PreparedBinary, checkedDigest string) (err error) {
	s.setStage("waiting_for_lock")
	op, release, err := s.operations.Enter(ctx, core.OperationCoreUpdate)
	if err != nil {
		return err
	}
	defer release()
	current, err := application.DigestDirectory(s.layout.ConfigDir)
	if err != nil {
		return err
	}
	if current != checkedDigest && current != "" {
		op.SetStage("checking_config")
		s.setStage("checking_config")
		if _, err := s.checker.Check(ctx, prepared.Path, s.layout.ConfigDir); err != nil {
			return fmt.Errorf("配置在更新期间变更且新核心预检失败: %w", err)
		}
	}
	previous := s.core.Status(ctx)
	op.SetStage("stopping_core")
	s.setStage("stopping_core")
	if previous.Running() {
		if err := s.core.Stop(ctx); err != nil {
			return err
		}
	}
	op.SetStage("installing")
	s.setStage("installing")
	replacement, err := beginBinaryReplacement(s.layout, prepared.Path)
	if err != nil {
		return s.restartPrevious(ctx, previous, err)
	}
	op.SetStage("restarting_core")
	s.setStage("restarting_core")
	if previous.ExpectedRunning {
		if previous.AppliedRevision == nil {
			return errors.Join(errors.New("期望运行状态缺少应用修订"), replacement.Restore())
		}
		if err := s.core.Start(ctx, core.StartRequest{ProfileID: previous.CurrentProfileID, DraftRevision: *previous.AppliedRevision}); err != nil {
			return errors.Join(err, replacement.Restore(), s.restartPrevious(ctx, previous, nil))
		}
	}
	if err := replacement.CommitSingleBackup(); err != nil {
		return err
	}
	return s.store.RecordInstalled(prepared.Version, prepared.BinaryDigest)
}

func (s *Service) restartPrevious(ctx context.Context, previous core.Status, cause error) error {
	if !previous.Running() && !previous.ExpectedRunning {
		return cause
	}
	if previous.AppliedRevision == nil {
		return errors.Join(cause, errors.New("旧核心缺少应用修订，无法恢复"))
	}
	err := s.core.Start(ctx, core.StartRequest{ProfileID: previous.CurrentProfileID, DraftRevision: *previous.AppliedRevision})
	return errors.Join(cause, err)
}

func (s *Service) setStage(stage string) {
	s.mu.Lock()
	if s.active {
		s.status.Stage = stage
		if stage != "downloading" {
			s.status.BytesPerSecond = 0
		}
	}
	s.mu.Unlock()
}

func (s *Service) setDownloadProgress(progress DownloadProgress) {
	if progress.DownloadedBytes < 0 {
		progress.DownloadedBytes = 0
	}
	if progress.TotalBytes < 0 {
		progress.TotalBytes = 0
	}
	if progress.BytesPerSecond < 0 {
		progress.BytesPerSecond = 0
	}
	s.mu.Lock()
	if s.active && s.status.Stage == "downloading" {
		s.status.DownloadedBytes = progress.DownloadedBytes
		s.status.TotalBytes = progress.TotalBytes
		s.status.BytesPerSecond = progress.BytesPerSecond
	}
	s.mu.Unlock()
}

func (s *Service) operationID() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(s.random, raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func verifyVersion(ctx context.Context, path, expected string) error {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(checkCtx, path, "version")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	output, err := command.Output()
	if len(output) > 64<<10 {
		return fmt.Errorf("核心版本输出超过限制")
	}
	if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("核心版本检查超时")
	}
	if err != nil {
		return fmt.Errorf("核心版本检查失败: %w", err)
	}
	if !strings.Contains(string(output), "sing-box version "+expected) {
		return fmt.Errorf("核心版本与确认版本不符")
	}
	return nil
}

type binaryReplacement struct {
	layout paths.Layout
	hadOld bool
	active bool
}

func beginBinaryReplacement(layout paths.Layout, prepared string) (*binaryReplacement, error) {
	if err := os.MkdirAll(layout.BinDir, 0o700); err != nil {
		return nil, err
	}
	if err := storage.RejectSymlinkPath(layout.BinDir); err != nil {
		return nil, err
	}
	info, err := os.Lstat(prepared)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("准备的核心程序不可执行")
	}
	_ = os.Remove(layout.CoreBackup)
	r := &binaryReplacement{layout: layout}
	if _, err := os.Lstat(layout.CoreBinary); err == nil {
		if err := os.Rename(layout.CoreBinary, layout.CoreBackup); err != nil {
			return nil, err
		}
		r.hadOld = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.Rename(prepared, layout.CoreBinary); err != nil {
		if r.hadOld {
			_ = os.Rename(layout.CoreBackup, layout.CoreBinary)
		}
		return nil, err
	}
	if err := syncDirectory(layout.BinDir); err != nil {
		return nil, err
	}
	r.active = true
	return r, nil
}
func (r *binaryReplacement) Restore() error {
	if r == nil || !r.active || !r.hadOld {
		return nil
	}
	if err := os.Remove(r.layout.CoreBinary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	source, err := os.Open(r.layout.CoreBackup)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(r.layout.CoreBinary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(target, source)
	syncErr := target.Sync()
	closeErr := target.Close()
	r.active = false
	return errors.Join(copyErr, syncErr, closeErr, syncDirectory(r.layout.BinDir))
}
func (r *binaryReplacement) CommitSingleBackup() error {
	if r == nil {
		return nil
	}
	r.active = false
	return syncDirectory(r.layout.BinDir)
}
func syncDirectory(directory string) error {
	file, err := os.Open(filepath.Clean(directory))
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
func safeMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToValidUTF8(err.Error(), "")
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}
