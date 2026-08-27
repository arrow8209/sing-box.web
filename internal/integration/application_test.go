package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"singbox-web/internal/app"
	"singbox-web/internal/application"
	"singbox-web/internal/auth"
	"singbox-web/internal/configgen"
	"singbox-web/internal/core"
	"singbox-web/internal/profile"
)

func createApplicationProfile(t *testing.T, env *integrationEnvironment) (string, uint64) {
	t.Helper()
	meta, draft, err := env.profiles.Create(context.Background(), profile.CreateInput{Name: "应用故障测试"})
	if err != nil {
		t.Fatal(err)
	}
	buffers := make(map[string]string, len(draft.Buffers))
	for module := range draft.Buffers {
		buffers[module] = ""
	}
	buffers["route"] = `{}`
	buffers["inbounds"] = `[]`
	buffers["outbounds"] = `[]`
	updated, _, err := env.profiles.SaveDraft(context.Background(), meta.ID, profile.RevisionETag(draft.Revision), profile.DraftInput{Buffers: buffers})
	if err != nil {
		t.Fatal(err)
	}
	return meta.ID, updated.Revision
}

func newCoordinatorForTest(t *testing.T, env *integrationEnvironment, checkExit int) (*application.Coordinator, *core.Store) {
	t.Helper()
	writeCoreScript(t, env.layout.CoreBinary, "1.12.0", checkExit, false)
	checker := configgen.NewChecker(env.workDir, configgen.ExecRunner{}, time.Second, 64<<10)
	store := core.NewStore(env.layout.CoreFile)
	manager := core.NewManagerWithFactory(env.layout, store, nil, core.NewOperationLock(), &testProcessFactory{})
	return application.NewCoordinator(env.profiles, env.rules, nil, checker, manager, core.NewOperationLock()), store
}

func TestApplicationCheckFailureLeavesFormalConfigUntouched(t *testing.T) {
	env := newIntegrationEnvironment(t)
	profileID, revision := createApplicationProfile(t, env)
	coordinator, _ := newCoordinatorForTest(t, env, 7)
	_, err := coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: profileID, Revision: revision})
	if !errors.Is(err, application.ErrCheckFailed) {
		t.Fatalf("err=%v", err)
	}
	var failure *application.CheckFailure
	if !errors.As(err, &failure) {
		t.Fatalf("missing detailed check failure: %v", err)
	}
	if failure.Result.ExitCode != 7 || failure.CheckedConfigDirectory == "" {
		t.Fatalf("failure=%+v", failure)
	}
	if failure.ProfileID != profileID || failure.Revision != revision || failure.ConfigDigest == "" {
		t.Fatalf("candidate identity=%+v", failure)
	}
	wantReview := filepath.Join(env.workDir, "config.check-failed")
	if failure.ReviewConfigDirectory != wantReview || failure.ReviewError != nil {
		t.Fatalf("review=%q err=%v", failure.ReviewConfigDirectory, failure.ReviewError)
	}
	if _, statErr := os.Stat(filepath.Join(wantReview, "60-inbounds.json")); statErr != nil {
		t.Fatalf("failed candidate was not retained: %v", statErr)
	}
	if _, statErr := os.Stat(env.layout.ConfigDir); !os.IsNotExist(statErr) {
		t.Fatalf("failed check installed config: %v", statErr)
	}
	assertNoTransactionDirs(t, env.workDir)
	metas, err := env.profiles.List(context.Background())
	if err != nil || len(metas) != 1 || !metas[0].NeedsApply {
		t.Fatalf("metas=%+v err=%v", metas, err)
	}
}

func TestApplicationRefusesExternalConfigDriftBeforeStop(t *testing.T) {
	env := newIntegrationEnvironment(t)
	profileID, revision := createApplicationProfile(t, env)
	coordinator, store := newCoordinatorForTest(t, env, 0)
	if err := os.MkdirAll(env.layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.layout.ConfigDir, "host.json"), []byte("host-edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(func(state *core.PersistentState) error { state.ConfigDirectoryDigest = "different"; return nil }); err != nil {
		t.Fatal(err)
	}
	_, err := coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: profileID, Revision: revision})
	if !errors.Is(err, application.ErrExternalDrift) {
		t.Fatalf("err=%v", err)
	}
	if _, statErr := os.Stat(filepath.Join(env.layout.ConfigDir, "host.json")); statErr != nil {
		t.Fatalf("drift config changed: %v", statErr)
	}
}

func TestServerRestartRestoresExpectedCore(t *testing.T) {
	env := newIntegrationEnvironment(t)
	meta, draft, err := env.profiles.Create(context.Background(), profile.CreateInput{Name: "恢复测试"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(env.layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.layout.ConfigDir, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := application.DigestDirectory(env.layout.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := env.profiles.MarkApplied(context.Background(), profile.AppliedResult{ProfileID: meta.ID, Revision: draft.Revision, Digest: digest, ResourceDigests: map[string]string{}, AppliedAt: now, Validation: profile.ValidationSummary{Valid: true, Revision: draft.Revision, CheckedAt: now}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(env.layout.BinDir, 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$1\" in\nversion) echo 'sing-box version 1.12.0';;\ncheck) exit 0;;\nrun) echo run >> fake-run.log; trap 'exit 0' TERM INT; while :; do sleep 1; done;;\nesac\n"
	if err := os.WriteFile(env.layout.CoreBinary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	store := core.NewStore(env.layout.CoreFile)
	if err := store.RecordInstalled("old", "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(func(state *core.PersistentState) error {
		state.CurrentProfileID = meta.ID
		state.AppliedRevision = &draft.Revision
		state.ExpectedRunning = true
		state.ConfigDirectoryDigest = digest
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(env.layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	server, err := app.NewServer(env.workDir, app.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	if contents, err := os.ReadFile(filepath.Join(env.workDir, "fake-run.log")); err != nil || string(contents) != "run\n" {
		t.Fatalf("core was not restored: %q err=%v", contents, err)
	}
	state, err := store.Load()
	if err != nil || !state.ExpectedRunning || state.InstalledVersion != "1.12.0" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

// The process factory is only used when an application reaches its final
// start step. These failure tests stop earlier, but keeping it deterministic
// ensures an accidental start cannot spawn a real process.
type testProcessFactory struct{}

func (*testProcessFactory) Start(core.ProcessSpec) (core.Process, error) {
	return &testProcess{done: make(chan core.ProcessResult)}, nil
}

type testProcess struct{ done chan core.ProcessResult }

func (*testProcess) PID() int { return 4242 }
func (p *testProcess) SignalGroup(_ syscall.Signal) error {
	select {
	case p.done <- core.ProcessResult{}:
	default:
	}
	return nil
}
func (p *testProcess) Exited() <-chan core.ProcessResult { return p.done }

func writeCoreScript(t *testing.T, path, version string, checkExit int, runForever bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	run := "exit 0"
	if runForever {
		run = "trap 'exit 0' TERM INT; while :; do sleep 1; done"
	}
	script := "#!/bin/sh\ncase \"$1\" in\nversion) echo 'sing-box version " + version + "'; exit 0;;\ncheck) exit " + strconv.Itoa(checkExit) + ";;\nrun) " + run + ";;\nesac\nexit 2\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}
