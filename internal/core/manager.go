package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

var (
	ErrAlreadyRunning  = errors.New("core_already_running")
	ErrNotInstalled    = errors.New("core_not_installed")
	ErrNoAppliedConfig = errors.New("core_config_not_applied")
	ErrNotRunning      = errors.New("core_not_running")
)

type StartRequest struct {
	ProfileID     string
	DraftRevision uint64
}

type afterClock interface {
	After(time.Duration) <-chan time.Time
}
type realClock struct{}

func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Manager is the sole owner of the managed core process. Durable application
// bindings are committed or restored by the application coordinator.
type Manager struct {
	layout    paths.Layout
	store     *Store
	log       *BoundedLog
	factory   ProcessFactory
	clock     afterClock
	operation *OperationLock

	mu        sync.Mutex
	process   Process
	done      chan struct{}
	state     ProcessState
	startedAt *time.Time
	lastError string
	current   StartRequest
}

func NewManager(layout paths.Layout, store *Store, log *BoundedLog, operation *OperationLock) *Manager {
	return newManager(layout, store, log, operation, LinuxProcessFactory{}, realClock{})
}

// NewManagerWithFactory is provided for deterministic integration tests and
// controlled embedding; production should use NewManager.
func NewManagerWithFactory(layout paths.Layout, store *Store, log *BoundedLog, operation *OperationLock, factory ProcessFactory) *Manager {
	return newManager(layout, store, log, operation, factory, realClock{})
}

func newManager(layout paths.Layout, store *Store, log *BoundedLog, operation *OperationLock, factory ProcessFactory, clock afterClock) *Manager {
	if factory == nil {
		factory = LinuxProcessFactory{}
	}
	if clock == nil {
		clock = realClock{}
	}
	if operation == nil {
		operation = NewOperationLock()
	}
	state := ProcessStopped
	if ensureExecutable(layout.CoreBinary) != nil {
		state = ProcessNotInstalled
	}
	return &Manager{layout: layout, store: store, log: log, factory: factory, clock: clock, operation: operation, state: state}
}

func (m *Manager) Layout() paths.Layout { return m.layout }

func (m *Manager) Start(ctx context.Context, request StartRequest) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	if m.process != nil {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	if ensureExecutable(m.layout.CoreBinary) != nil {
		m.state = ProcessNotInstalled
		m.mu.Unlock()
		return ErrNotInstalled
	}
	m.state, m.lastError = ProcessStarting, ""
	process, err := m.factory.Start(ProcessSpec{
		Path: filepath.FromSlash("data/bin/sing-box"), Args: []string{"run", "-C", "config"}, Dir: m.layout.WorkDir,
		Env: allowedEnvironment(), Stdout: m.log, Stderr: m.log,
	})
	if err != nil {
		m.recordStartFailureLocked(err)
		m.mu.Unlock()
		m.persistError(err.Error(), false)
		return err
	}
	m.process, m.current, m.done = process, request, make(chan struct{})
	now := time.Now().UTC()
	m.startedAt = &now
	m.mu.Unlock()
	exit := m.observe(process)
	select {
	case result := <-exit:
		return m.finishEarlyExit(process, result)
	case <-m.clock.After(2 * time.Second):
		return m.markRunning(process)
	case <-ctx.Done():
		_ = m.Stop(context.Background())
		return context.Cause(ctx)
	}
}

func (m *Manager) observe(process Process) <-chan ProcessResult {
	result := make(chan ProcessResult, 1)
	go func() {
		outcome, ok := <-process.Exited()
		if !ok {
			outcome = ProcessResult{ExitCode: -1, Err: errors.New("核心退出结果缺失")}
		}
		result <- outcome
		m.finishExit(process, outcome)
	}()
	return result
}

func (m *Manager) finishEarlyExit(process Process, result ProcessResult) error {
	m.finishExit(process, result)
	if result.Err != nil {
		return result.Err
	}
	return fmt.Errorf("核心在启动观察期内退出，退出码 %d", result.ExitCode)
}

