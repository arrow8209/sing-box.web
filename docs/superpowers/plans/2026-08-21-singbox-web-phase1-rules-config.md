# sing-box Web 一期规则资源与配置生成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付由 Web 独立管理的规则资源、可靠的服务端下载与调度、档案绑定，以及可预览并通过真实受管二进制检查的确定性多文件 sing-box 配置。

**Architecture:** 规则资源服务拥有 `rule-resources.json`、固定文件和资源级锁；配置生成器只消费不可变的档案与资源快照，不直接访问 HTTP 或进程。语义检查与二进制检查是两层明确门槛，预览和检查共用同一个生成器，确保页面所见模块与应用阶段将写入的模块一致。

**Tech Stack:** Go 1.26、标准库 `net/http`/`crypto/sha256`/`encoding/json`/`os/exec`、`httptest`、上一计划的 `paths`/`storage`/`profile`/`apierror`。

## Global Constraints

- 规则资源 URL 仅允许 HTTP 或 HTTPS；每次重定向都重新检查 URL，并限制重定向次数、连接与总超时、响应大小。
- Web 直接访问规则 URL，不解释也不使用 sing-box 的 `download_detour`。
- 每个规则资源只维护一个固定本地文件；成功替换后不保留旧规则版本，失败时不得替换现有文件。
- `source` 格式必须是可解析的规则集 JSON；`binary` 至少检查非空和大小限制。
- 同一资源的手动更新和定时更新不得并发；应用快照按稳定顺序锁定所引用资源。
- 内容摘要未变化时只更新时间，不设置 `needsApply`；内容变化时所有引用档案设置 `needsApply=true`。
- 原生 `type: "remote"` 的 `route.rule_set` 不允许直接应用；正式配置只注入指向固定本地文件的 `local` 项。
- 每个存在的顶层模块生成一个文件，不存在的模块不生成空文件；输出顺序和缩进必须确定。
- 检查使用当前 `data/bin/sing-box check -C <temporary-directory>`，默认 30 秒超时，stdout 和 stderr 分别限制为 64 KiB，不经过 shell。
- 不维护、下载或引用固定 JSON Schema，不提供合并配置或导出接口。

---

## File Responsibility Map

- `internal/rules/model.go`、`store.go`：资源 DTO、严格状态文件和固定路径。
- `internal/rules/downloader.go`：受限网络下载及内容校验，不修改档案元数据。
- `internal/rules/service.go`：CRUD、资源锁、摘要比较、原子安装、引用保护和 `needsApply` 通知。
- `internal/rules/scheduler.go`：根据持久化状态重建服务端定时任务。
- `internal/configgen/generator.go`：模块映射、本地规则注入、稳定序列化和摘要。
- `internal/configgen/semantic.go`：基础应用语义错误及模块/字段路径。
- `internal/configgen/checker.go`：实际二进制执行与有界输出。
- `internal/configgen/handler.go`：预览和完整检查 API。

### Task 1: 建立规则资源模型、固定路径、CRUD 与引用保护

**Files:**
- Modify: `internal/rules/model.go`
- Modify: `internal/rules/store.go`
- Create: `internal/rules/locks.go`
- Modify: `internal/rules/store_test.go`
- Create: `internal/rules/service.go`
- Create: `internal/rules/service_test.go`
- Create: `internal/rules/handler.go`
- Create: `internal/rules/handler_test.go`
- Modify: `internal/paths/layout.go`
- Modify: `internal/profile/service.go`
- Modify: `internal/httpapi/router.go`

**Interfaces:**
- Consumes: 基础计划已创建并用默认模板初始化的 `rules.Store`、固定 `paths.Layout.RulesFile`/`RuleSetsDir`、随机 reader、UTC clock、`audit.Logger`，以及 `rules.ProfileReferences` 查询档案绑定。
- Produces: `rules.Service.List/Create/Patch/Delete/References`；`GET/POST/PATCH/DELETE /api/v1/rule-resources`；`Layout.RuleSetPath(resourceID, extension string) (absolute string, relative string, error)`。

- [ ] **Step 1: 写安全路径、严格 DTO、引用拒绝和损坏状态测试**

```go
func TestRuleSetPathUsesIDAndFormatOnly(t *testing.T) {
    layout, _ := paths.Resolve(t.TempDir())
    absolute, relative, err := layout.RuleSetPath("0123456789abcdef0123456789abcdef", ".srs")
    if err != nil { t.Fatal(err) }
    if relative != "data/rulesets/0123456789abcdef0123456789abcdef.srs" { t.Fatalf("relative=%q", relative) }
    if absolute != filepath.Join(layout.WorkDir, filepath.FromSlash(relative)) { t.Fatalf("absolute=%q", absolute) }
    for _, bad := range []string{"../escape", "/absolute", "id/name"} {
        if _, _, err := layout.RuleSetPath(bad, ".srs"); err == nil { t.Fatalf("accepted id %q", bad) }
    }
    if _, _, err := layout.RuleSetPath("0123456789abcdef0123456789abcdef", ".exe"); err == nil { t.Fatal("accepted non-ruleset extension") }
}

func TestDeleteReferencedResourceListsProfiles(t *testing.T) {
    service := newRuleService(t, map[string][]profile.Reference{"resource-a": {{ID: "profile-a", Name: "家庭出口"}}})
    err := service.Delete(context.Background(), "resource-a")
    var referenced *rules.ReferencedError
    if !errors.As(err, &referenced) || len(referenced.Profiles) != 1 || referenced.Profiles[0].Name != "家庭出口" { t.Fatalf("err=%v", err) }
}

func TestCreateRejectsClientLocalPath(t *testing.T) {
    api := newRuleAPI(t)
    response := authenticatedJSON(t, api, http.MethodPost, "/api/v1/rule-resources", `{"name":"CN","url":"https://example.test/cn.srs","format":"binary","localPath":"/tmp/cn.srs"}`)
    if response.Code != http.StatusBadRequest { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
}
```

