package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"singbox-web/internal/paths"
)

func TestManagerStartsFixedCommandAndOneProcess(t *testing.T) {
	manager, factory, clock := newTestManager(t)
	started := make(chan error, 1)
	go func() {
		started <- manager.Start(context.Background(), StartRequest{ProfileID: "profile-a", DraftRevision: 4})
	}()
	await(t, func() bool { return factory.LastStart().Path != "" })
	got := factory.LastStart()
	if got.Path != filepath.FromSlash("data/bin/sing-box") || !slices.Equal(got.Args, []string{"run", "-C", "config"}) || got.Dir != manager.Layout().WorkDir {
		t.Fatalf("start=%+v", got)
	}
	if err := manager.Start(context.Background(), StartRequest{ProfileID: "profile-b", DraftRevision: 1}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err=%v", err)
	}
	await(t, clock.HasWaiter)
	clock.Advance(2 * time.Second)
	if err := <-started; err != nil {
		t.Fatal(err)
	}
}

func TestStopKillsProcessGroupAfterTenSeconds(t *testing.T) {
	manager, factory, clock := newTestManager(t)
	started := make(chan error, 1)
	go func() { started <- manager.Start(context.Background(), StartRequest{}) }()
	await(t, func() bool { return factory.LastProcess() != nil })
	await(t, clock.HasWaiter)
	clock.Advance(2 * time.Second)
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	process := factory.LastProcess()
	stopped := make(chan error, 1)
	go func() { stopped <- manager.Stop(context.Background()) }()
	await(t, func() bool { return process.HasSignal(syscall.SIGTERM) })
	await(t, clock.HasWaiter)
	clock.Advance(10 * time.Second)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if !process.HasSignal(syscall.SIGKILL) {
		t.Fatal("process group was not SIGKILLed")
	}
}

func TestUnexpectedExitReportsFailureWithoutRestart(t *testing.T) {
	manager, factory, clock := newTestManager(t)
	started := make(chan error, 1)
	go func() { started <- manager.Start(context.Background(), StartRequest{}) }()
	await(t, func() bool { return factory.LastProcess() != nil })
	await(t, clock.HasWaiter)
	clock.Advance(2 * time.Second)
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	factory.LastProcess().Exit(ProcessResult{ExitCode: 23, Err: errors.New("exit status 23")})
	await(t, func() bool { return manager.Status(context.Background()).State == ProcessExited })
	status := manager.Status(context.Background())
	if status.PID != 0 || status.LastError == "" || factory.StartCount() != 1 {
		t.Fatalf("status=%+v starts=%d", status, factory.StartCount())
	}
}

func TestManagerRejectsSymlinkBinary(t *testing.T) {
	root := t.TempDir()
	layout, err := paths.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.BinDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/bin/true", layout.CoreBinary); err != nil {
		t.Fatal(err)
	}
	manager := newManager(layout, nil, nil, nil, &fakeFactory{}, &fakeClock{})
	if err := manager.Start(context.Background(), StartRequest{}); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("err=%v", err)
	}
}

type fakeFactory struct {
	mu        sync.Mutex
	starts    []ProcessSpec
	processes []*fakeProcess
}

func (f *fakeFactory) Start(spec ProcessSpec) (Process, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := newFakeProcess(len(f.processes) + 100)
	f.starts = append(f.starts, spec)
	f.processes = append(f.processes, p)
	return p, nil
}
func (f *fakeFactory) LastStart() ProcessSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.starts) == 0 {
		return ProcessSpec{}
	}
	return f.starts[len(f.starts)-1]
}
func (f *fakeFactory) LastProcess() *fakeProcess {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.processes) == 0 {
		return nil
	}
	return f.processes[len(f.processes)-1]
}
func (f *fakeFactory) StartCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.starts) }

type fakeProcess struct {
	pid     int
	exit    chan ProcessResult
	mu      sync.Mutex
	signals []syscall.Signal
	once    sync.Once
}

func newFakeProcess(pid int) *fakeProcess {
	return &fakeProcess{pid: pid, exit: make(chan ProcessResult, 1)}
}
func (p *fakeProcess) PID() int { return p.pid }
func (p *fakeProcess) SignalGroup(signal syscall.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, signal)
	p.mu.Unlock()
	return nil
}
func (p *fakeProcess) Exited() <-chan ProcessResult { return p.exit }
func (p *fakeProcess) Exit(result ProcessResult) {
	p.once.Do(func() { p.exit <- result; close(p.exit) })
}
func (p *fakeProcess) HasSignal(signal syscall.Signal) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Contains(p.signals, signal)
}

type fakeClock struct {
	mu      sync.Mutex
	waiters []fakeTimer
}
type fakeTimer struct {
	at time.Duration
	ch chan time.Time
}

func (c *fakeClock) After(duration time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	c.waiters = append(c.waiters, fakeTimer{at: duration, ch: ch})
	return ch
}
func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	var remaining []fakeTimer
	for _, waiter := range c.waiters {
		if waiter.at <= duration {
			waiter.ch <- time.Unix(0, 0)
		} else {
			waiter.at -= duration
			remaining = append(remaining, waiter)
		}
	}
	c.waiters = remaining
	c.mu.Unlock()
}
func (c *fakeClock) HasWaiter() bool { c.mu.Lock(); defer c.mu.Unlock(); return len(c.waiters) != 0 }

func newTestManager(t *testing.T) (*Manager, *fakeFactory, *fakeClock) {
	t.Helper()
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(layout.BinDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(layout.CoreBinary, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	factory, clock := &fakeFactory{}, &fakeClock{}
	return newManager(layout, nil, nil, nil, factory, clock), factory, clock
}
func await(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}
