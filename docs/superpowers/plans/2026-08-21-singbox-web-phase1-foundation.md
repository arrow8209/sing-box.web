# sing-box Web 一期基础、认证与档案 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可启动的 Go 服务基础设施、可靠的固定目录持久化、单密码认证，以及支持 ETag 自动保存的多档案草稿 API。

**Architecture:** `internal/app` 只负责命令和生命周期组合，文件安全能力集中在 `paths` 与 `storage`，认证和档案各自拥有持久化文件与 HTTP handler。档案以顶层 `json.RawMessage` 和模块文本缓冲共同保存，既允许结构化编辑，也不会因服务端反序列化而删除未知字段。

**Tech Stack:** Go 1.26、标准库 `net/http`/`encoding/json`/`crypto`、`golang.org/x/crypto/argon2`、Go `embed`。

## Global Constraints

- 应用数据目录固定为 `data`，正式配置目录固定为 `config`；程序不提供命令行参数、环境变量、应用配置、页面或 API 来修改这些目录。
- Web 启动时获取 `data/app.lock`；无法获取时拒绝运行，禁止两个实例共享同一数据目录。
- 所有应用 JSON 文件执行文件同步、原子重命名和目录同步。
- `profiles.json`、`core.json`、`rule-resources.json` 或 `auth.json` 损坏时拒绝进入可写状态并报告文件；单个草稿损坏时隔离对应档案，不得用默认值覆盖。
- 应用只支持一个用户身份，只使用密码认证，不存在用户名或账号字段。
- 会话使用 HMAC 签名的 `HttpOnly`、`SameSite=Strict` Cookie；一期不设置 `Secure`，默认在 24 小时无操作后过期。
- 所有写请求必须同时通过 Cookie 和 CSRF Token 检查；API 不开放跨域访问。
- 新档案只接受显示名称，由服务端生成内部 ID、Secret 和 `cache_id`，不接受目录或路径字段。
- 生产运行时不能依赖 `temp` 目录。
- 本计划不创建或切换 Git 分支、worktree，不提交 `temp/`、日志、密码或生成的数据目录。

---

## File Responsibility Map

- `cmd/singbox-web/main.go` 与 `internal/app/command.go`：进程入口和两个合法命令形态。
- `internal/paths/layout.go`：固定目录、固定文件和安全派生文件名。
- `internal/storage/`：原子 JSON、目录同步、实例锁、损坏错误。
- `internal/audit/`：只允许操作元数据字段的结构化审计输出，禁止记录配置、密码或下载正文。
- `tools/templategen/` 与 `internal/templates/`：把设计输入转换为嵌入模板并为每个档案随机化敏感标识；基础 `rules.Store` 为模板资源分配安装内随机稳定 ID。
- `internal/auth/`：`auth.json`、Argon2id、HMAC 会话、CSRF、登录限速与四个认证端点。
- `internal/profile/`：`profiles.json`、单档案草稿、修订、隔离、CRUD 与草稿端点。
- `internal/httpapi/`：共享严格 JSON、错误封装、安全头、中间件和总路由。
- `internal/webui/`：可编译的嵌入目录和 SPA fallback；真正 Vue 构建在第四份计划中替换占位资源。

### Task 1: 建立 Go 命令入口与可重复构建骨架

**Files:**
- Create: `go.mod`
- Create: `cmd/singbox-web/main.go`
- Create: `internal/app/command.go`
- Create: `internal/app/command_test.go`
- Create: `internal/webui/embed.go`
- Create: `internal/webui/assets/_placeholder`
- Create: `internal/webui/fallback/index.html`
- Create: `scripts/build.sh`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: 进程的 `args`、`stdin`、`stdout`、`stderr` 与启动时当前工作目录。
- Produces: `app.Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int`；合法命令仅为 `serve` 和 `admin set-password`。

- [ ] **Step 1: 写出命令形态测试**

```go
func TestParseCommand(t *testing.T) {
    tests := []struct {
        args []string
        want Command
    }{
        {[]string{"serve"}, CommandServe},
        {[]string{"admin", "set-password"}, CommandSetPassword},
    }
    for _, test := range tests {
        got, err := ParseCommand(test.args)
        if err != nil || got != test.want {
            t.Fatalf("ParseCommand(%v) = %q, %v", test.args, got, err)
        }
    }
}

func TestParseCommandRejectsPathFlags(t *testing.T) {
    for _, args := range [][]string{
        {"serve", "--data-dir", "/tmp/data"},
        {"admin", "set-password", "--auth-file", "auth.json"},
    } {
        if _, err := ParseCommand(args); err == nil {
            t.Fatalf("ParseCommand(%v) accepted a path override", args)
        }
    }
}
```

- [ ] **Step 2: 运行测试并确认因模块和实现不存在而失败**

Run: `go test ./internal/app -run TestParseCommand -v`

Expected: FAIL，错误包含 `go.mod file not found` 或 `undefined: ParseCommand`。

- [ ] **Step 3: 添加最小模块、命令解析和入口**

```go
// go.mod
module singbox-web

go 1.26.0

require golang.org/x/crypto v0.41.0
require golang.org/x/term v0.34.0
```

