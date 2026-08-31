package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"singbox-web/internal/configgen"
	"singbox-web/internal/core"
	"singbox-web/internal/updater"
)

type fixedRelease struct {
	release updater.Release
	err     error
}

func (f fixedRelease) LatestStable(context.Context) (updater.Release, error) { return f.release, f.err }

type fixedPreparer struct {
	prepared updater.PreparedBinary
	err      error
}

func (f fixedPreparer) Prepare(context.Context, updater.Release) (updater.PreparedBinary, func(), error) {
	return f.prepared, func() {}, f.err
}

type failingFactory struct{}

func (failingFactory) Start(core.ProcessSpec) (core.Process, error) {
	return nil, errors.New("injected start failure")
}

type failOnceFactory struct {
	starts int
}

func (factory *failOnceFactory) Start(core.ProcessSpec) (core.Process, error) {
	factory.starts++
	if factory.starts == 1 {
		return nil, errors.New("injected new-core start failure")
	}
	return &testProcess{done: make(chan core.ProcessResult)}, nil
}

type failingRunner struct{}

func (failingRunner) Run(context.Context, configgen.CommandSpec, time.Duration, int) (configgen.ProcessResult, error) {
	return configgen.ProcessResult{ExitCode: 1}, errors.New("injected check failure")
}

func updateStatus(t *testing.T, service *updater.Service) updater.Status {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status := service.Status()
		if status.FinishedAt != nil {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("update did not finish: %+v", service.Status())
	return updater.Status{}
}

func preparedProgram(t *testing.T, env *integrationEnvironment, version string, valid bool) updater.PreparedBinary {
	t.Helper()
	path := filepath.Join(env.workDir, "prepared-sing-box")
	if valid {
		writeCoreScript(t, path, version, 0, false)
	} else if err := os.WriteFile(path, []byte("#!/bin/sh\necho bad\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return updater.PreparedBinary{Path: path, Version: version, BinaryDigest: fmt.Sprintf("%x", sha256.Sum256(payload))}
}

func newUpdateService(t *testing.T, env *integrationEnvironment, release fixedRelease, preparer fixedPreparer, checker *configgen.Checker, factory core.ProcessFactory) (*updater.Service, *core.Store) {
	t.Helper()
	if checker == nil {
		checker = configgen.NewChecker(env.workDir, nil, time.Second, 64<<10)
	}
	store := core.NewStore(env.layout.CoreFile)
	manager := core.NewManagerWithFactory(env.layout, store, nil, core.NewOperationLock(), factory)
	return updater.NewService(env.layout, release, preparer, manager, store, checker, core.NewOperationLock()), store
}

func TestCoreUpdatePreflightFailuresDoNotReplaceProgram(t *testing.T) {
	cases := []struct {
		name       string
		releaseErr error
		prepareErr error
		valid      bool
	}{
		{"release", errors.New("release unavailable"), nil, true},
		{"checksum", nil, errors.New("checksum mismatch"), true},
		{"version", nil, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newIntegrationEnvironment(t)
			writeCoreScript(t, env.layout.CoreBinary, "old", 0, false)
			prepared := preparedProgram(t, env, "1.12.0", tc.valid)
			service, _ := newUpdateService(t, env, fixedRelease{release: updater.Release{Version: "1.12.0"}, err: tc.releaseErr}, fixedPreparer{prepared: prepared, err: tc.prepareErr}, nil, &testProcessFactory{})
			if _, err := service.Start(context.Background(), updater.UpdateConfirmation{ConfirmedVersion: "1.12.0"}); err != nil {
				t.Fatal(err)
			}
			status := updateStatus(t, service)
			if status.Stage != "failed" {
				t.Fatalf("status=%+v", status)
			}
			payload, err := os.ReadFile(env.layout.CoreBinary)
			if err != nil || !bytes.Contains(payload, []byte("old")) {
				t.Fatalf("old program changed: %v", err)
			}
			if _, err := os.Stat(env.layout.CoreBackup); !os.IsNotExist(err) {
				t.Fatalf("unexpected backup: %v", err)
			}
		})
	}
}

func TestCoreUpdateConfigCheckFailureDoesNotReplaceProgram(t *testing.T) {
	env := newIntegrationEnvironment(t)
	writeCoreScript(t, env.layout.CoreBinary, "old", 0, false)
	if err := os.MkdirAll(env.layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.layout.ConfigDir, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared := preparedProgram(t, env, "1.12.0", true)
	checker := configgen.NewChecker(env.workDir, failingRunner{}, time.Second, 64<<10)
	service, _ := newUpdateService(t, env, fixedRelease{release: updater.Release{Version: "1.12.0"}}, fixedPreparer{prepared: prepared}, checker, &testProcessFactory{})
	_, _ = service.Start(context.Background(), updater.UpdateConfirmation{ConfirmedVersion: "1.12.0"})
	if status := updateStatus(t, service); status.Stage != "failed" {
		t.Fatalf("status=%+v", status)
	}
	payload, _ := os.ReadFile(env.layout.CoreBinary)
	if !bytes.Contains(payload, []byte("old")) {
		t.Fatal("check failure replaced old program")
	}
}

func TestCoreUpdateStartFailureRestoresOldProgram(t *testing.T) {
	env := newIntegrationEnvironment(t)
	writeCoreScript(t, env.layout.CoreBinary, "old", 0, false)
	prepared := preparedProgram(t, env, "1.12.0", true)
	factory := &failOnceFactory{}
	service, store := newUpdateService(t, env, fixedRelease{release: updater.Release{Version: "1.12.0"}}, fixedPreparer{prepared: prepared}, nil, factory)
	revision := uint64(1)
	if _, err := store.Update(func(state *core.PersistentState) error {
		state.CurrentProfileID = "profile"
		state.AppliedRevision = &revision
		state.ExpectedRunning = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, _ = service.Start(context.Background(), updater.UpdateConfirmation{ConfirmedVersion: "1.12.0"})
	if status := updateStatus(t, service); status.Stage != "failed" {
		t.Fatalf("status=%+v", status)
	}
	payload, err := os.ReadFile(env.layout.CoreBinary)
	if err != nil || !bytes.Contains(payload, []byte("old")) {
		t.Fatalf("old program not restored: %v", err)
	}
	if _, err := os.Stat(env.layout.CoreBackup); err != nil {
		t.Fatalf("recovery should preserve one backup: %v", err)
	}
	state, err := store.Load()
	if err != nil || !state.ExpectedRunning || factory.starts != 2 {
		t.Fatalf("state=%+v starts=%d err=%v", state, factory.starts, err)
	}
}
