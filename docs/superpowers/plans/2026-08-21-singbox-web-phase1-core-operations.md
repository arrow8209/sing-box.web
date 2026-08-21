# sing-box Web 一期核心生命周期与应用事务 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付唯一受管 sing-box 子进程、可恢复的配置应用事务、真实状态报告，以及带官方摘要校验和单备份回退的手动核心下载/更新。

**Architecture:** `core.Manager` 是唯一能创建和终止子进程的对象，应用与更新通过同一个 `core.OperationLock` 串行化。`application.Coordinator` 在资源快照锁内完成停止、生成、检查、目录切换和启动，任何失败都按明确阶段恢复；`updater.Service` 先在锁外下载与预检，再在锁内停止、替换、启动和回退，缩短核心停机时间。

**Tech Stack:** Go 1.26、Linux `os/exec`/进程组信号/`Pdeathsig`、标准库 tar/gzip/sha256/net/http、上一计划的 storage/rules/configgen/profile/httpapi。

## Global Constraints

- CoreManager 直接持有一个 sing-box 子进程，不调用 systemd，同一时刻只允许一个受管子进程。
- 子进程使用相对程序路径、与 Web 相同的固定工作目录和参数数组，不经过 shell，环境变量采用明确允许列表。
- 停止时先发送优雅退出信号，默认等待 10 秒；超时后强制终止整个受管进程组。
- Linux 下必须确保 Web 异常退出后受管子进程不会成为失去管理的孤儿；一期不无限自动重启意外退出的核心。
- sing-box 始终读取固定 `config`，应用其他档案会停止当前核心并整体替换该目录。
- “应用”必须依次停止核心、生成模块、实际 `check -C`、切换目录、`run -C config` 并观察启动；不允许绕过检查。
- `config.staging-*` 和 `config.rollback-*` 只存在于一次事务，成功或恢复后立即删除，不构成历史版本。
- 外部修改固定 `config` 时必须阻止静默覆盖并要求管理员明确确认；外部内容不导入草稿。
- 核心不存在时允许编辑、保存与规则下载，但禁止应用并提示手动下载。
- 核心更新只由管理员确认后执行，只选择官方最新稳定版，必须校验官方 SHA-256、压缩包安全、真实 `version` 和当前配置兼容性；禁止自动更新。
- `data/bin/sing-box.bak` 只保留更新前唯一一份程序，首次下载不创建备份，每次更新覆盖旧备份。
- Web 正常关闭时优雅停止核心；再次启动时仅在 `core.json` 记录期望运行且存在已应用档案时恢复。

---

## File Responsibility Map

- `internal/core/model.go`、`store.go`：持久安装/期望状态与进程内真实运行状态。
- `internal/core/process_linux.go`：只负责 Linux 子进程创建、进程组信号和退出等待。
- `internal/core/log.go`：限制单条和总文件大小，不提供完整日志 API。
- `internal/core/manager.go`：唯一 Start/Stop/Status、启动观察和意外退出状态。
- `internal/core/operation_lock.go`：应用、验证、更新和生命周期共用的可观察全局操作。
- `internal/application/`：配置漂移、apply 编排、目录回滚与当前档案删除。
- `internal/updater/`：稳定 Release 解析、安全下载解压、预检、安装和程序回退。
- `internal/integration/`：每个故障点的可执行 fake core 集成验证。

### Task 1: 建立核心状态、受限日志和全局操作锁

**Files:**
- Create: `internal/core/model.go`
- Create: `internal/core/store.go`
- Create: `internal/core/store_test.go`
- Create: `internal/core/log.go`
- Create: `internal/core/log_test.go`
- Create: `internal/core/operation_lock.go`
- Create: `internal/core/operation_lock_test.go`
- Modify: `internal/configgen/gate.go`

**Interfaces:**
- Consumes: `paths.Layout.CoreFile`/`CoreLog`、UTC clock 和基础计划的 `audit.Logger`。
- Produces: `core.Store.Load/Update`、`core.NewBoundedLog(path, 4<<20, 16<<10)`、`OperationLock.Enter(ctx, kind) (*Operation, release, error)`；config validate 改用同一锁。

- [ ] **Step 1: 写损坏状态、日志上限、单操作和可观察阶段测试**

```go
func TestCoreStoreRejectsCorruptState(t *testing.T) {
    path := filepath.Join(t.TempDir(), "data", "core.json")
    os.MkdirAll(filepath.Dir(path), 0o700)
    os.WriteFile(path, []byte(`{"version":1,"unknown":true}`), 0o600)
    _, err := core.NewStore(path).Load()
    var corrupt *storage.CorruptFileError
    if !errors.As(err, &corrupt) || corrupt.Path != path { t.Fatalf("err=%v", err) }
}

func TestBoundedLogLimitsLineAndTotalSize(t *testing.T) {
    path := filepath.Join(t.TempDir(), "sing-box.log")
    log, _ := core.NewBoundedLog(path, 4096, 128)
    log.Write(append(bytes.Repeat([]byte("x"), 300), '\n'))
    for range 100 { log.Write([]byte("normal line\n")) }
    log.Close()
    payload, _ := os.ReadFile(path)
    if len(payload) > 4096 || bytes.Contains(payload, bytes.Repeat([]byte("x"), 129)) { t.Fatalf("log limits failed: %d", len(payload)) }
}

func TestOperationLockPublishesCurrentStage(t *testing.T) {
    lock := core.NewOperationLock()
    operation, release, err := lock.Enter(context.Background(), core.OperationApply)
    if err != nil { t.Fatal(err) }
    operation.SetStage("check_config")
    status := lock.Status()
    if status.Kind != "apply" || status.Stage != "check_config" { t.Fatalf("status=%+v", status) }
    release()
    if lock.Status().Active { t.Fatal("operation remained active") }
}
```

- [ ] **Step 2: 运行核心基础测试并确认失败**

Run: `go test ./internal/core -run 'TestCoreStore|TestBoundedLog|TestOperationLock' -v`

Expected: FAIL，核心包或方法不存在。

