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
	ErrAlreadyRunning = errors.New("core_already_running")
	ErrNotInstalled   = errors.New("core_not_installed")
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
// bindings are intentionally owned by the later application coordinator.
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
	m.state = ProcessStopping
	m.mu.Unlock()
	if err := process.SignalGroup(syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
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
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
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
		LastError: state.LastStartError, Operation: m.operation.Status()}
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
	m.mu.Unlock()
	close(done)
	if !stopped && result.Err != nil {
		m.persistError(exitError(result), false)
	}
}

func (m *Manager) recordStartFailureLocked(err error) {
	m.process, m.state, m.lastError = nil, ProcessStartFailed, err.Error()
	go m.persistError(err.Error(), false)
}

func (m *Manager) persistError(message string, _ bool) {
	if m.store == nil {
		return
	}
	_, _ = m.store.Update(func(state *PersistentState) error { state.LastStartError = message; return nil })
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
