//go:build linux

package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"singbox-web/internal/paths"
)

func TestLinuxProcessFactoryReportsExit(t *testing.T) {
	process, err := (LinuxProcessFactory{}).Start(ProcessSpec{
		Path: "/bin/true", Dir: filepath.Dir("/bin/true"), Env: allowedEnvironment(), Stdout: io.Discard, Stderr: io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := <-process.Exited()
	if !ok || result.Err != nil || result.ExitCode != 0 {
		t.Fatalf("result=%+v open=%t", result, ok)
	}
}

func TestRequestStopTerminatesOwnedLinuxProcess(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.BinDir, 0o700); err != nil {
		t.Fatal(err)
	}
	program := "#!/bin/sh\ntrap 'exit 0' TERM INT\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(layout.CoreBinary, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	store := NewStore(layout.CoreFile)
	if _, err := store.Update(func(state *PersistentState) error {
		state.ExpectedRunning = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(layout, store, nil, nil)
	t.Cleanup(func() { _ = manager.Stop(context.Background()) })
	if err := manager.Start(context.Background(), StartRequest{}); err != nil {
		t.Fatal(err)
	}
	pid := manager.Status(context.Background()).PID
	if pid <= 0 {
		t.Fatalf("pid=%d", pid)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.RequestStop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("managed process %d still exists: %v", pid, err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.ExpectedRunning {
		t.Fatal("ExpectedRunning remained true")
	}
}