- [ ] **Step 2: 运行资源模型测试并确认失败**

Run: `go test ./internal/paths ./internal/rules -run 'TestRuleSetPath|TestDeleteReferenced|TestCreateRejects' -v`

Expected: FAIL，缺少资源服务、handler 或安全路径方法。

- [ ] **Step 3: 定义资源模型和严格补丁输入**

```go
type Format string

const (
    FormatSource Format = "source"
    FormatBinary Format = "binary"
)

type Resource struct {
    ID             string     `json:"id"`
    TemplateKey    string     `json:"templateKey,omitempty"`
    Name           string     `json:"name"`
    URL            string     `json:"url"`
    Format         Format     `json:"format"`
    UpdateInterval string     `json:"updateInterval"`
    Disabled       bool       `json:"disabled"`
    LocalPath      string     `json:"localPath"`
    LastAttemptAt  *time.Time `json:"lastAttemptAt"`
    LastSuccessAt  *time.Time `json:"lastSuccessAt"`
    LastDigest     string     `json:"lastDigest"`
    LastError      string     `json:"lastError"`
    NextUpdateAt   *time.Time `json:"nextUpdateAt"`
}

type CreateInput struct {
    Name           string `json:"name"`
    URL            string `json:"url"`
    Format         Format `json:"format"`
    UpdateInterval string `json:"updateInterval"`
    Disabled       bool   `json:"disabled"`
}

type PatchInput struct {
    Name           *string `json:"name"`
    URL            *string `json:"url"`
    Format         *Format `json:"format"`
    UpdateInterval *string `json:"updateInterval"`
    Disabled       *bool   `json:"disabled"`
}
```

资源 ID 固定为 16 随机字节小写 hex；`source` 对应 `.json`，`binary` 对应 `.srs`。名称为 1–80 个字符；URL 不含 userinfo、fragment，必须有 host；周期为空表示仅手动，否则使用 Go duration，范围固定为 15 分钟至 30 天。

`TemplateKey` 只用于在安装内重建默认资源映射，handler 映射到不含该字段的 `ResourceResponse`；客户端既看不到也不能提交它。

- [ ] **Step 4: 实现 CRUD、格式切换和引用保护**

`Patch` 修改格式时获取资源锁，把旧固定文件原子移到同目录事务临时名，更新 `localPath`、清空成功时间/digest 并写状态，随后删除临时文件；状态写失败则把旧文件移回。引用档案设置 `needsApply=true`，且再次成功下载前无法应用。URL 或周期变化只更新配置和调度，不伪造下载成功。`Delete` 先查询所有档案绑定，存在引用时返回 `409 rule_resource_referenced` 和 `details.profiles=[{id,name}]`；无引用时同样用临时改名包住状态更新，确保失败时不会留下“状态已删但固定文件静默孤立”的结果。

```go
type ProfileReferences interface {
    ReferencesForResource(ctx context.Context, resourceID string) ([]profile.Reference, error)
}

func (s *Service) Delete(ctx context.Context, id string) error {
    refs, err := s.profiles.ReferencesForResource(ctx, id)
    if err != nil { return err }
    if len(refs) != 0 { return &ReferencedError{ResourceID: id, Profiles: refs} }
    release, err := s.locks.Lock(ctx, id)
    if err != nil { return err }
    defer release()
    resource, err := s.store.get(id)
    if err != nil { return err }
    absolute, _, err := s.layout.RuleSetPath(id, extension(resource.Format))
    if err != nil { return err }
    transaction, err := moveAsideIfExists(absolute)
    if err != nil { return err }
    if err := s.store.remove(id); err != nil { transaction.Restore(); return err }
    return transaction.CommitDelete()
}
```

- [ ] **Step 5: 暴露 API 并验证错误结构**

Run: `go test -race ./internal/paths ./internal/profile ./internal/rules ./internal/httpapi -v`

Expected: PASS；列表的 `localPath` 始终为只读相对路径；任何路径字段和未知字段返回 400；被引用删除返回 409 并列出档案；损坏 `rule-resources.json` 阻止所有写操作并报告文件。

- [ ] **Step 6: 提交规则资源 CRUD**

```bash
git add internal/paths internal/profile internal/rules internal/httpapi
git commit -m "实现规则资源模型与引用保护"
```

### Task 2: 实现受限下载、格式检查和原子固定文件替换

**Files:**
- Create: `internal/rules/downloader.go`
- Create: `internal/rules/downloader_test.go`
- Create: `internal/rules/validator.go`
- Create: `internal/rules/validator_test.go`
- Modify: `internal/storage/atomic.go`
- Modify: `internal/storage/atomic_test.go`
- Modify: `internal/rules/service.go`
- Modify: `internal/rules/service_test.go`
- Modify: `internal/rules/handler.go`
- Create: `internal/rules/recovery.go`
- Create: `internal/rules/recovery_test.go`

**Interfaces:**
- Consumes: `Resource`、受控 `http.Client`、固定目标路径和资源级互斥锁。
- Produces: `POST /api/v1/rule-resources/{id}/update`；`Downloader.Fetch(ctx, resource) (DownloadedFile, error)`；`Service.UpdateNow(ctx, id) (UpdateResult, error)`。

- [ ] **Step 1: 写协议、重定向、超时、大小、格式和旧文件保留测试**