- [ ] **Step 3: 定义持久状态和公开运行状态**

```go
type PersistentState struct {
    Version               int        `json:"version"`
    InstalledVersion      string     `json:"installedVersion"`
    InstalledDigest       string     `json:"installedDigest"`
    CurrentProfileID      string     `json:"currentProfileId"`
    AppliedRevision       *uint64    `json:"appliedRevision"`
    ExpectedRunning       bool       `json:"expectedRunning"`
    ConfigDirectoryDigest string     `json:"configDirectoryDigest"`
    LastStartError        string     `json:"lastStartError"`
    LastStartedAt         *time.Time `json:"lastStartedAt"`
}

type Status struct {
    InstalledVersion string          `json:"installedVersion"`
    InstalledDigest  string          `json:"installedDigest"`
    State            ProcessState    `json:"state"`
    PID              int             `json:"pid"`
    CurrentProfileID string          `json:"currentProfileId"`
    AppliedRevision  *uint64         `json:"appliedRevision"`
    ExpectedRunning  bool            `json:"expectedRunning"`
    StartedAt        *time.Time      `json:"startedAt"`
    LastError        string          `json:"lastError"`
    Operation        OperationStatus `json:"operation"`
}

func (s Status) Running() bool { return s.State == ProcessRunning || s.State == ProcessStarting }
```

进程状态枚举固定为 `not_installed`、`stopped`、`starting`、`running`、`stopping`、`start_failed`、`exited`。PID 只在内存状态中存在，重启时绝不信任旧 PID。

- [ ] **Step 4: 实现有界日志与操作锁**

日志把 stdout/stderr 逐行合并写入 `data/logs/sing-box.log`，每行最大 16 KiB，超出部分追加 ` [truncated]`；文件达到 4 MiB 时在同一文件内保留最新 2 MiB 的完整行并同步，不创建日志版本。`BoundedLog.Recent(64<<10)` 只在启动失败或意外退出时为 `Status.LastError` 提供最近原始输出，仍受 64 KiB 限制；API 不提供完整日志读取。OperationLock 同一时刻只允许一个操作，等待遵守 context；状态包括 kind、stage、startedAt，错误结果最多 64 KiB。

- [ ] **Step 5: 将配置检查 gate 替换为共享锁适配器**

```go
func (l *OperationLock) EnterGate(ctx context.Context, kind string) (func(), error) {
    _, release, err := l.Enter(ctx, OperationKind(kind))
    return release, err
}
```

验证使用 kind=`validate`，应用和更新分别使用 `apply`、`core_update`。handler 不持有自己的 mutex，避免验证与更新交换二进制时并发。

- [ ] **Step 6: 运行竞态与文件上限测试**

Run: `go test -race ./internal/core ./internal/configgen -v`

Expected: PASS；第二个操作等待或随 context 取消；每次释放清空活动状态；损坏 `core.json` 阻止服务进入可写状态。

- [ ] **Step 7: 提交核心状态基础设施**

```bash
git add internal/core internal/configgen/gate.go
git commit -m "建立核心状态与全局操作锁"
```

### Task 2: 实现唯一 Linux 子进程的启动、停止与退出观察

**Files:**
- Create: `internal/core/process.go`
- Create: `internal/core/process_linux.go`
- Create: `internal/core/process_linux_test.go`
- Create: `internal/core/manager.go`
- Create: `internal/core/manager_test.go`
- Create: `internal/core/handler.go`
- Create: `internal/core/handler_test.go`
- Modify: `internal/httpapi/router.go`

**Interfaces:**
- Consumes: 固定 layout、`core.Store`、`BoundedLog`、允许列表环境和 process factory。
- Produces: `Manager.Start(ctx, StartRequest) error`、`Stop(ctx) error`、`Status(ctx) Status`、`GET /api/v1/core`；只有 Manager 导入 `os/exec`。

- [ ] **Step 1: 写参数、工作目录、单进程、优雅/强杀和意外退出测试**

```go
func TestManagerStartsFixedCommandAndOneProcess(t *testing.T) {
    manager, factory := newManagerWithFakeProcess(t)
    err := manager.Start(context.Background(), core.StartRequest{ProfileID: "profile-a", DraftRevision: 4})
    if err != nil { t.Fatal(err) }
    started := factory.LastStart()
    if started.Path != filepath.FromSlash("data/bin/sing-box") { t.Fatalf("path=%q", started.Path) }
    if !slices.Equal(started.Args, []string{"run", "-C", "config"}) || started.Dir != manager.Layout().WorkDir { t.Fatalf("start=%+v", started) }
    if err := manager.Start(context.Background(), core.StartRequest{ProfileID: "profile-b", DraftRevision: 1}); !errors.Is(err, core.ErrAlreadyRunning) { t.Fatalf("err=%v", err) }
}

func TestStopKillsProcessGroupAfterTenSeconds(t *testing.T) {
    clock := newFakeClock(time.Unix(0, 0))
    manager, process := newManagerWithHungProcess(t, clock)
    startManager(t, manager)
    stopped := make(chan error, 1)
    go func() { stopped <- manager.Stop(context.Background()) }()
    awaitSignal(t, process, syscall.SIGTERM)
    clock.Advance(10 * time.Second)
    if err := <-stopped; err != nil { t.Fatal(err) }
    if !process.ProcessGroupKilled() { t.Fatal("process group was not SIGKILLed") }
}

func TestUnexpectedExitReportsRealFailureWithoutRestart(t *testing.T) {
    manager, process := newManagerWithFakeProcess(t)
    startManager(t, manager)
    process.Exit(23, errors.New("exit status 23"))
    awaitState(t, manager, core.ProcessExited)
    status := manager.Status(context.Background())
    if status.PID != 0 || !strings.Contains(status.LastError, "23") || process.StartCount() != 1 { t.Fatalf("status=%+v starts=%d", status, process.StartCount()) }
}
```

- [ ] **Step 2: 运行进程测试并确认失败**

Run: `go test ./internal/core -run 'TestManager|TestStop|TestUnexpectedExit' -v`

