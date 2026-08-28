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

func TestValidationFailureRetainsProfileStagingDirectory(t *testing.T) {
	env := newIntegrationEnvironment(t)
	env.InstallFakeCore(t, fakeCoreBehavior{Version: "sing-box version 1.13.0", CheckExit: 7, Stderr: "FATAL invalid inbound"})
	profileID, revision := createApplicationProfile(t, env)
	result, err := env.config.Validate(context.Background(), profileID, revision)
	if err != nil {
		t.Fatal(err)
	}
	wantDirectory := "config.staging-" + profileID
	if result.Valid || result.ExitCode != 7 || result.ConfigDirectory != wantDirectory || result.Stderr != "FATAL invalid inbound" {
		t.Fatalf("result=%+v", result)
	}
	if len(result.Command) != 4 || result.Command[3] != wantDirectory {
		t.Fatalf("command=%v", result.Command)
	}
	assertPersistentStaging(t, env.workDir, profileID)
	if _, err := os.Stat(env.layout.ConfigDir); !os.IsNotExist(err) {
		t.Fatalf("validation touched formal config: %v", err)
	}
}

func TestApplicationCheckFailureLeavesRunningCoreAndFormalConfigUntouched(t *testing.T) {
	env := newIntegrationEnvironment(t)
	profileID, revision := createApplicationProfile(t, env)
	manager, coordinator, runner, store := newApplicationCoordinator(t, env, errors.New("invalid config"))
	prepareRunningCore(t, env, manager, store)
	defer manager.Stop(context.Background())

	result, err := coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: profileID, Revision: revision})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.Validation.Valid || result.Validation.ExitCode != 7 || result.Validation.Stderr != "FATAL invalid config" {
		t.Fatalf("result=%+v", result)
	}
	if !runner.checkedWhileRunning {
		t.Fatal("配置检查没有在现有核心保持运行时执行")
	}
	if status := manager.Status(context.Background()); status.State != core.ProcessRunning || status.CurrentProfileID != "old-profile" {
		t.Fatalf("check failure changed core status: %+v", status)
	}
	if marker, readErr := os.ReadFile(filepath.Join(env.layout.ConfigDir, "marker")); readErr != nil || string(marker) != "old" {
		t.Fatalf("formal config changed: %q err=%v", marker, readErr)
	}
	assertPersistentStaging(t, env.workDir, profileID)
}

func TestApplicationChecksAndCopiesConfigWithoutOperatingRunningCore(t *testing.T) {
	env := newIntegrationEnvironment(t)
	profileID, revision := createApplicationProfile(t, env)
	factory := &failSecondStartFactory{}
	manager, coordinator, runner, store := newApplicationCoordinatorWithFactory(t, env, nil, factory)
	prepareRunningCore(t, env, manager, store)
	defer manager.Stop(context.Background())
	before := manager.Status(context.Background())

	result, err := coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: profileID, Revision: revision})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || !result.Validation.Valid || !runner.checkedWhileRunning {
		t.Fatalf("result=%+v checkedWhileRunning=%v", result, runner.checkedWhileRunning)
	}
	status := manager.Status(context.Background())
	if status.State != core.ProcessRunning || status.PID != before.PID || status.CurrentProfileID != "old-profile" || status.AppliedRevision == nil || *status.AppliedRevision != 1 {
		t.Fatalf("status=%+v", status)
	}
	if factory.starts != 1 {
		t.Fatalf("应用触发了核心启动: starts=%d", factory.starts)
	}
	directoryDigest, err := application.DigestDirectory(env.layout.ConfigDir)
	if err != nil || directoryDigest == "" || status.ConfigDirectoryDigest == directoryDigest || status.ConfigDirectoryDigest != before.ConfigDirectoryDigest {
		t.Fatalf("directory digest=%q status=%q err=%v", directoryDigest, status.ConfigDirectoryDigest, err)
	}
	if _, err := os.Stat(filepath.Join(env.layout.ConfigDir, "60-inbounds.json")); err != nil {
		t.Fatalf("validated files were not copied to formal config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.layout.ConfigDir, "marker")); !os.IsNotExist(err) {
		t.Fatalf("old formal config remains: %v", err)
	}
	assertPersistentStaging(t, env.workDir, profileID)
	assertNoTransactionDirs(t, env.workDir)
	metas, err := env.profiles.List(context.Background())
	if err != nil || len(metas) != 1 || !metas[0].ConfigCurrent || metas[0].NeedsApply || metas[0].AppliedRevision == nil || *metas[0].AppliedRevision != revision {
		t.Fatalf("metas=%+v err=%v", metas, err)
	}
}