```go
func TestDownloaderRevalidatesEveryRedirect(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
    }))
    defer server.Close()
    downloader := rules.NewDownloader(rules.DownloadPolicyForTest(server.Client().Transport))
    _, err := downloader.Fetch(context.Background(), rules.Resource{URL: server.URL, Format: rules.FormatBinary})
    if !errors.Is(err, rules.ErrUnsupportedURL) { t.Fatalf("err=%v", err) }
}

func TestFailedUpdatePreservesCurrentFile(t *testing.T) {
    service, target := newRuleServiceWithFile(t, []byte("old-binary"))
    service.SetDownloader(failingDownloader(errors.New("response exceeded 64 MiB")))
    if _, err := service.UpdateNow(context.Background(), "resource-a"); err == nil { t.Fatal("update succeeded") }
    got, _ := os.ReadFile(target)
    if string(got) != "old-binary" { t.Fatalf("current file changed: %q", got) }
    resource := mustResource(t, service, "resource-a")
    if resource.LastError == "" || resource.LastSuccessAt == nil { t.Fatalf("status=%+v", resource) }
}

func TestValidateSourceRequiresRuleSetObject(t *testing.T) {
    for _, body := range [][]byte{nil, []byte(`[]`), []byte(`{"version":3}`), []byte(`{"version":3,"rules":[]}`)} {
        err := rules.ValidateDownloaded(rules.FormatSource, body)
        if len(body) > 0 && bytes.Contains(body, []byte(`"rules"`)) {
            if err != nil { t.Fatalf("valid body: %v", err) }
        } else if err == nil { t.Fatalf("accepted %q", body) }
    }
}
```

- [ ] **Step 2: 运行下载测试并确认失败**

Run: `go test ./internal/rules ./internal/storage -run 'TestDownloader|TestFailedUpdate|TestValidateSource' -v`

Expected: FAIL，缺少下载器或原子安装方法。

- [ ] **Step 3: 实现固定下载策略和逐跳 URL 检查**

```go
const (
    maxRedirects = 5
    maxRuleBytes = int64(64 << 20)
    connectTimeout = 10 * time.Second
    responseHeaderTimeout = 15 * time.Second
    totalDownloadTimeout = 60 * time.Second
)

func NewDownloader() *Downloader {
    transport := &http.Transport{
        Proxy: nil,
        DialContext: (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext,
        TLSHandshakeTimeout: 10 * time.Second,
        ResponseHeaderTimeout: responseHeaderTimeout,
        DisableCompression: false,
    }
    client := &http.Client{Transport: transport, Timeout: totalDownloadTimeout}
    client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
        if len(via) >= maxRedirects { return ErrTooManyRedirects }
        return validateRemoteURL(req.URL)
    }
    return &Downloader{client: client, maxBytes: maxRuleBytes}
}
```

`Fetch` 先验证初始 URL，再要求 2xx；通过 `io.LimitReader(response.Body, maxRuleBytes+1)` 读取到同一文件系统临时文件，同时计算 SHA-256，超过限制立即失败。临时文件权限 `0600`。错误响应正文不进入持久化错误，只记录状态码和通用原因。

- [ ] **Step 4: 实现 source/binary 校验与原子安装**

```go
func ValidateDownloaded(format Format, payload []byte) error {
    if len(payload) == 0 { return ErrEmptyRuleSet }
    if format == FormatBinary { return nil }
    var document struct {
        Version json.RawMessage   `json:"version"`
        Rules   []json.RawMessage `json:"rules"`
    }
    decoder := json.NewDecoder(bytes.NewReader(payload))
    if err := decoder.Decode(&document); err != nil { return fmt.Errorf("source 规则 JSON 无效: %w", err) }
    if len(document.Version) == 0 || document.Rules == nil { return errors.New("source 规则必须包含 version 和 rules") }
    if decoder.Decode(&struct{}{}) != io.EOF { return errors.New("source 规则只能包含一个 JSON 值") }
    return nil
}
```

`Service.UpdateNow` 在持有该资源锁时下载和验证；成功后调用 `storage.BeginFileReplacement(tempPath, fixedPath, 0o600)`，它把旧文件移到同目录随机 rollback、原子安装新文件并同步目录。只有资源状态 JSON 写入成功才 `Commit()` 删除 rollback；状态写入失败则 `Restore()` 旧文件（首次下载则删除新文件）。rollback 只活在本次调用，成功/恢复即删除，不形成规则历史。无当前文件的新资源下载失败时 `LastDigest` 仍为空。

服务启动在调度前调用 `RecoverRuleTransactions`：每个资源最多接受一个严格命名且非符号链接的 rollback；固定文件摘要等于状态 `LastDigest` 时删除 rollback，rollback 摘要等于状态而固定文件不同时恢复 rollback，两者都不匹配时拒绝规则写操作并报告资源 ID。只有下载 temp 而没有 rollback 时删除 temp。故障测试覆盖每个 rename/JSON write/fsync 点，证明崩溃后不会把未提交的新内容当成当前规则。

- [ ] **Step 5: 更新状态并仅在摘要变化时标记档案**

```go
changed := resource.LastDigest != downloaded.Digest
resource.LastAttemptAt = ptr(now)
resource.LastSuccessAt = ptr(now)
resource.LastDigest = downloaded.Digest
resource.LastError = ""
if err := s.store.update(resource); err != nil { return UpdateResult{}, err }
if changed {
    if err := s.profiles.MarkResourcesChanged(ctx, []string{resource.ID}); err != nil { return UpdateResult{}, err }
}
return UpdateResult{Changed: changed, Digest: downloaded.Digest, Message: updateMessage(changed)}, nil
```

首次成功下载计为变化。`UpdateNow` 在入口记录 UTC start，返回前调用 `audit.Logger.Record(audit.Event{Operation:"update_rule_resource", InternalID:resource.ID, Digest:downloaded.Digest, Stage:"completed", DurationMS:s.clock().Sub(start).Milliseconds(), Result:"success"})`；失败写同结构但 digest 为空、stage 为实际失败阶段、result 为 `failed`。具体错误保存在资源状态，不把 URL 或响应正文复制进审计输出。API 在变化时返回固定中文提示 `规则资源已更新，请点击“应用”使 sing-box 使用新规则。`，未变化时返回 `规则资源内容未变化。`。