Expected: FAIL，缺少 Manager 或 process abstraction。

- [ ] **Step 3: 实现 Linux process factory 与防孤儿属性**

```go
func (f LinuxProcessFactory) Start(spec ProcessSpec) (Process, error) {
    ready := make(chan startResult, 1)
    go func() {
        runtime.LockOSThread()
        defer runtime.UnlockOSThread()
        cmd := exec.Command(spec.Path, spec.Args...)
        cmd.Dir, cmd.Env = spec.Dir, slices.Clone(spec.Env)
        cmd.Stdout, cmd.Stderr = spec.Stdout, spec.Stderr
        cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
        if err := cmd.Start(); err != nil { ready <- startResult{err: err}; return }
        process := &linuxProcess{cmd: cmd, pgid: cmd.Process.Pid, exit: make(chan ProcessResult, 1)}
        ready <- startResult{process: process}
        process.exit <- resultFromWait(cmd.Wait())
        close(process.exit)
    }()
    result := <-ready
    return result.process, result.err
}

func (p *linuxProcess) SignalGroup(signal syscall.Signal) error {
    return syscall.Kill(-p.pgid, signal)
}

func (p *linuxProcess) Exited() <-chan ProcessResult { return p.exit }
```

`ProcessSpec.Path` 固定为相对 `data/bin/sing-box`，`Dir` 固定为 Web 工作目录；任何客户端值都不能进入二者。由于 Linux `Pdeathsig` 绑定创建子进程的线程，factory 在专用 goroutine 中调用 `runtime.LockOSThread()`，在同一锁定线程完成 `cmd.Start()` 和 `cmd.Wait()`，收到退出后才 unlock，避免 Go 运行时提前销毁父线程导致核心误退出。测试子进程读取父进程退出 pipe，验证父 helper 异常退出后子进程在 2 秒内消失；测试只操作自身创建的进程组。

`ensureExecutable` 对固定绝对路径执行 `Lstat`，只接受当前用户可执行的普通非符号链接文件，并再次确认 `data/bin` 没有符号链接组件；不通过时状态为未安装或不可用，绝不跟随到工作目录外程序。

- [ ] **Step 4: 实现 Manager 状态机和 2 秒启动观察期**

```go
type StartRequest struct {
    ProfileID    string
    DraftRevision uint64
}

func (m *Manager) Start(ctx context.Context, request StartRequest) error {
    m.mu.Lock()
    if m.process != nil { m.mu.Unlock(); return ErrAlreadyRunning }
    if err := ensureExecutable(m.layout.CoreBinary); err != nil { m.mu.Unlock(); return ErrNotInstalled }
    m.state = ProcessStarting
    process, err := m.factory.Start(ProcessSpec{Path: filepath.FromSlash("data/bin/sing-box"), Args: []string{"run", "-C", "config"}, Dir: m.layout.WorkDir, Env: allowedEnvironment(), Stdout: m.log, Stderr: m.log})
    if err != nil { m.recordStartFailureLocked(err); m.mu.Unlock(); return err }
    m.process = process
    m.current = request
    m.mu.Unlock()
    exit := m.observe(process)
    select {
    case result := <-exit:
        return m.finishEarlyExit(process, result)
    case <-m.clock.After(2 * time.Second):
        return m.markRunning(process, request)
    case <-ctx.Done():
        _ = m.Stop(context.Background())
        return context.Cause(ctx)
    }
}
```

启动观察后由 goroutine 继续等待退出。`Stop` 只改变进程内事实，不改变持久 `ExpectedRunning`；Web shutdown、应用切换、更新和当前档案删除分别由各自协调器决定随后保留或清除期望状态。停止旧核心失败时保留真实状态并返回错误，调用者不得继续写配置。

Manager 只更新进程内 PID/state/pending StartRequest，并可持久化最近启动/退出错误；它不得自行改写 `CurrentProfileID/AppliedRevision/ExpectedRunning/ConfigDirectoryDigest`。这些正式关联由 application Coordinator 在目录切换和启动观察都成功后一次提交，由 updater 在程序回退完成后保持原值，由 startup recovery 从既有持久状态恢复。`Status` 合并持久关联与运行时进程事实，操作进行中另显示 pending profile，避免崩溃时把尚未提交的新档案伪装成已应用。

- [ ] **Step 5: 暴露真实核心状态并验证无独立控制端点**

`GET /core` 返回安装版本/摘要、状态、PID、当前档案、应用修订、启动时间、最近错误和当前操作。不得添加 `POST /core/start`、`stop` 或 `restart`。缺失程序时状态为 `not_installed`，档案与规则 API 仍正常。

- [ ] **Step 6: 运行进程、handler 和竞态测试**

Run: `go test -race ./internal/core ./internal/httpapi -v`

Expected: PASS；命令始终是参数数组；第二个进程不会启动；停止超时向整个测试进程组强杀；意外退出不自动重启且 PID 归零。

- [ ] **Step 7: 提交核心子进程管理**

```bash
git add internal/core internal/httpapi
git commit -m "实现唯一核心子进程管理"
```

### Task 3: 实现正式配置漂移检测与目录切换原语

**Files:**
- Create: `internal/application/drift.go`
- Create: `internal/application/drift_test.go`
- Create: `internal/application/directories.go`
- Create: `internal/application/directories_test.go`
- Modify: `internal/storage/cleanup.go`

**Interfaces:**
- Consumes: 固定工作目录、固定 `config`、期望目录摘要。
- Produces: `application.DigestDirectory(configDir) (string, error)`、`CheckDrift(expected, confirmed string) (DriftResult, error)`、`BeginDirectorySwap(layout, staging, expectedCurrentDigest) (*DirectoryTransaction, error)`。

- [ ] **Step 1: 写稳定目录摘要、明确确认和恢复测试**