```go
package app

type Command string

const (
    CommandServe       Command = "serve"
    CommandSetPassword Command = "admin-set-password"
)

func ParseCommand(args []string) (Command, error) {
    if slices.Equal(args, []string{"serve"}) {
        return CommandServe, nil
    }
    if slices.Equal(args, []string{"admin", "set-password"}) {
        return CommandSetPassword, nil
    }
    return "", fmt.Errorf("用法：singbox-web serve | singbox-web admin set-password")
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
    command, err := ParseCommand(args)
    if err != nil {
        fmt.Fprintln(stderr, err)
        return 2
    }
    return runCommand(ctx, command, stdin, stdout, stderr)
}

func runCommand(_ context.Context, command Command, _ io.Reader, _ io.Writer, stderr io.Writer) int {
    fmt.Fprintf(stderr, "命令 %s 尚未接通对应服务组件\n", command)
    return 1
}
```

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    os.Exit(app.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
```

`runCommand` 在本任务中对两个已识别但尚未组合的命令返回上述确定错误码 1；Task 4 和 Task 8 分别用密码初始化与 server runner 替换这个单一分派函数。`internal/webui/embed.go` 使用 `//go:embed all:assets all:fallback`；构建产物不存在时 handler 返回带 `<div id="app">前端尚未构建</div>` 的受版本控制 fallback，构建后优先返回 Vue index。`scripts/build.sh` 固定执行 `npm --prefix web run build` 和 `go build -o dist/singbox-web ./cmd/singbox-web`，不接收目录参数。

- [ ] **Step 4: 运行命令测试和全仓库语法测试**

Run: `go test ./internal/app -v && go test ./...`

Expected: PASS；`go test ./...` 能在尚未生成 Vue 产物时编译 `internal/webui`。

- [ ] **Step 5: 提交构建骨架**

```bash
git add go.mod cmd/singbox-web internal/app internal/webui scripts/build.sh .gitignore
git commit -m "建立 Go 服务构建骨架"
```

### Task 2: 固定工作目录、原子 JSON 与实例锁

**Files:**
- Create: `internal/paths/layout.go`
- Create: `internal/paths/layout_test.go`
- Create: `internal/storage/atomic.go`
- Create: `internal/storage/atomic_test.go`
- Create: `internal/storage/errors.go`
- Create: `internal/storage/lock_linux.go`
- Create: `internal/storage/lock_linux_test.go`
- Create: `internal/storage/cleanup.go`
- Create: `internal/storage/cleanup_test.go`
- Create: `internal/audit/event.go`
- Create: `internal/audit/event_test.go`

**Interfaces:**
- Consumes: 经 `filepath.Abs` 规范化的启动工作目录，不消费任何用户提交的目录。
- Produces: `paths.Resolve(workDir string) (paths.Layout, error)`、`storage.ReadJSON[T any](path string) (T, error)`、`storage.WriteJSON(path string, value any, perm fs.FileMode) error`、`storage.AcquireInstanceLock(path string) (*storage.InstanceLock, error)`、`audit.Logger.Record(event audit.Event)`。

- [ ] **Step 1: 写固定路径、原子写入、损坏和双实例测试**

```go
func TestResolveUsesOnlyFixedRelativePaths(t *testing.T) {
    root := t.TempDir()
    layout, err := paths.Resolve(root)
    if err != nil {
        t.Fatal(err)
    }
    assertPath(t, layout.DataDir, filepath.Join(root, "data"))
    assertPath(t, layout.ConfigDir, filepath.Join(root, "config"))
    assertPath(t, layout.CoreBinary, filepath.Join(root, "data", "bin", "sing-box"))
    assertPath(t, layout.AppLock, filepath.Join(root, "data", "app.lock"))
}

func TestResolveRejectsManagedSymlinkEscape(t *testing.T) {
    root, outside := t.TempDir(), t.TempDir()
    if err := os.Symlink(outside, filepath.Join(root, "data")); err != nil { t.Fatal(err) }
    if _, err := paths.Resolve(root); err == nil { t.Fatal("managed data symlink was accepted") }
}

func TestWriteJSONReplacesCompleteDocument(t *testing.T) {
    path := filepath.Join(t.TempDir(), "state.json")
    if err := storage.WriteJSON(path, map[string]int{"revision": 1}, 0o600); err != nil {
        t.Fatal(err)
    }
    state, err := storage.ReadJSON[map[string]int](path)
    if err != nil || state["revision"] != 1 {
        t.Fatalf("state=%v err=%v", state, err)
    }
    matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".state.json.tmp-*"))
    if len(matches) != 0 {
        t.Fatalf("temporary files remain: %v", matches)
    }
}

func TestAcquireInstanceLockRejectsSecondOwner(t *testing.T) {
    path := filepath.Join(t.TempDir(), "data", "app.lock")
    first, err := storage.AcquireInstanceLock(path)
    if err != nil {
        t.Fatal(err)
    }
    defer first.Close()
    if _, err := storage.AcquireInstanceLock(path); !errors.Is(err, storage.ErrAlreadyLocked) {
        t.Fatalf("second lock error = %v", err)
    }
}

func TestAuditLoggerAllowsMetadataButNotPayloads(t *testing.T) {
    var output bytes.Buffer
    logger := audit.NewLogger(&output)
    logger.Record(audit.Event{Operation: "save_draft", InternalID: "profile-a", Revision: 3, Digest: strings.Repeat("a", 64), Stage: "completed", DurationMS: 12, Result: "success"})
    text := output.String()
    for _, expected := range []string{"save_draft", "profile-a", `"revision":3`, `"result":"success"`} {
        if !strings.Contains(text, expected) { t.Fatalf("missing %q in %s", expected, text) }
    }
    for _, forbidden := range []string{"config", "password", "downloadBody", "url"} {
        if strings.Contains(text, forbidden) { t.Fatalf("forbidden field %q in %s", forbidden, text) }
    }
}
```

- [ ] **Step 2: 运行测试并确认实现缺失**

Run: `go test ./internal/paths ./internal/storage -v`

Expected: FAIL，错误包含 `package singbox-web/internal/paths is not in std` 或未定义函数。

- [ ] **Step 3: 实现固定 Layout 和包含性检查**

```go
type Layout struct {
    WorkDir       string
    DataDir       string
    AuthFile      string
    ProfilesFile  string
    CoreFile      string
    RulesFile     string
    DraftsDir     string
    RuleSetsDir   string
    BinDir        string
    CoreBinary    string
    CoreBackup    string
    LogsDir       string
    CoreLog       string
    AppLock       string
    ConfigDir     string
}

func Resolve(workDir string) (Layout, error) {
    root, err := filepath.Abs(workDir)
    if err != nil {
        return Layout{}, err
    }
    root, err = filepath.EvalSymlinks(root)
    if err != nil { return Layout{}, err }
    data := filepath.Join(root, "data")
    layout := Layout{
        WorkDir: root, DataDir: data,
        AuthFile: filepath.Join(data, "auth.json"),
        ProfilesFile: filepath.Join(data, "profiles.json"),
        CoreFile: filepath.Join(data, "core.json"),
        RulesFile: filepath.Join(data, "rule-resources.json"),
        DraftsDir: filepath.Join(data, "drafts"),
        RuleSetsDir: filepath.Join(data, "rulesets"),
        BinDir: filepath.Join(data, "bin"),
        CoreBinary: filepath.Join(data, "bin", "sing-box"),
        CoreBackup: filepath.Join(data, "bin", "sing-box.bak"),
        LogsDir: filepath.Join(data, "logs"),
        CoreLog: filepath.Join(data, "logs", "sing-box.log"),
        AppLock: filepath.Join(data, "app.lock"),
        ConfigDir: filepath.Join(root, "config"),
    }
    for _, path := range layout.allManagedPaths() {
        if !isWithin(root, path) {
            return Layout{}, fmt.Errorf("固定路径越出工作目录: %s", path)
        }
        if err := rejectExistingSymlinkComponents(root, path); err != nil { return Layout{}, err }
    }
    return layout, nil
}
```

- [ ] **Step 4: 实现同步原子写入、严格读取、Linux `flock` 与受限清理**

```go
func WriteJSON(path string, value any, perm fs.FileMode) error {
    payload, err := json.MarshalIndent(value, "", "  ")
    if err != nil {
        return err
    }
    payload = append(payload, '\n')
    return WriteFile(path, payload, perm)
}

func WriteFile(path string, payload []byte, perm fs.FileMode) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
        return err
    }
    if err := RejectSymlinkPath(filepath.Dir(path)); err != nil { return err }
    file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
    if err != nil {
        return err
    }
    tempPath := file.Name()
    keep := false
    defer func() {
        file.Close()
        if !keep {
            os.Remove(tempPath)
        }
    }()
    if err := file.Chmod(perm); err != nil { return err }
    if _, err := file.Write(payload); err != nil { return err }
    if err := file.Sync(); err != nil { return err }
    if err := file.Close(); err != nil { return err }
    if err := os.Rename(tempPath, path); err != nil { return err }
    keep = true
    return syncDirectory(filepath.Dir(path))
}
```

`ReadJSON` 必须用 `json.Decoder.DisallowUnknownFields` 读取受管状态结构并确认只有一个 JSON 值；语法或字段错误包装为含文件路径的 `CorruptFileError`。`ListTransactionDirs` 只接受工作目录，只枚举其直接子项中匹配 `config.staging-[0-9a-f]+`、`config.rollback-[0-9a-f]+` 的目录并拒绝符号链接；基础层不得盲删它们，Core Operations Task 7 会结合 `core.json` 和目录摘要决定恢复或清理。

`audit.Event` 的字段固定为 `timestamp/operation/internalId/revision/digest/stage/durationMs/result`，Logger 以单行 JSON 写入注入的 Web 日志 writer。类型没有配置对象、密码、URL、响应正文或任意 `map[string]any` 字段，从编译边界阻止调用者把完整配置与下载内容写入日志。

- [ ] **Step 5: 运行基础设施测试与竞态检查**

Run: `go test -race ./internal/paths ./internal/storage ./internal/audit -v`

Expected: PASS；损坏 JSON 的测试错误包含确切文件路径；第二把锁立即返回 `ErrAlreadyLocked`。

- [ ] **Step 6: 提交固定路径与持久化层**

```bash
git add internal/paths internal/storage internal/audit
git commit -m "实现固定目录与原子持久化"
```

### Task 3: 生成并嵌入默认档案与规则资源模板

**Files:**
- Create: `tools/templategen/main.go`
- Create: `tools/templategen/transform.go`
- Create: `tools/templategen/transform_test.go`
- Create: `internal/templates/embed.go`
- Create: `internal/templates/factory.go`
- Create: `internal/templates/factory_test.go`
- Create: `internal/app/default_bindings.go`
- Create: `internal/app/default_bindings_test.go`
- Create: `internal/templates/assets/default-profile.json`
- Create: `internal/templates/assets/default-rule-resources.json`
- Create: `internal/templates/assets/default-bindings.json`
- Create: `internal/rules/model.go`
- Create: `internal/rules/store.go`
- Create: `internal/rules/store_test.go`
- Create: `internal/profile/model.go`

**Interfaces:**
- Consumes: 开发期显式参数 `-input temp/config.default.json`；运行时只消费三个嵌入资源和固定 `data/rule-resources.json`。
- Produces: `templates.NewFactory(random io.Reader) (*templates.Factory, error)`、`(*Factory).NewProfileSeed(now time.Time) (templates.ProfileSeed, error)`、`rules.Store.EnsureTemplateResources(ctx, templates, random) (map[string]string, error)`、`app.defaultBindingSource.DefaultBindings(ctx)`；模板键由原始 tag 的 SHA-256 前 16 个十六进制字符稳定派生，实际资源 ID 在每个安装内随机生成并持久化。

- [ ] **Step 1: 写远程规则提取与随机化测试**

```go
func TestTransformExtractsRemoteRuleSets(t *testing.T) {
    input := []byte(`{"route":{"rule_set":[{"type":"remote","tag":"GeoSite-CN","format":"binary","url":"https://example.test/cn.srs","download_detour":"direct","update_interval":"1d"}],"rules":[{"rule_set":"GeoSite-CN","outbound":"direct"}]},"experimental":{"clash_api":{"secret":"old"},"cache_file":{"cache_id":"old"}}}`)
    result, err := Transform(input)
    if err != nil { t.Fatal(err) }
    if bytes.Contains(result.Profile, []byte(`"type": "remote"`)) { t.Fatal("remote rule leaked into profile") }
    if bytes.Contains(result.Resources, []byte("download_detour")) { t.Fatal("download_detour leaked into resource template") }
    if bytes.Contains(result.Profile, []byte(`"secret": "old"`)) || bytes.Contains(result.Profile, []byte(`"cache_id": "old"`)) { t.Fatal("source identifiers leaked into tracked template") }
    assertJSONPath(t, result.Resources, "0.url", "https://example.test/cn.srs")
    assertJSONPath(t, result.Bindings, "0.tag", "GeoSite-CN")
}

func TestTransformRejectsEmbeddedCredentialsOutsideGeneratedFields(t *testing.T) {
    input := []byte(`{"outbounds":[{"type":"test","tag":"node-a","password":"real-value-must-not-be-embedded"}],"experimental":{"clash_api":{"secret":"old"},"cache_file":{"cache_id":"old"}}}`)
    if _, err := Transform(input); err == nil || !strings.Contains(err.Error(), "outbounds[0].password") { t.Fatalf("err=%v", err) }
}

func TestFactoryCreatesUniqueSecretsAndCacheIDs(t *testing.T) {
    factory := newTestFactory(t)
    first, _ := factory.NewProfileSeed(time.Unix(1, 0))
    second, _ := factory.NewProfileSeed(time.Unix(2, 0))
    if first.ClashSecret == second.ClashSecret || len(first.ClashSecret) != 64 {
        t.Fatalf("secrets are not unique 32-byte hex values")
    }
    if first.CacheID == second.CacheID || !strings.HasPrefix(first.CacheID, "ID_") {
        t.Fatalf("cache IDs are not unique: %q %q", first.CacheID, second.CacheID)
    }
}

func TestEnsureTemplateResourcesAssignsRandomStableIDs(t *testing.T) {
    layout, _ := paths.Resolve(t.TempDir())
    store, _ := rules.OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(1, 0)))
    templates := []rules.ResourceTemplate{{TemplateKey: "template-cn", Name: "GeoSite-CN", URL: "https://example.test/cn.srs", Format: rules.FormatBinary}}
    first, err := store.EnsureTemplateResources(context.Background(), templates, bytes.NewReader(bytes.Repeat([]byte{0x11}, 32)))
    if err != nil { t.Fatal(err) }
    reopened, _ := rules.OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(2, 0)))
    second, err := reopened.EnsureTemplateResources(context.Background(), templates, bytes.NewReader(bytes.Repeat([]byte{0x22}, 32)))
    if err != nil { t.Fatal(err) }
    if first["template-cn"] == "" || first["template-cn"] != second["template-cn"] { t.Fatalf("ids first=%v second=%v", first, second) }
}
```

- [ ] **Step 2: 运行模板测试并确认失败**

Run: `go test ./tools/templategen ./internal/templates ./internal/rules ./internal/app -v`

Expected: FAIL，错误指向缺失的 `Transform` 和 `NewFactory`。

- [ ] **Step 3: 实现确定性转换器**

```go
type Result struct {
    Profile   []byte
    Resources []byte
    Bindings  []byte
}

func Transform(input []byte) (Result, error) {
    var config map[string]json.RawMessage
    if err := json.Unmarshal(input, &config); err != nil { return Result{}, err }
    route, err := decodeObject(config["route"])
    if err != nil { return Result{}, fmt.Errorf("route: %w", err) }
    remote, local, err := splitRuleSets(route["rule_set"])
    if err != nil { return Result{}, err }
    route["rule_set"] = marshalRaw(local)
    config["route"] = marshalRaw(route)
    if err := setTemplateString(config, []string{"experimental", "clash_api", "secret"}, ""); err != nil { return Result{}, err }
    if err := setTemplateString(config, []string{"experimental", "cache_file", "cache_id"}, "ID_template"); err != nil { return Result{}, err }
    if err := rejectNonEmptyCredentialFields(config); err != nil { return Result{}, err }
    resources, bindings := makeTemplates(remote)
    return Result{
        Profile: stableIndent(config),
        Resources: stableIndent(resources),
        Bindings: stableIndent(bindings),
    }, nil
}
```

`splitRuleSets` 保留手工 `local` 和 `inline` 项，只提取 `type == "remote"`；资源仅保留 `templateKey`、`name`、`url`、`format`、`updateInterval`，绑定仅保留 `templateKey`、`tag`、`enabled`。转换器把跟踪模板中的 clash secret 固定清空、cache ID 固定为非凭据值 `ID_template`，运行时 Factory 必须替换二者；随后递归拒绝其他非空 `password/uuid/private_key/token/client_secret/secret` 字段并报告 JSON path，避免把真实凭据提交进 Git。未知或缺少 `tag/url/format` 的远程项让生成命令失败并指出数组索引。

- [ ] **Step 4: 从当前设计输入生成受版本控制资源**

Run:

```bash
go run ./tools/templategen \
  -input temp/config.default.json \
  -profile internal/templates/assets/default-profile.json \
  -resources internal/templates/assets/default-rule-resources.json \
  -bindings internal/templates/assets/default-bindings.json
```

Expected: 命令退出码 0；`default-profile.json` 不含 `"type": "remote"`、规则 URL、`download_detour` 或输入中的 clash secret/cache ID；另外两个文件合计包含原输入中的 6 个远程规则 tag。

- [ ] **Step 5: 实现运行时工厂和 32 字节随机替换**

```go
type ProfileSeed struct {
    Config      map[string]json.RawMessage
    Buffers     map[string]string
    CreatedAt   time.Time
    ClashSecret string
    CacheID     string
}

func (f *Factory) NewProfileSeed(now time.Time) (ProfileSeed, error) {
    config := cloneRawMap(f.profile)
    secret, err := randomHex(f.random, 32)
    if err != nil { return ProfileSeed{}, err }
    cacheSuffix, err := randomHex(f.random, 16)
    if err != nil { return ProfileSeed{}, err }
    cacheID := "ID_" + cacheSuffix
    if err := setNestedString(config, []string{"experimental", "clash_api", "secret"}, secret); err != nil { return ProfileSeed{}, err }
    if err := setNestedString(config, []string{"experimental", "cache_file", "cache_id"}, cacheID); err != nil { return ProfileSeed{}, err }
    return ProfileSeed{
        Config: config, Buffers: moduleBuffers(config), CreatedAt: now.UTC(),
        ClashSecret: secret, CacheID: cacheID,
    }, nil
}
```

基础 `rules.Store` 在 `rule-resources.json` 中保存完整 Resource 模型和内部 `templateKey`。`EnsureTemplateResources` 对每个嵌入模板复用已有 templateKey 对应 ID；若用户此前删除了一个不再被引用的默认资源，则下一次创建档案前以新随机 ID 重建该模板。它返回 templateKey→resourceID 映射。组合层 `app.defaultBindingSource` 同时持有模板 Factory 与 rules Store，调用 Ensure 后按嵌入的 `default-bindings.json` 生成真实 `profile.RuleBinding`，因此草稿中从不保存模板占位键，也不会在 `templates/profile/rules` 之间形成 import cycle。Rules/Config 计划在相同 Store 上增加 CRUD、下载和调度，不另建状态文件。

```go
type Format string

const (
    FormatSource Format = "source"
    FormatBinary Format = "binary"
)

type ResourceTemplate struct {
    TemplateKey    string `json:"templateKey"`
    Name           string `json:"name"`
    URL            string `json:"url"`
    Format         Format `json:"format"`
    UpdateInterval string `json:"updateInterval"`
}

type Resource struct {
    ID             string `json:"id"`
    TemplateKey    string `json:"templateKey,omitempty"`
    Name           string `json:"name"`
    URL            string `json:"url"`
    Format         Format `json:"format"`
    UpdateInterval string `json:"updateInterval"`
    Disabled       bool   `json:"disabled"`
    LocalPath      string `json:"localPath"`
}
```

```go
func (s *defaultBindingSource) DefaultBindings(ctx context.Context) ([]profile.RuleBinding, error) {
    ids, err := s.rules.EnsureTemplateResources(ctx, s.templates.ResourceTemplates(), s.random)
    if err != nil { return nil, err }
    bindings := make([]profile.RuleBinding, 0, len(s.templates.BindingTemplates()))
    for _, item := range s.templates.BindingTemplates() {
        resourceID, ok := ids[item.TemplateKey]
        if !ok { return nil, fmt.Errorf("默认规则模板缺少资源: %s", item.TemplateKey) }
        bindings = append(bindings, profile.RuleBinding{ResourceID: resourceID, Tag: item.Tag, Enabled: item.Enabled})
    }
    return bindings, nil
}
```

Task 3 同时在 `internal/profile/model.go` 只定义 `RuleBinding{ResourceID,Tag,Enabled}`，让组合层可以返回强类型绑定；Task 6 在同一文件追加完整档案模型。Rules/Config Task 1 在 Resource 上追加下载与调度状态字段；HTTP handler 使用单独 response DTO，不暴露 `templateKey`。

- [ ] **Step 6: 验证生成结果可重复且运行时不访问 `temp`**

Run: `go test ./tools/templategen ./internal/templates ./internal/rules ./internal/app -v && git diff --exit-code -- internal/templates/assets`

Expected: PASS；重复生成后的三个资源无差异；工厂测试在切换到一个不存在 `temp` 的临时工作目录后仍通过。

- [ ] **Step 7: 提交模板转换器与嵌入资源**

```bash
git add tools/templategen internal/templates internal/rules internal/profile/model.go internal/app/default_bindings.go internal/app/default_bindings_test.go
git commit -m "生成默认档案与规则资源模板"
```

### Task 4: 实现 Argon2id 密码文件与初始化命令

**Files:**
- Create: `internal/auth/model.go`
- Create: `internal/auth/store.go`
- Create: `internal/auth/password.go`
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/store_test.go`
- Create: `internal/app/password_command.go`
- Create: `internal/app/password_command_test.go`
- Modify: `internal/app/command.go`

**Interfaces:**
- Consumes: `paths.Layout.AuthFile`/`AppLock`、终端 reader 和确认式密码输入器；不得从 args、环境变量或日志读取密码。
- Produces: `auth.Store.Load() (auth.State, error)`、`auth.Store.SetPassword(password []byte) error`、`auth.Verify(password []byte, encoded string) (bool, error)`；`admin set-password` 成功返回 0。

- [ ] **Step 1: 写密码哈希、轮换和命令输入测试**

```go
func TestSetPasswordStoresArgon2idAndRotatesSessions(t *testing.T) {
    store := newTestStore(t)
    if err := store.SetPassword([]byte("correct horse battery staple")); err != nil { t.Fatal(err) }
    first, _ := store.Load()
    if !strings.HasPrefix(first.PasswordHash, "$argon2id$v=19$m=65536,t=3,p=2$") { t.Fatalf("hash=%q", first.PasswordHash) }
    if ok, _ := Verify([]byte("correct horse battery staple"), first.PasswordHash); !ok { t.Fatal("password did not verify") }
    if err := store.SetPassword([]byte("a second sufficiently long password")); err != nil { t.Fatal(err) }
    second, _ := store.Load()
    if second.SessionGeneration != first.SessionGeneration+1 || second.SessionKey == first.SessionKey { t.Fatal("sessions were not invalidated") }
}

func TestSetPasswordCommandNeverAcceptsPasswordArgument(t *testing.T) {
    if _, err := ParseCommand([]string{"admin", "set-password", "secret"}); err == nil { t.Fatal("password argument accepted") }
}
```

- [ ] **Step 2: 运行认证存储测试并确认失败**

Run: `go test ./internal/auth ./internal/app -run 'TestSetPassword|TestSetPasswordCommand' -v`

Expected: FAIL，缺少 `auth.Store` 或 `Verify`。

- [ ] **Step 3: 实现固定 Argon2id 编码和严格 auth.json**

```go
type State struct {
    Version           int    `json:"version"`
    PasswordHash      string `json:"passwordHash"`
    SessionKey        string `json:"sessionKey"`
    SessionGeneration uint64 `json:"sessionGeneration"`
}

var passwordParams = Params{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 2, SaltBytes: 16, KeyBytes: 32}

func Hash(password []byte, random io.Reader) (string, error) {
    salt := make([]byte, passwordParams.SaltBytes)
    if _, err := io.ReadFull(random, salt); err != nil { return "", err }
    key := argon2.IDKey(password, salt, passwordParams.Iterations, passwordParams.MemoryKiB, passwordParams.Parallelism, passwordParams.KeyBytes)
    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, passwordParams.MemoryKiB, passwordParams.Iterations, passwordParams.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}
```

`Verify` 解析并限制参数上限后重新计算，使用 `subtle.ConstantTimeCompare`；`Store.SetPassword` 生成新的 32 字节签名密钥、递增代次，并通过 `storage.WriteJSON(..., 0o600)` 写入。密码最少 12 个 Unicode 字符，最大 UTF-8 长度 1024 字节，错误信息不回显密码。

- [ ] **Step 4: 接通交互式 `admin set-password`**

```go
type PasswordPrompter interface {
    ReadPassword(prompt string) ([]byte, error)
}

func runSetPassword(layout paths.Layout, prompt PasswordPrompter, stdout io.Writer) error {
    lock, err := storage.AcquireInstanceLock(layout.AppLock)
    if err != nil { return fmt.Errorf("无法在 Web 服务运行时通过 CLI 修改密码: %w", err) }
    defer lock.Close()
    first, err := prompt.ReadPassword("请输入新密码：")
    if err != nil { return err }
    defer clear(first)
    second, err := prompt.ReadPassword("请再次输入新密码：")
    if err != nil { return err }
    defer clear(second)
    if subtle.ConstantTimeCompare(first, second) != 1 { return errors.New("两次输入的密码不一致") }
    if err := auth.NewStore(layout.AuthFile, rand.Reader).SetPassword(first); err != nil { return err }
    fmt.Fprintln(stdout, "管理密码已设置，全部既有会话已失效。")
    return nil
}
```

真实实现对 TTY 使用 `term.ReadPassword`，测试注入 `PasswordPrompter`；标准输出与错误日志只包含结果，不包含密码长度或内容。

- [ ] **Step 5: 运行认证和命令测试**

Run: `go test -race ./internal/auth ./internal/app -v`

Expected: PASS；`auth.json` 权限为 `0600`；修改密码后的旧会话代次不可再验证。

- [ ] **Step 6: 提交密码初始化能力**

```bash
git add go.mod go.sum internal/auth internal/app
git commit -m "实现单用户密码初始化"
```

### Task 5: 实现会话、CSRF、限速与认证 HTTP API

**Files:**
- Create: `internal/apierror/error.go`
- Create: `internal/httpapi/json.go`
- Create: `internal/httpapi/security.go`
- Create: `internal/auth/session.go`
- Create: `internal/auth/session_test.go`
- Create: `internal/auth/limiter.go`
- Create: `internal/auth/limiter_test.go`
- Create: `internal/auth/handler.go`
- Create: `internal/auth/handler_test.go`

**Interfaces:**
- Consumes: `auth.Store`、请求来源 IP、同源 `Origin`、Cookie `singbox_session`、Header `X-CSRF-Token`。
- Produces: `POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`GET /api/v1/auth/session`、`PUT /api/v1/auth/password`；`auth.Middleware(next http.Handler) http.Handler` 和 `auth.RequireCSRF(next http.Handler) http.Handler`。

- [ ] **Step 1: 写认证请求契约和安全属性测试**

```go
func TestLoginAcceptsOnlyPasswordAndReturnsStrictCookie(t *testing.T) {
    handler := newAuthHandler(t, "correct horse battery staple")
    response := postJSON(handler, "/api/v1/auth/login", `{"password":"correct horse battery staple"}`)
    if response.Code != http.StatusOK { t.Fatalf("status=%d body=%s", response.Code, response.Body.String()) }
    cookie := response.Result().Cookies()[0]
    if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure || cookie.MaxAge != 86400 { t.Fatalf("cookie=%+v", cookie) }
    assertJSONPath(t, response.Body.Bytes(), "csrfToken", isNonEmptyString)

    rejected := postJSON(handler, "/api/v1/auth/login", `{"username":"admin","password":"correct horse battery staple"}`)
    if rejected.Code != http.StatusBadRequest { t.Fatalf("username status=%d", rejected.Code) }
}

func TestWriteRequiresSessionAndMatchingCSRF(t *testing.T) {
    handler, cookie, csrf := authenticatedHandler(t)
    request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
    request.AddCookie(cookie)
    response := httptest.NewRecorder()
    handler.ServeHTTP(response, request)
    if response.Code != http.StatusForbidden { t.Fatalf("missing csrf status=%d", response.Code) }
    request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
    request.AddCookie(cookie)
    request.Header.Set("X-CSRF-Token", csrf)
    response = httptest.NewRecorder()
    handler.ServeHTTP(response, request)
    if response.Code != http.StatusNoContent { t.Fatalf("logout status=%d", response.Code) }
}
```

- [ ] **Step 2: 运行 handler 测试并确认失败**

Run: `go test ./internal/auth -run 'TestLogin|TestWrite|TestSession|TestLimiter' -v`

Expected: FAIL，缺少会话或 HTTP handler。

- [ ] **Step 3: 实现签名会话与节流续期**

```go
type Claims struct {
    Generation uint64 `json:"generation"`
    IssuedAt   int64  `json:"issuedAt"`
    LastSeenAt int64  `json:"lastSeenAt"`
    ExpiresAt  int64  `json:"expiresAt"`
    Nonce      string `json:"nonce"`
}

func (s *SessionManager) Issue(state State, now time.Time) (token, csrf string, err error) {
    claims := Claims{Generation: state.SessionGeneration, IssuedAt: now.Unix(), LastSeenAt: now.Unix(), ExpiresAt: now.Add(24*time.Hour).Unix(), Nonce: randomToken(s.random, 16)}
    payload, err := json.Marshal(claims)
    if err != nil { return "", "", err }
    token = signAndEncode(payload, state.SessionKey)
    csrf = deriveCSRF(claims.Nonce, claims.Generation, state.SessionKey)
    return token, csrf, nil
}
```

验证时检查签名、代次和 24 小时无操作窗口；距上次续期至少 5 分钟才用相同 nonce 重发 `LastSeenAt=now`、`ExpiresAt=now+24h` 的 Cookie，避免每个请求都刷新且允许持续活跃会话。CSRF 由稳定 nonce、会话代次和签名密钥派生并常量时间比较，因此滑动续期不会让前端持有的 token 突然失效；修改密码后代次和密钥同时变化。Cookie 始终不可被脚本读取。

- [ ] **Step 4: 实现严格请求解码、统一错误与登录限速**

```go
type Error struct {
    Code           string `json:"code"`
    Message        string `json:"message"`
    Stage          string `json:"stage,omitempty"`
    FieldPath      string `json:"fieldPath,omitempty"`
    LatestRevision uint64 `json:"latestRevision,omitempty"`
    Details        any    `json:"details,omitempty"`
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
    r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields()
    if err := decoder.Decode(dst); err != nil { return apierror.BadRequest("invalid_json", "请求 JSON 无效", err) }
    if decoder.Decode(&struct{}{}) != io.EOF { return apierror.BadRequest("multiple_json_values", "请求只能包含一个 JSON 值", nil) }
    return nil
}
```

登录失败统一返回 `401 {code:"invalid_credentials", message:"密码无效"}`；每个来源在 10 分钟窗口允许 5 次失败，第 6 次返回 429 和 `Retry-After`。成功登录清除该来源计数。`PUT /auth/password` 请求固定为 `{"currentPassword":"...","newPassword":"..."}`，成功轮换签名密钥并清除当前 Cookie。

`GET /auth/session` 是唯一不要求有效会话的管理状态端点：无会话时返回 `200 {"valid":false,"expiresAt":null}`，有效响应的固定形状为 `200 {"valid":true,"expiresAt":"2026-08-22T00:00:00Z","csrfToken":"csrf-test-token"}`（测试时 clock/token 固定为该值，生产返回真实 UTC 时间与签名 token）；两种响应都不含用户名。其他受保护 API 缺少或失效 Cookie 时返回 401。

- [ ] **Step 5: 验证会话失效、同源和限速行为**

Run: `go test -race ./internal/auth ./internal/httpapi -v`

Expected: PASS；修改密码后旧 Cookie 返回 401；带不匹配 `Origin` 的写请求返回 403；第 6 次失败登录返回 429；响应从不含用户名、哈希或签名密钥。

- [ ] **Step 6: 提交认证 API**

```bash
git add internal/apierror internal/httpapi internal/auth
git commit -m "实现安全会话与认证 API"
```

### Task 6: 实现档案元数据、草稿存储与损坏隔离

**Files:**
- Modify: `internal/profile/model.go`
- Create: `internal/profile/store.go`
- Create: `internal/profile/store_test.go`
- Create: `internal/profile/service.go`
- Create: `internal/profile/service_test.go`

**Interfaces:**
- Consumes: `templates.Factory`、`profile.DefaultBindingSource`（由组合层 `app.defaultBindingSource` 实现）、`paths.Layout.ProfilesFile`、`paths.Layout.DraftsDir`、UTC clock 和随机 ID reader。
- Produces: `profile.Service.List`、`Create`、`Rename`、`Delete`、`GetDraft`、`SaveDraft`、`RecordValidation`、`MarkResourcesChanged`、`MarkApplied`；当前档案删除策略通过 `profile.DeleteGuard` 注入，Core Operations Task 4 接入核心实现。

- [ ] **Step 1: 写档案创建、未知字段、修订和草稿隔离测试**

```go
func TestCreateProfileUsesTemplateAndNoProfileDirectory(t *testing.T) {
    service, layout := newTestService(t)
    meta, draft, err := service.Create(context.Background(), CreateInput{Name: "家庭出口"})
    if err != nil { t.Fatal(err) }
    if meta.ID == "" || meta.Name != "家庭出口" || draft.Revision != 1 { t.Fatalf("meta=%+v draft=%+v", meta, draft) }
    if _, ok := draft.Config["experimental"]; !ok { t.Fatal("template not loaded") }
    if _, err := os.Stat(filepath.Join(layout.ConfigDir, meta.ID)); !errors.Is(err, os.ErrNotExist) { t.Fatalf("profile config directory exists: %v", err) }
}

func TestDraftRoundTripPreservesUnknownOutboundFields(t *testing.T) {
    service, _ := newTestService(t)
    meta, _, _ := service.Create(context.Background(), CreateInput{Name: "手工节点"})
    outbound := json.RawMessage(`[{"type":"future-protocol","tag":"node-a","future":{"nested":[1,"x",true]}}]`)
    saved, _, err := service.SaveDraft(context.Background(), meta.ID, `"rev-1"`, DraftInput{Buffers: map[string]string{"outbounds": string(outbound)}})
    if err != nil { t.Fatal(err) }
    if !bytes.Equal(saved.Config["outbounds"], outbound) { t.Fatalf("outbounds changed: %s", saved.Config["outbounds"]) }
}

func TestCorruptSingleDraftQuarantinesOnlyThatProfile(t *testing.T) {
    service, layout := newTestServiceWithTwoProfiles(t)
    os.WriteFile(filepath.Join(layout.DraftsDir, "profile-a.json"), []byte("{"), 0o600)
    profiles, err := service.List(context.Background())
    if err != nil { t.Fatal(err) }
    assertQuarantined(t, profiles, "profile-a")
    assertUsable(t, profiles, "profile-b")
}
```

- [ ] **Step 2: 运行档案测试并确认失败**

Run: `go test ./internal/profile -v`

Expected: FAIL，缺少模型与服务。

- [ ] **Step 3: 定义持久化模型和稳定修订 ETag**

```go
type Meta struct {
    ID                string            `json:"id"`
    Name              string            `json:"name"`
    DraftRevision     uint64            `json:"draftRevision"`
    AppliedRevision   *uint64           `json:"appliedRevision"`
    AppliedDigest     string            `json:"appliedDigest"`
    AppliedResourceDigests map[string]string `json:"appliedResourceDigests"`
    AppliedAt         *time.Time        `json:"appliedAt"`
    NeedsApply        bool              `json:"needsApply"`
    LastValidation    *ValidationSummary `json:"lastValidation"`
    QuarantinedError  string            `json:"quarantinedError,omitempty"`
}

type ValidationSummary struct {
    Valid     bool      `json:"valid"`
    Revision  uint64    `json:"revision"`
    CheckedAt time.Time `json:"checkedAt"`
    Stage     string    `json:"stage"`
    Message   string    `json:"message"`
}

type RuleBinding struct {
    ResourceID string `json:"resourceId"`
    Tag        string `json:"tag"`
    Enabled    bool   `json:"enabled"`
}

type AppliedResult struct {
    ProfileID       string
    Revision        uint64
    Digest          string
    ResourceDigests map[string]string
    AppliedAt       time.Time
    Validation      ValidationSummary
}

type Reference struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type DefaultBindingSource interface {
    DefaultBindings(ctx context.Context) ([]RuleBinding, error)
}

type Draft struct {
    ProfileID string                     `json:"profileId"`
    Revision  uint64                     `json:"revision"`
    Config    map[string]json.RawMessage `json:"config"`
    Buffers   map[string]string          `json:"buffers"`
    Bindings  []RuleBinding              `json:"bindings"`
    SavedAt   time.Time                  `json:"savedAt"`
}

type DraftInput struct {
    Buffers  map[string]string          `json:"buffers"`
    Bindings []RuleBinding              `json:"bindings"`
}

func RevisionETag(revision uint64) string { return fmt.Sprintf(`"rev-%d"`, revision) }
```

`profiles.json` 外层固定为 `{"version":1,"profiles":[...]}`。ID 由 16 个随机字节生成小写 hex；名称去首尾空白后必须为 1–80 个 Unicode 字符，同一时间的档案名称可以重复。服务使用单一互斥锁保证元数据和草稿写入顺序，写草稿成功后再提交元数据修订；启动时用草稿真实修订修复仅由进程崩溃造成的元数据落后，不覆盖损坏草稿。

`Create` 在生成档案前调用 `DefaultBindings(ctx)`；该方法先确保默认资源模板已落入 `rule-resources.json`，再返回包含真实随机 resource ID 的绑定。若规则状态损坏，档案创建失败且不留下元数据或草稿。

```go
func (s *Service) Create(ctx context.Context, input CreateInput) (Meta, Draft, error) {
    name, err := validateName(input.Name)
    if err != nil { return Meta{}, Draft{}, err }
    bindings, err := s.defaultBindings.DefaultBindings(ctx)
    if err != nil { return Meta{}, Draft{}, err }
    seed, err := s.templates.NewProfileSeed(s.clock().UTC())
    if err != nil { return Meta{}, Draft{}, err }
    id, err := randomHex(s.random, 16)
    if err != nil { return Meta{}, Draft{}, err }
    draft := Draft{ProfileID: id, Revision: 1, Config: seed.Config, Buffers: seed.Buffers, Bindings: bindings, SavedAt: seed.CreatedAt}
    meta := Meta{ID: id, Name: name, DraftRevision: 1, NeedsApply: true}
    return s.store.create(meta, draft)
}
```

- [ ] **Step 4: 实现模板创建、保存冲突与状态转换**

```go
func (s *Service) SaveDraft(ctx context.Context, profileID, ifMatch string, input DraftInput) (Draft, string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    meta, draft, err := s.loadUsableProfile(profileID)
    if err != nil { return Draft{}, "", err }
    expected, err := ParseRevisionETag(ifMatch)
    if err != nil { return Draft{}, "", ErrPreconditionRequired }
    if expected != draft.Revision {
        return Draft{}, RevisionETag(draft.Revision), RevisionConflict{LatestRevision: draft.Revision}
    }
    nextConfig, err := reconcileBuffers(draft.Config, input.Buffers)
    if err != nil { return Draft{}, "", err }
    next := Draft{ProfileID: profileID, Revision: draft.Revision + 1, Config: nextConfig, Buffers: maps.Clone(input.Buffers), Bindings: slices.Clone(input.Bindings), SavedAt: s.clock().UTC()}
    if err := validateDraftEnvelope(next, 1<<20, 4<<20); err != nil { return Draft{}, "", err }
    if err := s.store.writeDraft(next); err != nil { return Draft{}, "", err }
    meta.DraftRevision = next.Revision
    meta.NeedsApply = meta.AppliedRevision == nil || *meta.AppliedRevision != next.Revision
    if err := s.store.writeMeta(meta); err != nil { return Draft{}, "", err }
    return next, RevisionETag(next.Revision), nil
}
```

`DraftInput` 只接受 `buffers/bindings`，没有 `config/profileId/revision/savedAt`；严格解码器因此拒绝客户端伪造 envelope 或提交经过 JavaScript 数字转换的配置对象。`reconcileBuffers` 对每个合法模块缓冲使用标准库严格解析：有效文本以原始 `json.RawMessage` 更新 Config，空文本删除模块，无效文本保留上一份有效 Config，同时仍保存原始缓冲供用户修正。`validateDraftEnvelope` 逐个限制模块 RawMessage 与文本缓冲为 1 MiB，并限制整个草稿编码后为 4 MiB；超限错误包含对应模块 fieldPath。

- [ ] **Step 5: 运行存储、竞态和进程重启测试**

Run: `go test -race ./internal/profile -v`

Expected: PASS；同时提交同一个 `If-Match` 时只有一个成功；重建 `Service` 后草稿、修订和隔离状态保持；不存在档案专属 `config` 目录。

- [ ] **Step 6: 提交档案持久化服务**

```bash
git add internal/profile
git commit -m "实现档案草稿持久化"
```

### Task 7: 暴露档案 CRUD、ETag 自动保存与冲突 API

**Files:**
- Create: `internal/profile/handler.go`
- Create: `internal/profile/handler_test.go`
- Modify: `internal/httpapi/router.go`
- Create: `internal/httpapi/router_test.go`

**Interfaces:**
- Consumes: 已认证会话、写请求 CSRF、`profile.Service`。
- Produces: `GET/POST /api/v1/profiles`、`PATCH/DELETE /api/v1/profiles/{id}`、`GET/PUT /api/v1/profiles/{id}/draft`；读取返回 `ETag`，保存要求 `If-Match`。

- [ ] **Step 1: 写档案 API 和冲突响应测试**

```go
func TestDraftAPIRequiresIfMatchAndReturnsLatestRevision(t *testing.T) {
    handler, session := newAuthenticatedProfileAPI(t)
    profileID := createProfile(t, handler, session, `{"name":"家庭出口"}`)
    get := request(t, handler, session, http.MethodGet, "/api/v1/profiles/"+profileID+"/draft", "")
    if get.Header().Get("ETag") != `"rev-1"` { t.Fatalf("etag=%q", get.Header().Get("ETag")) }

    first := requestWithHeaders(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", validDraftBody(), map[string]string{"If-Match": `"rev-1"`})
    if first.Code != http.StatusOK || first.Header().Get("ETag") != `"rev-2"` { t.Fatalf("first=%d etag=%q", first.Code, first.Header().Get("ETag")) }

    stale := requestWithHeaders(t, handler, session, http.MethodPut, "/api/v1/profiles/"+profileID+"/draft", validDraftBody(), map[string]string{"If-Match": `"rev-1"`})
    if stale.Code != http.StatusConflict { t.Fatalf("stale=%d", stale.Code) }
    assertJSONPath(t, stale.Body.Bytes(), "error.code", "draft_revision_conflict")
    assertJSONPath(t, stale.Body.Bytes(), "error.latestRevision", float64(2))
}

func TestProfileCreateRejectsDirectoryFields(t *testing.T) {
    handler, session := newAuthenticatedProfileAPI(t)
    for _, body := range []string{`{"name":"x","configDir":"x"}`, `{"name":"x","path":"../x"}`} {
        response := request(t, handler, session, http.MethodPost, "/api/v1/profiles", body)
        if response.Code != http.StatusBadRequest { t.Fatalf("body=%s status=%d", body, response.Code) }
    }
}
```

- [ ] **Step 2: 运行 API 测试并确认失败**

Run: `go test ./internal/profile ./internal/httpapi -run 'TestDraftAPI|TestProfileCreate|TestProfile' -v`

Expected: FAIL，路由返回 404 或 handler 未定义。

- [ ] **Step 3: 实现严格 DTO 和 ETag handler**

```go
type createRequest struct { Name string `json:"name"` }
type renameRequest struct { Name string `json:"name"` }
type deleteRequest struct {
    ConfirmName string `json:"confirmName"`
    StopCurrent bool   `json:"stopCurrent"`
}

func (h *Handler) putDraft(w http.ResponseWriter, r *http.Request) {
    profileID := r.PathValue("id")
    ifMatch := r.Header.Get("If-Match")
    if ifMatch == "" {
        apierror.Write(w, http.StatusPreconditionRequired, apierror.Error{Code: "if_match_required", Message: "保存草稿必须携带 If-Match", Stage: "save_draft"})
        return
    }
    var input DraftInput
    if err := httpapi.DecodeJSON(w, r, &input, 4<<20); err != nil { apierror.WriteError(w, err); return }
    draft, etag, err := h.service.SaveDraft(r.Context(), profileID, ifMatch, input)
    if conflict := new(RevisionConflict); errors.As(err, conflict) {
        w.Header().Set("ETag", RevisionETag(conflict.LatestRevision))
        apierror.Write(w, http.StatusConflict, apierror.Error{Code: "draft_revision_conflict", Message: "服务器草稿已被其他页面更新", Stage: "save_draft", LatestRevision: conflict.LatestRevision})
        return
    }
    if err != nil { apierror.WriteError(w, err); return }
    w.Header().Set("ETag", etag)
    httpapi.WriteJSON(w, http.StatusOK, draft)
}
```

删除必须精确匹配档案显示名称。基础阶段的 `DeleteGuard` 对“当前档案”返回 `current_profile_requires_core_decision`；Core Operations 计划替换为可以停止核心和删除固定 `config` 的实现。普通档案删除只删除元数据和对应草稿，不扫描或删除其他路径。

- [ ] **Step 4: 运行 HTTP 契约、请求体限制与认证测试**

Run: `go test -race ./internal/profile ./internal/httpapi -v`

Expected: PASS；未认证返回 401；写请求无 CSRF 返回 403；超 4 MiB 草稿返回 413；冲突保留服务端和客户端内容且响应提供最新修订。

- [ ] **Step 5: 提交档案 API**

```bash
git add internal/profile internal/httpapi
git commit -m "提供档案与草稿 API"
```

### Task 8: 组合服务启动、安全头、能力声明与 SPA fallback

**Files:**
- Create: `internal/capabilities/service.go`
- Create: `internal/capabilities/handler.go`
- Create: `internal/capabilities/handler_test.go`
- Create: `internal/httpapi/router.go`
- Modify: `internal/httpapi/security.go`
- Create: `internal/httpapi/security_test.go`
- Create: `internal/webui/handler.go`
- Create: `internal/webui/handler_test.go`
- Create: `internal/app/server.go`
- Create: `internal/app/server_test.go`
- Modify: `internal/app/command.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `paths.Layout`、`auth.Store`、`profile.Service`、嵌入资源和 `127.0.0.1:8080` 固定默认监听地址。
- Produces: 可运行的 `singbox-web serve`；`GET /api/v1/capabilities`；安全响应头；前端路由 fallback；`Server.Close(ctx)` 释放实例锁。

- [ ] **Step 1: 写未初始化拒绝、独占锁、安全头和路由边界测试**

```go
func TestServeRefusesMissingPassword(t *testing.T) {
    app := newTestApplication(t)
    err := app.Start(context.Background())
    if !errors.Is(err, auth.ErrNotInitialized) || !strings.Contains(err.Error(), "singbox-web admin set-password") { t.Fatalf("err=%v", err) }
}

func TestSecurityHeadersAndSPAFallback(t *testing.T) {
    handler := newInitializedRouter(t)
    spa := httptest.NewRecorder()
    handler.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/profiles/abc/config", nil))
    if spa.Code != http.StatusOK || !strings.Contains(spa.Body.String(), "id=\"app\"") { t.Fatalf("spa response=%d %s", spa.Code, spa.Body.String()) }
    if spa.Header().Get("Content-Security-Policy") == "" || spa.Header().Get("X-Content-Type-Options") != "nosniff" { t.Fatalf("headers=%v", spa.Header()) }
    api := httptest.NewRecorder()
    handler.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil))
    if api.Code != http.StatusNotFound || strings.Contains(api.Body.String(), "<html") { t.Fatalf("api fallback leaked HTML") }
    traversal := httptest.NewRecorder()
    handler.ServeHTTP(traversal, httptest.NewRequest(http.MethodGet, "/assets/%2e%2e/%2e%2e/data/auth.json", nil))
    if traversal.Code != http.StatusBadRequest && traversal.Code != http.StatusNotFound { t.Fatalf("traversal status=%d", traversal.Code) }
}
```

- [ ] **Step 2: 运行组合测试并确认失败**

Run: `go test ./internal/app ./internal/httpapi ./internal/webui ./internal/capabilities -v`

Expected: FAIL，缺少 server、能力 handler 或 SPA handler。

- [ ] **Step 3: 实现一期能力声明与安全响应头**

```go
type Response struct {
    Configuration bool `json:"configuration"`
    RuleResources bool `json:"ruleResources"`
    Application   bool `json:"application"`
    CoreUpdate    bool `json:"coreUpdate"`
    Runtime       bool `json:"runtime"`
    Traffic       bool `json:"traffic"`
    Connections   bool `json:"connections"`
    ProxyGroups   bool `json:"proxyGroups"`
}