- [ ] **Step 6: 运行下载层竞态和故障测试**

Run: `go test -race ./internal/storage ./internal/rules -v`

Expected: PASS；并发两次更新同一资源的服务端最大并发为 1；协议、重定向、超时、超限、无效 JSON 均保留旧文件；临时文件在成功和失败后都被清理。

- [ ] **Step 7: 提交受限规则下载**

```bash
git add internal/storage internal/rules
git commit -m "实现规则资源安全下载"
```

### Task 3: 实现服务端调度、锁定快照和 `needsApply` 传播

**Files:**
- Modify: `internal/rules/locks.go`
- Create: `internal/rules/locks_test.go`
- Create: `internal/rules/scheduler.go`
- Create: `internal/rules/scheduler_test.go`
- Modify: `internal/rules/service.go`
- Modify: `internal/profile/service.go`
- Modify: `internal/profile/service_test.go`
- Modify: `internal/app/server.go`

**Interfaces:**
- Consumes: `rules.Service`、可注入 clock/timer、档案绑定查询。
- Produces: `Scheduler.Run(ctx)`；`Service.SnapshotForBindings(ctx, []profile.RuleBinding) (rules.Snapshot, release func(), error)`；`profile.Service.MarkResourcesChanged`。

- [ ] **Step 1: 写调度重建、禁用、失败重排和死锁顺序测试**

```go
func TestSchedulerRebuildsFromPersistentNextUpdate(t *testing.T) {
    clock := newFakeClock(time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC))
    service := persistedRuleService(t, rules.Resource{ID: idA, UpdateInterval: "1h", NextUpdateAt: ptr(clock.Now().Add(time.Minute))})
    scheduler := rules.NewScheduler(service, clock)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go scheduler.Run(ctx)
    clock.Advance(time.Minute)
    awaitUpdateCount(t, service, idA, 1)
}

func TestSnapshotLocksResourcesInIDOrder(t *testing.T) {
    locker := &recordingResourceLocker{}
    service := newRuleServiceWithResourcesAndLocker(t, locker, idB, idA)
    _, release, err := service.SnapshotForBindings(context.Background(), []profile.RuleBinding{{ResourceID: idB, Enabled: true}, {ResourceID: idA, Enabled: true}})
    if err != nil { t.Fatal(err) }
    defer release()
    if !slices.Equal(locker.AcquiredIDs(), []string{idA, idB}) { t.Fatalf("order=%v", locker.AcquiredIDs()) }
}
```

- [ ] **Step 2: 运行调度测试并确认失败**

Run: `go test ./internal/rules ./internal/profile -run 'TestScheduler|TestSnapshot|TestMarkResources' -v`

Expected: FAIL，缺少 scheduler、snapshot 或档案状态传播。

- [ ] **Step 3: 实现可取消调度循环**

```go
func (s *Scheduler) Run(ctx context.Context) error {
    for {
        next, ok := s.service.NextScheduled()
        if !ok {
            if err := s.clock.Wait(ctx, time.Minute); err != nil { return context.Cause(ctx) }
            continue
        }
        if err := s.clock.WaitUntil(ctx, next.At); err != nil { return context.Cause(ctx) }
        _, err := s.service.UpdateNow(ctx, next.ResourceID)
        s.service.RecordScheduleResult(ctx, next.ResourceID, err)
    }
}
```

启动时：启用且周期非空、`NextUpdateAt` 在未来的资源沿用该时间；缺少计划或时间已过的资源立即排到当前时间；禁用或手动资源的 `NextUpdateAt` 为 null。成功或失败后都按“本次计划时间 + 周期”计算下一次，若已落后则从当前时间加周期，避免快速补跑。

- [ ] **Step 4: 实现稳定资源锁顺序与不可变快照**

```go
type Snapshot struct {
    Resources map[string]Resource
    Digests   map[string]string
}

type ResourceLocker interface {
    Lock(ctx context.Context, resourceID string) (release func(), err error)
}

func (s *Service) SnapshotForBindings(ctx context.Context, bindings []profile.RuleBinding) (Snapshot, func(), error) {
    ids := enabledUniqueIDs(bindings)
    slices.Sort(ids)
    releases := make([]func(), 0, len(ids))
    for _, id := range ids {
        release, err := s.locks.Lock(ctx, id)
        if err != nil { releaseReverse(releases); return Snapshot{}, func(){}, err }
        releases = append(releases, release)
    }
    snapshot, err := s.snapshotLocked(ids)
    if err != nil { releaseReverse(releases); return Snapshot{}, func(){}, err }
    return snapshot, func() { releaseReverse(releases) }, nil
}
```

快照用 `Lstat` 确认每个启用资源是固定目录中的普通非符号链接文件，存在成功状态且当前摘要等于状态摘要；缺失返回 `rule_resource_not_downloaded`，符号链接或外部文件漂移返回 `rule_resource_file_drift`。释放函数可安全调用一次。

- [ ] **Step 5: 实现档案脏状态传播和应用清除语义**

`MarkResourcesChanged` 只更新绑定了任一给定资源且 `enabled=true` 的档案；`MarkApplied` 记录本次快照 digest map，只有当前 digest 与应用结果包含的 digest 相同才清除 `needsApply`。这样应用过程中发生但被资源锁阻塞的更新会在锁释放后重新标脏。

`Meta.AppliedResourceDigests` 保存最近成功应用快照中的逐资源摘要。服务启动和规则状态写入后调用 `profile.Service.ReconcileResourceDigests(current map[string]string)`：任一启用绑定缺文件、缺摘要或摘要不同都设置 `needsApply=true`。因此规则文件与状态已经成功原子替换、但随后档案元数据写入失败时，下次重试或 Web 重启仍会补齐脏状态，不会把新规则误报为已应用。