```go
func TestCheckDriftRequiresExactCurrentDigestConfirmation(t *testing.T) {
    dir := writeConfigDirectory(t, map[string]string{"10-log.json": `{"log":{"level":"info"}}\n`})
    expected, _ := application.DigestDirectory(dir)
    os.WriteFile(filepath.Join(dir, "10-log.json"), []byte(`{"log":{"level":"debug"}}`+"\n"), 0o600)
    current, _ := application.DigestDirectory(dir)
    result, err := application.CheckDrift(expected, "", current)
    if !errors.Is(err, application.ErrExternalDrift) || result.ConfirmationDigest != current { t.Fatalf("result=%+v err=%v", result, err) }
    if _, err := application.CheckDrift(expected, current, current); err != nil { t.Fatalf("confirmation rejected: %v", err) }
}

func TestSwapRestoreReturnsOldConfig(t *testing.T) {
    layout := newLayoutWithConfig(t, "old")
    staging := newStaging(t, layout, "new")
    expected, _ := application.DigestDirectory(layout.ConfigDir)
    tx, err := application.BeginDirectorySwap(layout, staging, expected)
    if err != nil { t.Fatal(err) }
    if got := readMarker(t, layout.ConfigDir); got != "new" { t.Fatalf("after swap=%q", got) }
    if err := tx.Restore(); err != nil { t.Fatal(err) }
    if got := readMarker(t, layout.ConfigDir); got != "old" { t.Fatalf("after restore=%q", got) }
    tx.Cleanup()
    assertNoTransactionDirs(t, layout.WorkDir)
}
```

- [ ] **Step 2: 运行目录事务测试并确认失败**

Run: `go test ./internal/application ./internal/storage -run 'TestCheckDrift|TestSwap' -v`

Expected: FAIL，缺少应用目录包。

- [ ] **Step 3: 实现可确认外部内容的安全目录摘要**

摘要用 `filepath.WalkDir` 对相对路径排序，纳入每项相对路径、类型、权限和内容；普通文件读取内容，符号链接只读取 link target 而不跟随，目录只记录自身。最多 1024 项、普通文件单个 16 MiB、合计 64 MiB，越限或无权限时拒绝自动覆盖并要求管理员先在主机侧清理；非固定文件、子目录和符号链接都能得到确认 digest，而不是让 UI 永远无法确认。目录不存在返回空摘要，供首次应用使用。生成器测试另保证 Web 自己写出的目录只含固定模块普通文件。

```go
func DigestDirectory(dir string) (string, error) {
    entries, err := collectTreeEntries(dir, 1024)
    if errors.Is(err, os.ErrNotExist) { return "", nil }
    if err != nil { return "", err }
    slices.SortFunc(entries, func(a, b treeEntry) int { return strings.Compare(a.RelativePath, b.RelativePath) })
    hash := sha256.New()
    for _, entry := range entries {
        hash.Write([]byte(entry.RelativePath)); hash.Write([]byte{0}); hash.Write([]byte(entry.Type)); hash.Write([]byte{0}); hash.Write([]byte(entry.Mode.String())); hash.Write([]byte{0})
        if entry.Type == "file" { payload, err := readLimited(entry.AbsolutePath, 16<<20); if err != nil { return "", err }; hash.Write(payload) }
        if entry.Type == "symlink" { target, err := os.Readlink(entry.AbsolutePath); if err != nil { return "", err }; hash.Write([]byte(target)) }
    }
    return hex.EncodeToString(hash.Sum(nil)), nil
}
```

- [ ] **Step 4: 实现同父目录原子切换和幂等恢复**

`BeginDirectorySwap` 验证 staging 名称为 `config.staging-<32hex>` 且直接位于工作目录。若旧 `config` 存在，先改名为新生成的 `config.rollback-<32hex>` 并同步父目录，立即重算 rollback 摘要并要求等于 `expectedCurrentDigest`；不等则原子恢复并返回漂移，关闭第二次摘要与 rename 之间的覆盖窗口。匹配后再把 staging 改名为 `config`；第二步失败立即把 rollback 改回。`Restore` 删除失败启动的新 config，再把 rollback 改回；首次应用无 rollback 时删除新 config。`Cleanup` 只删除该事务拥有的路径且不跟随其中符号链接。

- [ ] **Step 5: 运行故障注入和清理测试**

Run: `go test -race ./internal/application ./internal/storage -v`

Expected: PASS；每个 rename/fsync 故障点后最多有一个可识别的正式目录；成功/恢复后无 staging/rollback；确认 digest 只接受本次重新计算的当前摘要。

- [ ] **Step 6: 提交配置目录事务原语**

```bash
git add internal/application internal/storage/cleanup.go
git commit -m "实现正式配置目录切换与漂移检测"
```

### Task 4: 编排完整“应用”事务与当前档案删除

**Files:**
- Create: `internal/application/coordinator.go`
- Create: `internal/application/coordinator_test.go`
- Create: `internal/application/handler.go`
- Create: `internal/application/handler_test.go`
- Modify: `internal/profile/service.go`
- Modify: `internal/profile/handler.go`
- Modify: `internal/core/handler.go`
- Modify: `internal/httpapi/router.go`

**Interfaces:**
- Consumes: `core.OperationLock`、`profile.Service`、`rules.Service`、`configgen.Generator/Checker`、`core.Manager` 和固定 layout。
- Produces: `Coordinator.Apply(ctx, ApplyRequest) (ApplyResult, error)`、`POST /api/v1/profiles/{id}/apply`；当前档案删除通过 `Coordinator.DeleteCurrentProfile`。

- [ ] **Step 1: 写成功顺序、修订冲突、检查失败和启动失败恢复测试**