var PhaseOne = Response{Configuration: true, RuleResources: true, Application: true, CoreUpdate: true}
```

所有响应设置 `X-Content-Type-Options: nosniff`、`Referrer-Policy: no-referrer`、`Content-Security-Policy: default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'`。HTML 使用 `Cache-Control: no-cache`，带内容 hash 的静态文件使用一年 immutable，API 使用 `no-store`。

webui handler 只用 `io/fs` 打开嵌入 FS 中通过 `fs.ValidPath` 的相对资产名，不做宿主文件系统回退、目录列表或字符串拼接；包含 `..`、反斜杠、NUL 的路径直接 400/404。

- [ ] **Step 4: 组合启动流程并确保失败时释放资源**

```go
func NewServer(workDir string, deps Dependencies) (*Server, error) {
    layout, err := paths.Resolve(workDir)
    if err != nil { return nil, err }
    lock, err := storage.AcquireInstanceLock(layout.AppLock)
    if err != nil { return nil, err }
    keep := false
    defer func() { if !keep { lock.Close() } }()
    authStore := auth.NewStore(layout.AuthFile, deps.Random)
    if _, err := authStore.Load(); err != nil { return nil, fmt.Errorf("请先执行 singbox-web admin set-password: %w", err) }
    ruleStore, err := rules.OpenStore(layout.RulesFile, layout, deps.Clock)
    if err != nil { return nil, err }
    bindingSource := newDefaultBindingSource(deps.TemplateFactory, ruleStore, deps.Random)
    profileService, err := profile.Open(layout, deps.TemplateFactory, bindingSource, deps.Clock, deps.Random)
    if err != nil { return nil, err }
    handler := httpapi.NewRouter(httpapi.Dependencies{Auth: authStore, Profiles: profileService, Capabilities: capabilities.PhaseOne, WebUI: webui.Handler()})
    keep = true
    return &Server{layout: layout, lock: lock, http: &http.Server{Addr: "127.0.0.1:8080", Handler: handler}}, nil
}
```

`Server.Run(ctx)` 在 context 取消后最多等待 10 秒完成 HTTP shutdown 并总是关闭锁；初始化任一核心 JSON 文件损坏时返回包含文件路径的错误，且不启动监听。

- [ ] **Step 5: 更新 README 的一期开发命令和边界**

README 写明：

```text
go run ./cmd/singbox-web admin set-password
go run ./cmd/singbox-web serve
go test ./...
```

同时说明工作目录固定生成 `data/` 与 `config/`，服务默认只监听 `127.0.0.1:8080`，当前阶段不包含 systemd、Caddy、HTTPS 或公网部署。保留 `prototype/` 的预览说明，但明确它只是视觉参照，不是运行依赖。

- [ ] **Step 6: 运行基础阶段全部验证**

Run:

```bash
go test -race ./...
go vet ./...
go build ./cmd/singbox-web
npm --prefix prototype run check
git status --short
```

Expected: 前四条命令退出码为 0；`git status --short` 只显示本任务尚未提交的预期文件；没有生成或暂存 `data/`、`config/`、`temp/` 或日志。

- [ ] **Step 7: 提交可运行基础阶段**

```bash
git add internal/capabilities internal/httpapi internal/webui internal/app README.md
git commit -m "组合认证与档案服务"
```