- [ ] **Step 6: 接入服务启动与关闭并验证不依赖浏览器**

Run: `go test -race ./internal/rules ./internal/profile ./internal/app -v`

Expected: PASS；无 HTTP 请求时 fake clock 到期仍执行更新；取消 server context 后 scheduler 在 1 秒内退出；更新失败保留文件、记录准确错误并给出下一次计划。

- [ ] **Step 7: 提交规则调度与快照锁**

```bash
git add internal/rules internal/profile internal/app
git commit -m "实现规则调度与应用快照"
```

### Task 4: 生成确定性多文件配置和本地规则引用

**Files:**
- Create: `internal/configgen/model.go`
- Create: `internal/configgen/generator.go`
- Create: `internal/configgen/generator_test.go`
- Create: `internal/configgen/writer.go`
- Create: `internal/configgen/writer_test.go`

**Interfaces:**
- Consumes: `profile.Draft` 与持锁的 `rules.Snapshot`。
- Produces: `Generator.Generate(ctx, draft, snapshot) (configgen.Output, error)` 与 `Generator.WriteDirectory(ctx, output, dir) error`；`Output.Files` 按文件名排序。

- [ ] **Step 1: 写完整文件映射、无损字段、本地规则和稳定摘要测试**

```go
func TestGenerateWritesOneDeterministicFilePerPresentModule(t *testing.T) {
    draft := profile.Draft{Config: map[string]json.RawMessage{
        "log": json.RawMessage(`{"level":"info","future_log_field":{"enabled":true}}`),
        "outbounds": json.RawMessage(`[{"type":"future-protocol","tag":"node-a","future":{"x":1}}]`),
        "route": json.RawMessage(`{"rules":[{"rule_set":"GeoSite-CN","outbound":"node-a"}]}`),
    }, Bindings: []profile.RuleBinding{{ResourceID: idA, Tag: "GeoSite-CN", Enabled: true}}}
    snapshot := rules.Snapshot{Resources: map[string]rules.Resource{idA: {ID: idA, Format: rules.FormatBinary, LocalPath: "data/rulesets/"+idA+".srs"}}, Digests: map[string]string{idA: strings.Repeat("a", 64)}}
    first, err := configgen.NewGenerator().Generate(context.Background(), draft, snapshot)
    if err != nil { t.Fatal(err) }
    second, _ := configgen.NewGenerator().Generate(context.Background(), draft, snapshot)
    if first.Digest != second.Digest || !reflect.DeepEqual(first.Files, second.Files) { t.Fatal("generation is not deterministic") }
    assertFileJSON(t, first.Files, "10-log.json", `{"log":{"future_log_field":{"enabled":true},"level":"info"}}`)
    assertFileContains(t, first.Files, "70-outbounds.json", `"future":{"x":1}`)
    assertFileContains(t, first.Files, "80-route.json", `"type":"local"`)
    assertFileContains(t, first.Files, "80-route.json", `"path":"data/rulesets/`+idA+`.srs"`)
    assertNoFileContains(t, first.Files, "https://", "download_detour", "update_interval")
}

func TestGenerateOmitsMissingAndSchemaModules(t *testing.T) {
    output, _ := configgen.NewGenerator().Generate(context.Background(), profile.Draft{Config: map[string]json.RawMessage{"$schema": json.RawMessage(`"https://example.test/schema"`), "dns": json.RawMessage(`null`)}}, rules.Snapshot{})
    if len(output.Files) != 0 { t.Fatalf("files=%v", output.Files) }
}

func TestGenerateCanonicalizesObjectKeyOrderWithoutLosingNumbers(t *testing.T) {
    first := draftWithModule("log", `{"future":{"z":1,"a":9007199254740993},"level":"info"}`)
    second := draftWithModule("log", `{"level":"info","future":{"a":9007199254740993,"z":1}}`)
    outputA, _ := configgen.NewGenerator().Generate(context.Background(), first, rules.Snapshot{})
    outputB, _ := configgen.NewGenerator().Generate(context.Background(), second, rules.Snapshot{})
    if !bytes.Equal(outputA.Files[0].Content, outputB.Files[0].Content) { t.Fatalf("non-canonical output\n%s\n%s", outputA.Files[0].Content, outputB.Files[0].Content) }
    if !bytes.Contains(outputA.Files[0].Content, []byte("9007199254740993")) { t.Fatal("integer precision changed") }
}
```

- [ ] **Step 2: 运行生成测试并确认失败**

Run: `go test ./internal/configgen -run 'TestGenerate' -v`

Expected: FAIL，缺少生成器。

- [ ] **Step 3: 定义固定模块表和输出 DTO**

```go
var modules = []Module{
    {"log", "10-log.json"}, {"dns", "20-dns.json"}, {"ntp", "30-ntp.json"},
    {"certificate", "40-certificate.json"}, {"certificate_providers", "41-certificate-providers.json"},
    {"http_clients", "42-http-clients.json"}, {"network_namespaces", "43-network-namespaces.json"},
    {"endpoints", "50-endpoints.json"}, {"inbounds", "60-inbounds.json"},
    {"outbounds", "70-outbounds.json"}, {"route", "80-route.json"},
    {"services", "90-services.json"}, {"experimental", "99-experimental.json"},
}

type File struct {
    Name    string          `json:"name"`
    Content json.RawMessage `json:"content"`
}

type Output struct {
    Files           []File            `json:"files"`
    Digest          string            `json:"digest"`
    ResourceDigests map[string]string `json:"resourceDigests"`
}
```

- [ ] **Step 4: 实现模块封装、规则注入和摘要**