```go
func TestApplyRunsStrictSequenceAndCommitsState(t *testing.T) {
    env := newApplyEnvironment(t, runningProfile("profile-old", 2), validDraft("profile-new", 5))
    result, err := env.Coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: "profile-new", Revision: 5})
    if err != nil { t.Fatal(err) }
    if !slices.Equal(env.Events(), []string{"lock_global", "lock_rules", "stop", "generate", "write_staging", "check", "swap", "start", "mark_applied", "cleanup", "unlock_rules", "unlock_global"}) { t.Fatalf("events=%v", env.Events()) }
    if result.ProfileID != "profile-new" || result.Revision != 5 || env.CoreStatus().CurrentProfileID != "profile-new" { t.Fatalf("result=%+v status=%+v", result, env.CoreStatus()) }
}

func TestApplyCheckFailureKeepsOldConfigAndRestartsOldCore(t *testing.T) {
    env := newApplyEnvironment(t, runningProfile("profile-old", 2), validDraft("profile-new", 5))
    env.Checker.Fail(errors.New("unknown field future_bad"))
    _, err := env.Coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: "profile-new", Revision: 5})
    if !errors.Is(err, application.ErrCheckFailed) { t.Fatalf("err=%v", err) }
    if env.ConfigMarker() != "old" || env.CoreStatus().CurrentProfileID != "profile-old" { t.Fatalf("config=%q status=%+v", env.ConfigMarker(), env.CoreStatus()) }
    assertNoTransactionDirs(t, env.WorkDir())
}

func TestApplyStartFailureRestoresOldDirectoryAndProfile(t *testing.T) {
    env := newApplyEnvironment(t, runningProfile("profile-old", 2), validDraft("profile-new", 5))
    env.Core.FailNextStartFor("profile-new")
    _, err := env.Coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: "profile-new", Revision: 5})
    if !errors.Is(err, application.ErrStartFailed) { t.Fatalf("err=%v", err) }
    if env.ConfigMarker() != "old" || env.CoreStatus().CurrentProfileID != "profile-old" || !env.CoreStatus().Running() { t.Fatalf("recovery failed") }
}

func TestApplyRechecksExternalDriftImmediatelyBeforeSwap(t *testing.T) {
    env := newApplyEnvironment(t, runningProfile("profile-old", 2), validDraft("profile-new", 5))
    env.AfterCheck(func() { env.ExternallyRewriteConfig("changed-after-initial-check") })
    _, err := env.Coordinator.Apply(context.Background(), application.ApplyRequest{ProfileID: "profile-new", Revision: 5})
    if !errors.Is(err, application.ErrExternalDrift) { t.Fatalf("err=%v", err) }
    if env.ConfigMarker() != "changed-after-initial-check" || env.SwapCount() != 0 { t.Fatalf("external config was overwritten") }
}
```

- [ ] **Step 2: 运行应用测试并确认失败**

Run: `go test ./internal/application -run 'TestApply' -v`

Expected: FAIL，Coordinator 尚不存在。

- [ ] **Step 3: 实现阶段化事务和失败恢复**

```go
func (c *Coordinator) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
    operation, releaseOperation, err := c.operations.Enter(ctx, core.OperationApply)
    if err != nil { return ApplyResult{}, err }
    defer releaseOperation()
    operation.SetStage("snapshot")
    draft, _, err := c.profiles.GetDraft(ctx, request.ProfileID)
    if err != nil { return ApplyResult{}, err }
    if draft.Revision != request.Revision { return ApplyResult{}, profile.RevisionConflict{LatestRevision: draft.Revision} }
    snapshot, releaseRules, err := c.rules.SnapshotForBindings(ctx, draft.Bindings)
    if err != nil { return ApplyResult{}, err }
    defer releaseRules()
    previous := c.core.Status(ctx)
    if err := c.confirmNoUnapprovedDrift(request.ConfirmedConfigDigest); err != nil { return ApplyResult{}, err }
    operation.SetStage("stop_core")
    if previous.Running() {
        if err := c.core.Stop(ctx); err != nil { return ApplyResult{}, wrapStage("stop_core", err) }
    }
    return c.applyStopped(ctx, operation, request, draft, snapshot, previous)
}
```

`applyStopped` 依次执行语义检查、生成 staging、实际 check、再次计算正式 config 摘要、BeginDirectorySwap、启动新核心和 2 秒观察。第二次摘要必须等于事务开始时已检查/已确认的摘要；否则删除 staging、保留外部 config、按此前档案尝试重启并返回新的漂移确认值。生成/check 失败时旧 config 从未移动，并仅在此前运行时重启旧核心；新目录启动失败时先 Stop 新进程、Restore 旧目录，再按旧档案/修订启动。恢复也失败时保持停止、保留可用 config、记录两个准确错误，绝不写成功元数据。

阶段 code 固定为 `snapshot`、`stop_core`、`generate_config`、`check_config`、`swap_config`、`start_core`、`commit_state`、`recover`，每次进入步骤先调用 `operation.SetStage(code)`。事务结束写审计事件 `operation=apply_profile`、profile ID、revision、综合 digest、最后 stage、耗时和 success/failed；不记录草稿、模块内容、规则 URL 或检查输出。

- [ ] **Step 4: 只在成功启动后提交档案和核心状态**

`profile.MarkApplied` 写 `appliedRevision`、生成+规则综合摘要、UTC 时间、`needsApply=false` 和最近检查结果。`core.Store` 写当前档案、期望运行、配置目录摘要、版本和清空最近启动错误。若状态写入失败，视为事务失败并尝试恢复旧目录/核心，避免配置与持久状态悄然分叉。

- [ ] **Step 5: 暴露应用 API、漂移确认和准确阶段**

请求固定为：

```json
{
  "revision": 5,
  "confirmedConfigDigest": ""
}
```

外部漂移返回 `409 config_external_drift`、`stage:"snapshot"` 和 `details.confirmationDigest`；用户确认后原样重提该 digest，服务端再次计算相同才继续。HTTP 中断不取消已进入 `stop_core` 的事务：handler 用具有 2 分钟上限、继承认证审计值但不随客户端断开的 context 完成事务，客户端恢复后从 `GET /core` 和档案列表读取真实状态。

- [ ] **Step 6: 接通当前运行档案删除约束**

普通档案仍直接删除草稿。当前运行档案只在请求的 `confirmName` 精确匹配且 `stopCurrent=true` 时进入全局锁：停止核心、原子移走固定 config 到本事务临时目录、清除 core 当前/期望状态、删除草稿与元数据、删除临时目录。任一步失败恢复尚可恢复的 config 和核心。未明确停止返回 409；也可先应用其他档案后普通删除。