func (m *Manager) markRunning(process Process) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != process {
		return errors.New("核心在启动观察期内退出")
	}
	m.state = ProcessRunning
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	process := m.process
	if process == nil {
		m.mu.Unlock()
		return nil
	}
	done := m.done
	previousState := m.state
	m.state = ProcessStopping
	m.mu.Unlock()
	if err := process.SignalGroup(syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		m.mu.Lock()
		if m.process == process && m.state == ProcessStopping {
			m.state = previousState
		}
		m.mu.Unlock()
		return err
	}
	select {
	case <-done:
		// observe owns state cleanup; allow it to acquire the mutex.
		return nil
	case <-m.clock.After(10 * time.Second):
		if err := process.SignalGroup(syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// RequestStop records that the core should remain stopped for the current Web
// process lifetime and terminates only the sing-box process owned by this
// Manager. A later Web startup derives auto-start from the applied config,
// while internal update and shutdown flows call Stop directly.
func (m *Manager) RequestStop(ctx context.Context) error {
	if m == nil {
		return errors.New("核心管理器不可用")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	operation, release, err := m.operation.Enter(ctx, OperationCoreStop)
	if err != nil {
		return err
	}
	defer release()

	operation.SetStage("record_stop_intent")
	if m.store != nil {
		if _, err := m.store.Update(func(state *PersistentState) error {
			state.ExpectedRunning = false
			state.LastStartError = ""
			return nil
		}); err != nil {
			operation.SetError(err.Error())
			return fmt.Errorf("记录核心停止状态: %w", err)
		}
	}

	operation.SetStage("stop_core")
	if err := m.Stop(ctx); err != nil {
		operation.SetError(err.Error())
		return err
	}
	return nil
}

func (m *Manager) Status(_ context.Context) Status {
	state := PersistentState{Version: persistentStateVersion}
	if m.store != nil {
		if loaded, err := m.store.Load(); err == nil {
			state = loaded
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	status := Status{InstalledVersion: state.InstalledVersion, InstalledDigest: state.InstalledDigest, State: m.state,
		CurrentProfileID: state.CurrentProfileID, AppliedRevision: state.AppliedRevision, ExpectedRunning: state.ExpectedRunning,
		ConfigDirectoryDigest: state.ConfigDirectoryDigest,
		LastError:             state.LastStartError, Operation: m.operation.Status()}
	if status.LastError == "" {
		status.LastError = m.lastError
	}
	if m.process != nil {
		status.PID = m.process.PID()
		status.StartedAt = cloneTime(m.startedAt)
		status.PendingProfileID = m.current.ProfileID
		if m.current.DraftRevision != 0 {
			revision := m.current.DraftRevision
			status.PendingRevision = &revision
		}
	}
	if m.process == nil && ensureExecutable(m.layout.CoreBinary) != nil && m.state == ProcessStopped {
		status.State = ProcessNotInstalled
	}
	return status
}

func (m *Manager) CommitApplication(profileID string, revision uint64, digest string) error {
	if m == nil || m.store == nil {
		return errors.New("核心状态存储不可用")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil || m.state != ProcessRunning || m.current.ProfileID != profileID || m.current.DraftRevision != revision {
		return ErrNotRunning
	}
	_, err := m.store.Update(func(state *PersistentState) error {
		state.CurrentProfileID = profileID
		state.AppliedRevision = &revision
		state.ExpectedRunning = true
		state.ConfigDirectoryDigest = digest
		state.LastStartError = ""
		now := time.Now().UTC()
		state.LastStartedAt = &now
		return nil
	})
	return err
}

// RestoreApplication rewrites the durable binding captured before a failed
// configuration transaction. Process recovery remains the coordinator's job.
func (m *Manager) RestoreApplication(previous Status) error {
	if m == nil || m.store == nil {
		return errors.New("核心状态存储不可用")
	}
	_, err := m.store.Update(func(state *PersistentState) error {
		state.CurrentProfileID = previous.CurrentProfileID
		state.AppliedRevision = cloneRevision(previous.AppliedRevision)
		state.ExpectedRunning = previous.ExpectedRunning
		state.ConfigDirectoryDigest = previous.ConfigDirectoryDigest
		state.LastStartError = previous.LastError
		state.LastStartedAt = cloneTime(previous.StartedAt)
		return nil
	})
	return err
}

func (m *Manager) ClearApplication() error {
	if m == nil || m.store == nil {
		return errors.New("核心状态存储不可用")
	}
	_, err := m.store.Update(func(state *PersistentState) error {
		state.CurrentProfileID = ""
		state.AppliedRevision = nil
		state.ExpectedRunning = false
		state.ConfigDirectoryDigest = ""
		state.LastStartError = ""
		state.LastStartedAt = nil
		return nil
	})
	return err
}

// RecordRecoveryFailure exposes a non-fatal startup recovery failure to the
// status endpoint without claiming that a core process is running.
func (m *Manager) RecordRecoveryFailure(err error) {
	if m == nil || err == nil {
		return
	}
	m.mu.Lock()
	m.process, m.state, m.lastError = nil, ProcessStartFailed, err.Error()
	m.mu.Unlock()
	m.persistError(err.Error(), false)
}

func (m *Manager) finishExit(process Process, result ProcessResult) {
	m.mu.Lock()
	if m.process != process {
		m.mu.Unlock()
		return
	}
	done := m.done
	m.process, m.startedAt, m.done = nil, nil, nil
	stopped := m.state == ProcessStopping
	if stopped {
		m.state, m.lastError = ProcessStopped, ""
	} else {
		m.state, m.lastError = ProcessExited, exitError(result)
	}
	// Finish the old process's last line before another Start can write to
	// the shared log, including startup failures without a trailing newline.
	_ = m.log.Flush()
	m.mu.Unlock()
	close(done)
	if !stopped {
		m.persistError(exitError(result), false)
	}
}

func (m *Manager) recordStartFailureLocked(err error) {
	m.process, m.state, m.lastError = nil, ProcessStartFailed, err.Error()
}

func (m *Manager) persistError(message string, expectedRunning bool) {
	if m.store == nil {
		return
	}
	_, _ = m.store.Update(func(state *PersistentState) error {
		state.ExpectedRunning = expectedRunning
		state.LastStartError = message
		return nil
	})
}

func exitError(result ProcessResult) string {
	if result.Err != nil {
		return fmt.Sprintf("核心异常退出（退出码 %d）：%v", result.ExitCode, result.Err)
	}
	return fmt.Sprintf("核心异常退出（退出码 %d）", result.ExitCode)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneRevision(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func allowedEnvironment() []string {
	return []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
}

func ensureExecutable(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return ErrNotInstalled
	}
	binDir := filepath.Dir(path)
	if err := storage.RejectSymlinkPath(binDir); err != nil {
		return ErrNotInstalled
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return ErrNotInstalled
	}
	return nil
}