```go
func (g *Generator) Generate(ctx context.Context, draft profile.Draft, snapshot rules.Snapshot) (Output, error) {
    config := cloneRawMap(draft.Config)
    route, err := injectLocalRuleSets(config["route"], draft.Bindings, snapshot)
    if err != nil { return Output{}, err }
    if route != nil { config["route"] = route }
    files := make([]File, 0, len(modules))
    for _, module := range modules {
        value, exists := config[module.Key]
        if !exists || isJSONNull(value) { continue }
        content, err := stableModuleJSON(module.Key, value)
        if err != nil { return Output{}, apierror.Field("invalid_module_json", "模块 JSON 无效", "generate", module.Key, err) }
        files = append(files, File{Name: module.FileName, Content: content})
    }
    digests := maps.Clone(snapshot.Digests)
    return Output{Files: files, Digest: digestOutput(files, digests), ResourceDigests: digests}, nil
}
```

`injectLocalRuleSets` 先保留草稿中的 `local` 与 `inline`，遇到 `remote` 立即返回字段路径 `route.rule_set[i].type`；再按绑定数组顺序追加 `{type:"local",tag,format,path}`。重复 tag、缺失资源或路径不匹配快照均失败。`stableModuleJSON` 使用 `json.Decoder.UseNumber` 解码为对象树，再用 `json.MarshalIndent` 进行 map key 字典序规范化、两空格缩进和结尾换行；这样不会把大整数转成 float64。摘要按文件名、NUL、内容、资源 ID、资源 digest 的排序序列计算 SHA-256。

- [ ] **Step 5: 实现只写新目录的 writer**

`WriteDirectory` 要求目标不存在，创建权限 `0700`，逐个以 `0600` 写入文件并同步目录；文件名必须属于固定模块表，防止调用者注入路径。任一失败删除该新目录，不接触正式 `config`。

- [ ] **Step 6: 运行生成器与 writer 测试**

Run: `go test -race ./internal/configgen -v`

Expected: PASS；未知出站字段逐值保留；相同输入字节完全一致；规则 URL、下载 detour 和周期不出现在任何文件；缺失模块无空文件。

- [ ] **Step 7: 提交配置生成器**

```bash
git add internal/configgen
git commit -m "生成确定性多文件配置"
```

### Task 5: 实现 JSON 缓冲与基础应用语义检查

**Files:**
- Create: `internal/configgen/semantic.go`
- Create: `internal/configgen/semantic_test.go`
- Modify: `internal/profile/model.go`

**Interfaces:**
- Consumes: 档案所有模块缓冲、最近有效 `Config`、规则绑定和资源快照。
- Produces: `configgen.ValidateSemantic(draft, snapshot) []apierror.Error`，错误按 `fieldPath`、`code` 稳定排序。

- [ ] **Step 1: 写 JSON、tag、出站、DNS、规则集和条件字段测试表**