- [ ] **Step 7: 运行应用故障矩阵与 API 测试**

Run: `go test -race ./internal/application ./internal/profile ./internal/core ./internal/httpapi -v`

Expected: PASS；停止失败不写 staging；生成/check 失败不换目录；启动失败恢复旧目录/档案；恢复失败明确停止；过期修订返回 409；成功应用后仅有一个 config 和一个进程。

- [ ] **Step 8: 提交完整应用事务**

```bash
git add internal/application internal/profile internal/core internal/httpapi
git commit -m "实现配置应用与失败恢复事务"
```

### Task 5: 解析官方稳定版、校验摘要并安全提取程序

**Files:**
- Create: `internal/updater/model.go`
- Create: `internal/updater/releases.go`
- Create: `internal/updater/releases_test.go`
- Create: `internal/updater/archive.go`
- Create: `internal/updater/archive_test.go`
- Create: `internal/updater/downloader.go`
- Create: `internal/updater/downloader_test.go`

**Interfaces:**
- Consumes: 官方 GitHub Release JSON、当前 `runtime.GOARCH`、官方 checksums 资产、`.tar.gz` 资产和固定临时目录。
- Produces: `ReleaseClient.LatestStable(ctx) (Release, error)` 与 `Fetcher.Prepare(ctx, release) (PreparedBinary, cleanup func, error)`；此任务不停止或替换当前核心。

- [ ] **Step 1: 写稳定版选择、架构映射、摘要和穿越/链接拒绝测试**

```go
func TestLatestStableSelectsCurrentLinuxAssetAndChecksums(t *testing.T) {
    releaseJSON := `{"tag_name":"v1.12.3","draft":false,"prerelease":false,"assets":[{"name":"sing-box-1.12.3-linux-amd64.tar.gz","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-linux-amd64.tar.gz"},{"name":"sing-box-1.12.3-checksums.txt","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-checksums.txt"}]}`
    release, err := parseRelease([]byte(releaseJSON), "amd64")
    if err != nil { t.Fatal(err) }
    if release.Version != "1.12.3" || release.Asset.Name != "sing-box-1.12.3-linux-amd64.tar.gz" { t.Fatalf("release=%+v", release) }
}

func TestExtractRejectsTraversalAndLinks(t *testing.T) {
    for _, entry := range []tarEntry{{Name: "../../sing-box", Mode: 0o755}, {Name: "/tmp/sing-box", Mode: 0o755}, {Name: "sing-box-1.12.3-linux-amd64/sing-box", Mode: 0o755, Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"}} {
        archive := makeTarGz(t, entry)
        if _, err := updater.ExtractBinary(archive, t.TempDir(), "sing-box-1.12.3-linux-amd64/sing-box"); err == nil { t.Fatalf("accepted entry=%+v", entry) }
    }
}

func TestChecksumMustMatchExactAssetName(t *testing.T) {
    checksums := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  other.tar.gz\n")
    if _, err := updater.FindChecksum(checksums, "sing-box-1.12.3-linux-amd64.tar.gz"); err == nil { t.Fatal("wrong checksum accepted") }
}
```

- [ ] **Step 2: 运行 updater 解析测试并确认失败**

Run: `go test ./internal/updater -run 'TestLatestStable|TestExtract|TestChecksum' -v`

Expected: FAIL，updater 包尚不存在。

- [ ] **Step 3: 实现官方元数据和架构资产选择**

元数据 URL 固定为 `https://api.github.com/repos/SagerNet/sing-box/releases/latest`；拒绝 draft、prerelease 和不以 `v` 开头的版本。架构映射固定支持 `amd64→amd64`、`arm64→arm64`、`386→386`、`arm→armv7`，其他返回 `unsupported_architecture`。要求唯一的 `sing-box-<version>-linux-<arch>.tar.gz` 和唯一的 `sing-box-<version>-checksums.txt`。

- [ ] **Step 4: 实现有界 HTTPS 下载和官方 host allowlist**

只允许 `api.github.com`、`github.com`、`objects.githubusercontent.com`、`release-assets.githubusercontent.com` 的 HTTPS URL，每次最多 5 次重定向并重新检查；连接 10 秒、总下载 5 分钟、单资产 256 MiB、checksums 4 MiB。下载写 `data/bin` 同文件系统的随机临时目录，不使用系统 shell或环境代理。

- [ ] **Step 5: 实现摘要校验和安全 tar.gz 提取**

只接受规范相对路径且唯一匹配 `sing-box-<version>-linux-<arch>/sing-box` 的普通文件；拒绝绝对路径、`..`、重复目标、目录外文件、符号链接、硬链接、设备和超过 256 MiB 的展开内容。先比对压缩包 SHA-256，再写目标 `0600`，完成后改为 `0700`。

```go
type PreparedBinary struct {
    Path          string
    Version       string
    ArchiveDigest string
    BinaryDigest  string
}
```

- [ ] **Step 6: 运行网络、摘要和压缩包安全测试**

Run: `go test -race ./internal/updater -v`

Expected: PASS；预发布、错误资产、错误 checksum、host 跳转、超限、路径穿越和链接均失败；失败后没有可执行临时程序残留。

- [ ] **Step 7: 提交核心下载准备层**

```bash
git add internal/updater
git commit -m "实现官方核心下载与安全校验"
```

### Task 6: 实现手动安装/更新事务和状态 API

**Files:**
- Create: `internal/updater/service.go`
- Create: `internal/updater/service_test.go`
- Create: `internal/updater/handler.go`
- Create: `internal/updater/handler_test.go`
- Modify: `internal/core/handler.go`
- Modify: `internal/httpapi/router.go`
- Modify: `internal/app/server.go`

**Interfaces:**
- Consumes: `ReleaseClient`、`Fetcher`、`core.OperationLock/Manager/Store`、`configgen.Checker` 和固定 layout。
- Produces: `GET /api/v1/core/releases/latest`、`POST /api/v1/core/update`、`GET /api/v1/core/update`；`Updater.Start(ctx, confirmation UpdateConfirmation) (operationID string, error)`。