func TestApplicationCopiesConfigWithoutStartingStoppedCore(t *testing.T) {
	env := newIntegrationEnvironment(t)
	profileID, revision := createApplicationProfile(t, env)
	factory := &failSecondStartFactory{}
	manager, coordinator, _, _ := newApplicationCoordinatorWithFactory(t, env, nil, factory)

	result, err := coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: profileID, Revision: revision})
	if err != nil || !result.Applied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	status := manager.Status(context.Background())
	if status.State != core.ProcessStopped || status.PID != 0 || factory.starts != 0 {
		t.Fatalf("status=%+v", status)
	}
	if _, statErr := os.Stat(filepath.Join(env.layout.ConfigDir, "60-inbounds.json")); statErr != nil {
		t.Fatalf("配置未复制: %v", statErr)
	}
	metas, listErr := env.profiles.List(context.Background())
	if listErr != nil || len(metas) != 1 || !metas[0].ConfigCurrent || metas[0].NeedsApply || metas[0].AppliedRevision == nil || *metas[0].AppliedRevision != revision {
		t.Fatalf("metas=%+v err=%v", metas, listErr)
	}
	assertNoTransactionDirs(t, env.workDir)
}

type applicationCheckRunner struct {
	manager             *core.Manager
	err                 error
	checkedWhileRunning bool
}

func (runner *applicationCheckRunner) Run(_ context.Context, _ configgen.CommandSpec, _ time.Duration, _ int) (configgen.ProcessResult, error) {
	runner.checkedWhileRunning = runner.manager.Status(context.Background()).State == core.ProcessRunning
	if runner.err != nil {
		return configgen.ProcessResult{ExitCode: 7, Stderr: []byte("FATAL invalid config")}, runner.err
	}
	return configgen.ProcessResult{ExitCode: 0}, nil
}

func newApplicationCoordinator(t *testing.T, env *integrationEnvironment, checkErr error) (*core.Manager, *application.Coordinator, *applicationCheckRunner, *core.Store) {
	return newApplicationCoordinatorWithFactory(t, env, checkErr, &testProcessFactory{})
}

func newApplicationCoordinatorWithFactory(t *testing.T, env *integrationEnvironment, checkErr error, factory core.ProcessFactory) (*core.Manager, *application.Coordinator, *applicationCheckRunner, *core.Store) {
	t.Helper()
	writeCoreScript(t, env.layout.CoreBinary, "1.13.0", 0, false)
	operation := core.NewOperationLock()
	store := core.NewStore(env.layout.CoreFile)
	manager := core.NewManagerWithFactory(env.layout, store, nil, operation, factory)
	runner := &applicationCheckRunner{manager: manager, err: checkErr}
	checker := configgen.NewChecker(env.workDir, runner, time.Second, 64<<10)
	configuration := configgen.NewHandler(env.layout, env.profiles, env.rules, nil, checker, operation, time.Now)
	coordinator := application.NewCoordinator(env.profiles, configuration, nil, manager, operation)
	return manager, coordinator, runner, store
}

type failSecondStartFactory struct{ starts int }

func (factory *failSecondStartFactory) Start(core.ProcessSpec) (core.Process, error) {
	factory.starts++
	if factory.starts == 2 {
		return nil, errors.New("injected new-core start failure")
	}
	return &testProcess{done: make(chan core.ProcessResult)}, nil
}

func prepareRunningCore(t *testing.T, env *integrationEnvironment, manager *core.Manager, store *core.Store) {
	t.Helper()
	if err := os.MkdirAll(env.layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.layout.ConfigDir, "marker"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := application.DigestDirectory(env.layout.ConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordInstalled("1.13.0", "installed-digest"); err != nil {
		t.Fatal(err)
	}
	if err := manager.CommitApplication("old-profile", 1, digest); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background(), core.StartRequest{ProfileID: "old-profile", DraftRevision: 1}); err != nil {
		t.Fatal(err)
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

// The process factory keeps lifecycle assertions deterministic and ensures an
// accidental application start cannot spawn a real process.
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