```go
func TestValidateSemanticReportsStablePaths(t *testing.T) {
    tests := []struct {
        name string
        draft profile.Draft
        code string
        path string
    }{
        {"invalid buffer", draftWithBuffer("outbounds", `[{`), "invalid_json_buffer", "outbounds"},
        {"duplicate outbound", draftWithModule("outbounds", `[{"type":"direct","tag":"x"},{"type":"block","tag":"x"}]`), "duplicate_tag", "outbounds[1].tag"},
        {"missing route outbound", draftWithModules(map[string]string{"outbounds": `[{"type":"direct","tag":"direct"}]`, "route": `{"final":"missing"}`}), "unknown_outbound", "route.final"},
        {"missing dns server", draftWithModule("dns", `{"servers":[{"type":"udp","tag":"dns-a","server":"1.1.1.1"}],"final":"dns-missing"}`), "unknown_dns_server", "dns.final"},
        {"remote rule set", draftWithModule("route", `{"rule_set":[{"type":"remote","tag":"remote-a","url":"https://example.test/a.srs"}]}`), "remote_rule_set_forbidden", "route.rule_set[0].type"},
        {"missing binding", draftWithBinding(profile.RuleBinding{ResourceID: idA, Tag: "GeoSite-CN", Enabled: true}), "unknown_rule_resource", "bindings[0].resourceId"},
    }
    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            errors := configgen.ValidateSemantic(test.draft, rules.Snapshot{})
            assertError(t, errors, test.code, test.path)
        })
    }
}
```

- [ ] **Step 2: 运行语义测试并确认失败**

Run: `go test ./internal/configgen -run TestValidateSemantic -v`

Expected: FAIL，缺少 `ValidateSemantic`。

- [ ] **Step 3: 实现缓冲解析和顶层容器检查**

对 `draft.Buffers` 的每个键分别解析；允许键只限 13 个生成模块和 `$schema`。对象模块为 `log/dns/ntp/certificate/route/experimental`，数组模块为其余受管模块；空文本表示删除模块。文本无效时保留草稿中的最近有效 `Config` 供继续编辑，但返回 `invalid_json_buffer` 并禁止预览、检查和应用。有效缓冲已由 profile 保存服务以原始 `json.RawMessage` 重建 Config，因此语义检查和生成器读取的是同一事实。

```go
func ValidateSemantic(draft profile.Draft, snapshot rules.Snapshot) []apierror.Error {
    collector := newErrorCollector()
    parsed := validateBuffers(collector, draft.Buffers)
    validateContainers(collector, parsed)
    tags := collectTags(collector, parsed)
    validateOutboundReferences(collector, parsed, tags.Outbounds)
    validateDNSReferences(collector, parsed, tags.DNSServers)
    validateRuleSetReferences(collector, parsed, draft.Bindings, snapshot)
    validateRequiredFields(collector, parsed)
    return collector.Sorted()
}
```

- [ ] **Step 4: 实现明确的基础引用规则**

检查下列路径，不对未知字段猜测 Schema：

- `inbounds[i].tag`、`outbounds[i].tag`、`endpoints[i].tag`、`services[i].tag`、`dns.servers[i].tag` 各自非空且集合内唯一。
- `route.final`、`route.rules[i].outbound`、`outbounds[i].detour`、`outbounds[i].outbounds[j]`、`inbounds[i].detour` 必须引用已知出站 tag。
- `dns.final`、`dns.rules[i].server` 与对象形态 `dns.rules[i].action.server` 必须引用已知 DNS server tag。
- `route.rules[i].rule_set` 的字符串或字符串数组必须引用草稿本地/inline tag 或启用绑定 tag。
- 每个启用绑定要求非空且唯一 tag、存在资源、已有成功文件；绑定 tag 不得与草稿规则集 tag 重复。
- `type` 和 `tag` 是入站、出站、端点、服务、DNS server 和规则集项目的基础必填字段；URL/协议细节交给当前 sing-box 二进制检查。

- [ ] **Step 5: 运行语义全表和未知字段回归测试**

Run: `go test -race ./internal/configgen -run 'TestValidateSemantic|TestUnknownFields' -v`

Expected: PASS；每个错误都有稳定 code、stage=`semantic_check` 和字段路径；未知字段不触发错误也不被修改；所有错误一次返回，便于前端定位。

- [ ] **Step 6: 提交语义检查器**

```bash
git add internal/configgen internal/profile/model.go
git commit -m "实现配置基础语义检查"
```

### Task 6: 执行实际受管二进制检查并暴露预览/验证 API

**Files:**
- Create: `internal/configgen/checker.go`
- Create: `internal/configgen/checker_test.go`
- Create: `internal/configgen/handler.go`
- Create: `internal/configgen/handler_test.go`
- Create: `internal/configgen/gate.go`
- Modify: `internal/httpapi/router.go`
- Modify: `internal/paths/layout.go`

**Interfaces:**
- Consumes: 当前档案修订、规则快照、`data/bin/sing-box`、临时目录创建器和 `configgen.OperationGate`。
- Produces: `POST /api/v1/profiles/{id}/preview`、`POST /api/v1/profiles/{id}/validate`；`Checker.Check(ctx, binaryPath, configDir) (CheckResult, error)`。

- [ ] **Step 1: 写命令参数、超时、分流限长和不写正式目录测试**

```go
func TestCheckerUsesArgumentArrayAndBoundsEachStream(t *testing.T) {
    runner := &recordingRunner{result: configgen.ProcessResult{Stdout: bytes.Repeat([]byte("o"), 70<<10), Stderr: bytes.Repeat([]byte("e"), 70<<10)}}
    checker := configgen.NewChecker("/work", runner, 30*time.Second, 64<<10)
    result, err := checker.Check(context.Background(), "/work/data/bin/sing-box", "/work/config.staging-a1")
    if err != nil { t.Fatal(err) }
    if !reflect.DeepEqual(runner.Command, []string{"/work/data/bin/sing-box", "check", "-C", "/work/config.staging-a1"}) { t.Fatalf("command=%v", runner.Command) }
    if runner.Dir != "/work" { t.Fatalf("work dir=%q", runner.Dir) }
    if len(result.Stdout) != 64<<10 || len(result.Stderr) != 64<<10 || !result.OutputTruncated { t.Fatalf("result=%+v", result) }
}

func TestValidateDoesNotStopCoreOrWriteConfig(t *testing.T) {
    api, layout, coreSpy := newValidationAPI(t)
    response := validateProfile(t, api, profileID, 3)
    if response.Code != http.StatusOK { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
    if coreSpy.StopCalls != 0 { t.Fatalf("core stopped %d times", coreSpy.StopCalls) }
    if _, err := os.Stat(layout.ConfigDir); !errors.Is(err, os.ErrNotExist) { t.Fatalf("formal config exists: %v", err) }
    assertNoTransactionDirs(t, layout.WorkDir)
}
```

- [ ] **Step 2: 运行 checker/handler 测试并确认失败**

Run: `go test ./internal/configgen -run 'TestChecker|TestValidate|TestPreview' -v`

Expected: FAIL，缺少 checker 或 HTTP handler。

- [ ] **Step 3: 实现无 shell runner 和双流有界缓冲**

```go
type CommandSpec struct {
    Args []string
    Dir  string
}

func (r ExecRunner) Run(ctx context.Context, spec CommandSpec, timeout time.Duration, limit int) (ProcessResult, error) {
    runCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    cmd := exec.CommandContext(runCtx, spec.Args[0], spec.Args[1:]...)
    cmd.Dir = spec.Dir
    cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
    stdout := newLimitedBuffer(limit)
    stderr := newLimitedBuffer(limit)
    cmd.Stdout, cmd.Stderr = stdout, stderr
    err := cmd.Run()
    if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
        return ProcessResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), TimedOut: true, OutputTruncated: stdout.Truncated() || stderr.Truncated()}, ErrCheckTimeout
    }
    return ProcessResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitCode(err), OutputTruncated: stdout.Truncated() || stderr.Truncated()}, err
}
```

Checker 构造时固定持有 `paths.Layout.WorkDir`，每次调用 Runner 都传 `CommandSpec{Args: []string{binaryPath,"check","-C",configDir}, Dir: workDir}`，确保生成配置中的 `data/rulesets/...` 总是相对同一 Web 工作目录解析。缺少二进制或非可执行文件返回 `core_not_installed`；非零退出返回 `singbox_check_failed`，响应保留原始 stdout/stderr（仅截长）和真实已安装版本字段，不做脱敏。

- [ ] **Step 4: 实现预览与验证协调器**

`preview` 要求请求体 `{"revision":3}`，确认服务器当前修订、获取资源快照、运行语义检查并返回 `Output.Files`，不写目录、不要求核心存在。`validate` 走同样流程后创建工作目录直接子项 `config.staging-<32hex>`、写模块、执行 checker，并在 defer 中删除临时目录；不停止核心、不替换 `config`。

`validate` 在语义失败、缺核心、check 失败、超时和成功时都调用 `profile.Service.RecordValidation(profileID, revision, ValidationSummary{...})`；只有服务器当前 draft revision 仍等于被检修订时才写入，避免迟到结果覆盖新草稿的状态。preview 不改变 `lastValidation`。

```go
type OperationGate interface {
    Enter(ctx context.Context, kind string) (release func(), err error)
}

func (h *Handler) Validate(ctx context.Context, profileID string, revision uint64) (ValidateResult, error) {
    releaseOperation, err := h.gate.Enter(ctx, "validate")
    if err != nil { return ValidateResult{}, err }
    defer releaseOperation()
    draft, _, err := h.profiles.GetDraft(ctx, profileID)
    if err != nil { return ValidateResult{}, err }
    if draft.Revision != revision { return ValidateResult{}, profile.RevisionConflict{LatestRevision: draft.Revision} }
    snapshot, releaseResources, err := h.rules.SnapshotForBindings(ctx, draft.Bindings)
    if err != nil { return ValidateResult{}, err }
    defer releaseResources()
    if semantic := ValidateSemantic(draft, snapshot); len(semantic) != 0 { return ValidateResult{Errors: semantic}, ErrSemanticValidation }
    output, err := h.generator.Generate(ctx, draft, snapshot)
    if err != nil { return ValidateResult{}, err }
    return h.checkTemporary(ctx, output)
}
```

本计划注入进程内 mutex gate；Core Operations 计划以共享的 `core.OperationLock` 替换而不改 handler 接口。

- [ ] **Step 5: 运行预览、验证、超时和清理测试**

Run: `go test -race ./internal/configgen ./internal/httpapi -v`

Expected: PASS；预览不需要二进制；验证缺核心返回明确错误；失败/超时后正式目录不变且 staging 被清理；响应含阶段、字段路径和未脱敏原始检查输出。

- [ ] **Step 6: 提交实际检查和 API**

```bash
git add internal/configgen internal/httpapi internal/paths
git commit -m "接入实际核心配置检查"
```

### Task 7: 完成规则与配置跨层集成验证

**Files:**
- Create: `internal/integration/rules_config_test.go`
- Create: `internal/integration/testcore_test.go`
- Modify: `internal/app/server.go`
- Modify: `internal/capabilities/service.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: 隔离临时工作目录、受控规则 HTTP server、当前 Go 测试进程的 fake sing-box helper。
- Produces: 已组合的规则 CRUD/调度/预览/验证 API，以及后续应用事务可复用的真实 `rules.Snapshot` 与 `configgen.Output`。

- [ ] **Step 1: 写从规则下载到二进制检查的集成测试**

```go
func TestRuleDownloadPreviewAndValidateFlow(t *testing.T) {
    env := newIntegrationEnvironment(t)
    env.InstallFakeCore(fakeCoreBehavior{Version: "sing-box version 1.12.0", CheckExit: 0})
    session := env.Login("correct horse battery staple")
    resourceID := env.CreateRuleResource(session, ruleServerURL(t, `{"version":3,"rules":[]}`), "source")
    update := env.UpdateRuleResource(session, resourceID)
    if !update.Changed { t.Fatal("first download not marked changed") }
    profileID, revision := env.CreateBoundProfile(session, resourceID, "GeoSite-Test")
    preview := env.Preview(session, profileID, revision)
    assertPreviewLocalRule(t, preview, "GeoSite-Test", "data/rulesets/"+resourceID+".json")
    validated := env.Validate(session, profileID, revision)
    if !validated.Valid || !slices.Equal(env.FakeCoreArguments(), []string{"check", "-C", env.LastStagingDirectory()}) { t.Fatalf("validate=%+v args=%v", validated, env.FakeCoreArguments()) }
    assertNoTransactionDirs(t, env.WorkDir())
}
```

- [ ] **Step 2: 实现可执行 fake core helper**

测试将当前测试二进制复制到 `data/bin/sing-box` 并设置专用环境 `SINGBOX_WEB_TEST_HELPER=1`；`TestMain` 在该值存在时解析 `version/check/run` 参数、把参数 JSON 追加到测试控制文件，并按控制文件返回指定 stdout/stderr/退出码。生产 runner 的环境允许列表由测试注入，不读取宿主敏感环境。

- [ ] **Step 3: 组合规则服务、scheduler 和 config handler**

`app.NewServer` 的构造顺序固定为 profile → rules → configgen；启动 HTTP 前打开并验证 `rule-resources.json`，随后启动 scheduler goroutine；关闭时先取消 scheduler，再等待 HTTP 请求，最后释放实例锁。`capabilities.ruleResources`、`configuration`、`application` 保持 true，但独立应用按钮在核心计划前仍由核心状态禁用。

- [ ] **Step 4: 更新开发验证说明**

README 增加 `go test ./internal/integration -v`，说明测试只使用 fake 核心和本地 `httptest` 服务，不访问公网、不启动真实代理、不写仓库工作目录。

- [ ] **Step 5: 运行本计划全部验证**

Run:

```bash
go test -race ./internal/rules ./internal/configgen ./internal/integration ./internal/app
go test ./...
go vet ./...
npm --prefix prototype run check
git status --short
```

Expected: 所有测试和 vet 退出码为 0；工作树只包含本任务预期修改；规则固定目录中每个资源最多一个文件，没有版本子目录；正式 `config` 未由预览或验证创建。

- [ ] **Step 6: 提交规则与配置阶段**

```bash
git add internal/integration internal/app internal/capabilities README.md
git commit -m "集成规则资源与配置检查"
```