- [ ] **Step 1: 写首次安装、运行中更新、预检失败和启动失败回退测试**

```go
func TestFirstInstallCreatesNoBackupAndRecordsRealVersion(t *testing.T) {
    env := newUpdateEnvironment(t, coreNotInstalled())
    env.PreparedBinary.VersionOutput = "sing-box version 1.12.3\n"
    status := env.RunConfirmedUpdate("1.12.3")
    if status.Stage != "completed" || env.CoreState().InstalledVersion != "1.12.3" { t.Fatalf("status=%+v state=%+v", status, env.CoreState()) }
    if _, err := os.Stat(env.Layout().CoreBackup); !errors.Is(err, os.ErrNotExist) { t.Fatalf("backup exists: %v", err) }
}

func TestUpdatePrecheckFailureLeavesRunningCoreUntouched(t *testing.T) {
    env := newUpdateEnvironment(t, runningCore("1.11.0", "profile-a"))
    env.NewBinaryCheckFails("unknown field")
    status := env.RunConfirmedUpdate("1.12.3")
    if status.Stage != "failed" || env.CoreVersion() != "1.11.0" || !env.CoreStatus().Running() || env.StopCount() != 0 { t.Fatalf("status=%+v", status) }
}

func TestUpdateStartFailureRestoresSingleBackupAndOldCore(t *testing.T) {
    env := newUpdateEnvironment(t, runningCore("1.11.0", "profile-a"))
    env.NewCoreStartFails()
    status := env.RunConfirmedUpdate("1.12.3")
    if status.Stage != "failed" || env.CoreVersion() != "1.11.0" || !env.CoreStatus().Running() { t.Fatalf("rollback failed: %+v", status) }
    assertExactlyOneBackup(t, env.Layout().BinDir)
}
```

- [ ] **Step 2: 运行更新服务测试并确认失败**

Run: `go test ./internal/updater -run 'TestFirstInstall|TestUpdate' -v`

Expected: FAIL，缺少更新事务服务。

- [ ] **Step 3: 在锁外执行版本、架构和现有配置预检**

`Start` 要求请求 `{"confirmedVersion":"1.12.3"}` 并创建随机 operation ID；后台第一阶段重新查询 latest，要求结果仍精确匹配确认版本，未确认、版本变化或已有更新返回 409/failed 状态并要求重新确认，绝不静默换成另一版本。Prepared binary 先直接运行临时目录中的 `sing-box version`，10 秒超时、双流 64 KiB，并解析准确版本；若固定 config 存在，再执行新程序 `check -C config`，失败时当前进程和程序完全不变。预检同时记录 config 目录摘要；取得全局锁后、停止核心前再次计算，若变化则对新摘要重跑 check，仍失败时不停止或替换，消除下载期间外部修改的竞态。

- [ ] **Step 4: 在全局锁内停止、单备份替换、启动与回退**

```go
func (s *Service) installPrepared(ctx context.Context, prepared PreparedBinary) error {
    operation, release, err := s.operations.Enter(ctx, core.OperationUpdate)
    if err != nil { return err }
    defer release()
    previous := s.core.Status(ctx)
    operation.SetStage("stop_core")
    if previous.Running() {
        if err := s.core.Stop(ctx); err != nil { return err }
    }
    operation.SetStage("install")
    replacement, err := beginBinaryReplacement(s.layout, prepared.Path)
    if err != nil { return s.restartPrevious(ctx, previous, err) }
    operation.SetStage("restart")
    if previous.ExpectedRunning {
        if previous.AppliedRevision == nil { return errors.New("期望运行状态缺少应用修订") }
        if err := s.core.Start(ctx, core.StartRequest{ProfileID: previous.CurrentProfileID, DraftRevision: *previous.AppliedRevision}); err != nil {
            restoreErr := replacement.Restore()
            restartErr := s.restartPrevious(ctx, previous, err)
            return errors.Join(err, restoreErr, restartErr)
        }
    }
    replacement.CommitSingleBackup()
    return s.store.RecordInstalled(prepared.Version, prepared.BinaryDigest)
}
```

替换前删除旧 `.bak`，再把当前 `sing-box` 原子改名为 `.bak`，新程序原子改名为 `sing-box` 并同步 `bin`；首次安装跳过备份。失败恢复后 `.bak` 仍只代表本次被恢复的程序，不生成版本列表。

- [ ] **Step 5: 实现可轮询更新状态与未经确认拒绝**

状态阶段固定为 `checking_release`、`downloading`、`verifying_checksum`、`verifying_binary`、`checking_config`、`waiting_for_lock`、`stopping_core`、`installing`、`restarting_core`、`completed`、`failed`，包含 operationID、targetVersion、startedAt、finishedAt、message，不含压缩包正文或配置。`GET /core` 同时显示全局 operation，页面可轮询两个端点。

更新结束另写审计事件 `operation=update_core`、内部 operation ID、新程序 digest、最终 stage、耗时和 success/failed；不记录 Release JSON、下载 URL、checksums 正文、压缩包路径或程序输出。

- [ ] **Step 6: 运行更新事务与 API 竞态测试**

Run: `go test -race ./internal/updater ./internal/core ./internal/httpapi -v`

Expected: PASS；只有确认请求启动；预检失败不停止；更新成功保留至多一个 `.bak`；失败恢复旧程序和原档案；首次下载后若无已应用 config 则保持核心停止。

- [ ] **Step 7: 提交手动核心更新**

```bash
git add internal/updater internal/core internal/httpapi internal/app
git commit -m "实现手动核心安装与更新事务"
```

### Task 7: 启动恢复、关闭语义和完整故障矩阵

**Files:**
- Create: `internal/core/recovery.go`
- Create: `internal/core/recovery_test.go`
- Modify: `internal/app/server.go`
- Modify: `internal/app/server_test.go`
- Modify: `internal/integration/testcore_test.go`
- Create: `internal/integration/application_test.go`
- Create: `internal/integration/updater_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: 持久 core/profile 状态、启动清理器、fake core 与受控 Release/rule HTTP servers。
- Produces: Web 重启后条件恢复、正常关闭停止、应用/更新各故障点集成保证；为 SPA 提供完整一期后端。

- [ ] **Step 1: 写重启恢复与正常关闭保留期望状态测试**

```go
func TestServerRestartRestoresExpectedCore(t *testing.T) {
    env := newPersistentServerEnvironment(t)
    env.ApplyProfile("profile-a", 4)
    env.Shutdown()
    if env.ProcessCount() != 0 || !env.ReadCoreState().ExpectedRunning { t.Fatalf("shutdown state=%+v", env.ReadCoreState()) }
    env.Restart()
    awaitCoreRunning(t, env.Server())
    status := env.CoreStatus()
    if status.CurrentProfileID != "profile-a" || deref(status.AppliedRevision) != 4 || env.ProcessCount() != 1 { t.Fatalf("status=%+v", status) }
}

func TestServerDoesNotRecoverWithoutAppliedProfile(t *testing.T) {
    env := newPersistentServerEnvironment(t)
    env.WriteCoreState(core.PersistentState{Version: 1, ExpectedRunning: true, CurrentProfileID: "missing"})
    env.Start()
    if env.ProcessCount() != 0 || env.CoreStatus().State != core.ProcessStartFailed { t.Fatalf("status=%+v", env.CoreStatus()) }
}
```

- [ ] **Step 2: 实现启动清理与条件恢复**

服务拿到 `app.lock` 后、打开 HTTP 前：严格读取所有核心状态；先让 rules Service 完成自己的规则文件恢复，再枚举遗留 config staging/rollback 与 core-update download 临时目录并按下面的摘要规则恢复，随后恢复规则 scheduler；若 `data/bin/sing-box` 存在，重新计算文件 SHA-256 并直接执行相对程序的 `version`（10 秒、双流各 64 KiB），用真实结果校准 `InstalledVersion/InstalledDigest`，而不盲信 `core.json`；版本执行失败则报告已安装但不可用。若 `ExpectedRunning=true`，确认当前档案存在且 `AppliedRevision` 匹配、config digest 匹配、核心可执行，再调用 Manager.Start。恢复失败不阻止登录和编辑，但记录准确 `LastStartError` 并报告 `start_failed`。

遗留目录恢复规则必须有集成测试：只允许至多一个 config staging 和一个 config rollback；有 rollback 且其 digest 等于 `core.json.configDirectoryDigest` 时，说明新目录尚未提交状态，删除当前 config（若有）并恢复 rollback；当前 config 已等于持久 digest 时删除 rollback；两者都不匹配时拒绝可写状态并报告歧义。只有 staging 且 config 存在时删除 staging；只有 staging 且 config 不存在时视为首次应用尚未切换并删除 staging。确认正式状态后再清理 core updater 拥有的下载临时目录。不得用“名称看起来像临时目录”作为唯一删除依据。

程序替换也在启动时协调：`sing-box` 缺失而 `.bak` 存在时把备份恢复为正式程序；两者都存在且正式程序 digest 等于持久 `InstalledDigest` 时保留合法单备份；备份 digest 等于持久值而正式程序不同则判定更新未提交，恢复备份；两者都不匹配时拒绝核心操作并报告歧义。首次安装崩溃留下正式程序且持久 digest 为空时，只有真实 `version` 成功后才把它登记为已安装且保持停止。每个分支都测试目录同步和至多一个 `.bak`。

- [ ] **Step 3: 实现关闭顺序**

收到 SIGINT/SIGTERM 后：停止接受新 HTTP → 取消 scheduler → 等待已有应用/更新操作最多 2 分钟 → 用 `Manager.Stop` 给核心 10 秒优雅退出并必要时强杀 → 同步状态/log → 释放 `app.lock`。正常关闭不把持久 `ExpectedRunning` 改 false，便于下一次 Web 启动恢复。

- [ ] **Step 4: 写并运行应用故障矩阵集成测试**

集成表逐一注入：停止失败、staging 写失败、check 非零、check 超时、旧 config rename 失败、新 config rename 失败、新核心观察期退出、恢复启动失败、HTTP 请求中断、外部 config 漂移、依次应用两个档案。每行断言正式目录、当前档案、PID、`needsApply`、临时目录和 fake core 调用序列。

Run: `go test -race ./internal/integration -run 'TestApplication|TestServerRestart' -v`

Expected: PASS；任何行都没有混合模块目录或两个子进程。

- [ ] **Step 5: 写并运行核心更新故障矩阵集成测试**

集成表逐一注入：Release 失败、资产缺失、摘要错误、解压穿越、version 失败、新程序 config check 失败、停止失败、安装 rename 失败、新核心启动失败、旧核心恢复失败。每行断言当前程序 digest、唯一 `.bak`、运行档案、PID 和操作状态。

Run: `go test -race ./internal/integration -run 'TestCoreUpdate' -v`

Expected: PASS；锁外失败不停止核心；锁内失败尽可能恢复且不伪造运行状态。

- [ ] **Step 6: 更新 README 的后端完成状态与边界**

README 说明：核心由 Web 作为子进程管理；应用自动包含停止/check/启动；核心仅手动确认更新；不提供 systemd、自启动方案、反向代理、TLS 或高级运行控制。记录 fake core 测试命令，不给出真实生产凭据或公网配置。

- [ ] **Step 7: 运行核心阶段全部验证**

Run:

```bash
go test -race ./...
go vet ./...
go build ./cmd/singbox-web
npm --prefix prototype run check
git status --short
```

Expected: 所有命令退出码为 0；测试结束没有残留测试子进程；工作树只显示本任务预期修改；没有仓库内 `data/`、`config/` 或日志。

- [ ] **Step 8: 提交核心操作阶段**

```bash
git add internal/core internal/app internal/integration README.md
git commit -m "完善核心恢复与事务故障测试"
```
