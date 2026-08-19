# sing-box Web 管理页面一期 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个经 Caddy 暴露到公网、由单个 Go 可执行文件承载 Vue 前端，并能安全管理多套 sing-box 多文件配置的第一期 Web 应用。

**Architecture:** Go 模块化单体负责单管理员认证、JSON 应用数据、服务器端草稿、Schema/语义检查、`sing-box check` 和原子发布；Vue 3 SPA 负责多档案配置编辑、JSON 兜底、发布状态及一期未接入能力的真实占位。应用内部 JSON 数据与正式配置目录分离，稳定 slug 符号链接指向每次发布生成的不可变文件集。

**Tech Stack:** Go 1.24+、Go 标准库 HTTP、`golang.org/x/crypto`、`golang.org/x/sys`、`golang.org/x/term`、`github.com/santhosh-tekuri/jsonschema/v5`、Vue 3、TypeScript、Vite、Pinia、Vue Router、Vitest、Vue Test Utils、Playwright、Caddy、systemd。

## Global Constraints

- 规格事实来源：`docs/superpowers/specs/2026-08-18-singbox-web-phase1-design.md`。
- 目标平台为 Linux 单机；Go 服务默认仅监听 `127.0.0.1`，Caddy 负责公网 HTTPS 到本机 HTTP 的反向代理。
- 生产产物是一个内嵌前端资源的 Go 可执行文件，服务以专用非 root 用户运行。
- 一期只允许调用 `sing-box version` 和 `sing-box check`；不得调用 `sing-box run`，不得启停核心或改变网络状态。
- 只面向 Web 发布时的最新稳定版 sing-box，不实现旧版本字段转换或多版本兼容。
- 官方 Schema 和模板必须作为受版本控制资源嵌入；新档案以 `temp/config.default.json` 为内容基准，并重新生成 32 字节十六进制 Clash API Secret 和独立 `cache_id`。
- 生产运行时不联网获取 Schema、订阅或规则资源；只有显式维护脚本可在获批后联网更新受版本控制资源。
- 应用数据和草稿只使用 JSON 文件，不引入 SQLite 或其他数据库。
- API 请求体上限固定为 2,000,000 bytes；草稿中每个顶层模块和每个原始编辑器缓冲分别限制为 1,000,000 UTF-8 bytes，与 Caddy 的 `2MB` 外层限制对齐。
- 支持多套档案；每套正式配置按一个顶层模块一个 JSON 文件拆分，合并后的 `config.json` 不得写进 `-C` 模块目录。
- 草稿在服务器自动保存，正式发布必须依次通过 JSON、Schema、语义和真实 `sing-box check -C`。
- 不提供配置历史、回滚、无效配置强制发布、已有配置导入、代理订阅拉取或运行核心操作。
- 完整导出包含敏感值；普通预览默认遮罩，并提供脱敏复制。
- 管理员会话采用 24 小时滑动空闲过期，认证请求至多每 15 分钟续签一次，并保持当前 CSRF Token 不变。
- 未接入页面不得展示模拟运行数据，必须根据 `/api/v1/capabilities` 显示明确不可用状态。
- 所有功能按测试先行实现；每个任务只暂存列出的文件，并使用中文 Git 提交信息。
- 当前工作树已有用户改动：`index.html` 被修改，`docs/automated-testing.md` 和 `docs/manual-testing.md` 被删除。实现期间不得恢复、覆盖或暂存这三项改动。
- 当前环境 Node 为 `v22.22.0`、npm 为 `10.9.4`；Go、Caddy 和 sing-box 尚未安装。执行 Task 1 前安装 Go 1.24+，执行真实代理与契约验证前安装 Caddy 和最新稳定版 sing-box；需要联网或系统写入时先请求用户批准。

---

## Scope Check

认证、档案、配置编译、发布和前端是同一个可独立验收的一期产品的依赖链，而不是可以分别交付的独立产品。本计划保留为一份纵向递进计划；每个任务仍形成可单独测试和审查的提交。

## Target File Map

### Go 入口与装配

- `go.mod`、`go.sum`：Go 模块和固定依赖。
- `cmd/singbox-web/main.go`、`cmd/singbox-web/deps.go`：进程入口和真实依赖装配。
- `internal/cli/cli.go`：`version`、`admin set-password`、`serve` 命令解析。
- `internal/app/app.go`：服务启动、目录锁、依赖装配、恢复和 HTTP 生命周期。

### 后端领域包

- `internal/atomicfile/json.go`、`internal/atomicfile/lock_linux.go`：原子 JSON 写入和 Linux 独占锁。
- `internal/auth/password.go`、`store.go`、`session.go`：Argon2id、`auth.json` 和签名会话。
- `internal/assets/assets.go`、`config.default.json`、`sing-box.schema.json`、`schema.version.json`：嵌入模板和最新版 Schema。
- `internal/configdoc/template.go`、`modules.go`、`redact.go`：模板实例化、模块拆合、稳定摘要和脱敏。
- `internal/profile/model.go`、`repository.go`、`service.go`：档案元数据、草稿修订和 JSON 持久化。
- `internal/validation/types.go`、`schema.go`、`semantic.go`：Schema 与跨模块语义检查。
- `internal/singbox/runner.go`：受限的 `version` 和 `check -C` 子进程。
- `internal/publication/service.go`：生成目录、原子链接切换、漂移检测和崩溃恢复。
- `internal/audit/logger.go`：只接受安全字段的结构化操作日志。
- `internal/capabilities/service.go`：一期和二期能力声明。
- `internal/httpapi/server.go`、`middleware.go`、`errors.go`、`auth_handlers.go`、`profile_handlers.go`、`config_handlers.go`：同源 `/api/v1`。
- `internal/webui/embed.go`、`handler.go`、`dist/placeholder.txt`：Vue 产物嵌入和 SPA fallback。

### Vue 前端

- `web/package.json`、`package-lock.json`、`vite.config.ts`、`tsconfig*.json`：前端工具链。
- `web/src/main.ts`、`App.vue`、`router.ts`：SPA 入口。
- `web/src/api/client.ts`、`types.ts`：类型化 HTTP 客户端和后端契约。
- `web/src/stores/auth.ts`、`capabilities.ts`、`profiles.ts`、`draft.ts`：集中状态。
- `web/src/features/auth/*`：登录和密码修改。
- `web/src/features/shell/*`、`dashboard/*`、`placeholders/*`、`settings/*`：原型外壳和能力占位。
- `web/src/features/profiles/*`：档案创建、切换、重命名和删除。
- `web/src/features/config/*`：通用编辑工作区、模块表单、JSON 编辑、引用和错误定位。
- `web/src/features/publication/*`：差异、检查、写入和导出。
- `web/src/styles/prototype.css`、`web/src/styles/app.css`：复用现有视觉样式和 Vue 补充样式。

### 构建、部署与验证

- `scripts/build.sh`、`scripts/update-singbox-schema.sh`：可复现构建与 Schema 更新。
- `deploy/caddy/Caddyfile.example`、`deploy/systemd/singbox-web.service`：生产部署示例。
- `tests/contract/*`：真实最新版 sing-box 只读契约测试。
- `web/e2e/*`、`web/playwright.config.ts`：浏览器端到端测试。
- `README.md`：开发、构建、部署、安全和恢复说明。

---

### Task 1: 建立 Go/Vue 双端工程与可运行测试入口

**Files:**
- Create: `go.mod`
- Create: `cmd/singbox-web/main.go`
- Create: `internal/cli/cli.go`
- Create: `internal/cli/cli_test.go`
- Create: `web/package.json`
- Create: `web/package-lock.json`
- Create: `web/tsconfig.json`
- Create: `web/tsconfig.app.json`
- Create: `web/vite.config.ts`
- Create: `web/src/appMeta.ts`
- Create: `web/src/appMeta.test.ts`
- Modify: `package.json:1-12`
- Modify: `.gitignore:1-4`

**Interfaces:**
- Consumes: 无。
- Produces: `cli.Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps cli.Dependencies) int`；`cli.Dependencies` 包含 `Version string`、`ReadPassword func() ([]byte, error)`、`SetPassword func(context.Context, string, []byte) error`、`Serve func(context.Context, cli.ServeOptions) error`；前端 `appTitle(): string`。

- [ ] **Step 1: 确认工具链并创建依赖清单**

Run:

```bash
go version
node --version
npm --version
```

Expected: Go 为 `go1.24.0` 或更高；Node 输出 `v22.22.0`；npm 输出 `10.9.4`。若 Go 仍不存在，先请求用户批准安装，不以其他语言替代。

Create `go.mod`:

```go
module singbox-web

go 1.24
```

Replace root `package.json` with:

```json
{
  "name": "singbox-web",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "npm --prefix web run dev",
    "build:web": "npm --prefix web run build",
    "test:web": "npm --prefix web run test",
    "typecheck": "npm --prefix web run typecheck"
  }
}
```

Create `web/package.json`:

```json
{
  "name": "singbox-web-frontend",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite --host 127.0.0.1",
    "build": "vue-tsc -b && vite build",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "vue-tsc -b --pretty false"
  }
}
```

Install and lock the frontend toolchain:

```bash
npm install --prefix web vue@3 pinia@3 vue-router@4
npm install --prefix web --save-dev typescript@5 @types/node@22 vite@7 @vitejs/plugin-vue@6 vue-tsc@3 vitest@3 jsdom@26 @vue/test-utils@2
```

- [ ] **Step 2: 写入失败的 CLI 和前端元数据测试**

Create `internal/cli/cli_test.go`:

```go
package cli

import (
    "bytes"
    "context"
    "io"
    "strings"
    "testing"
)

func TestRunVersion(t *testing.T) {
    var stdout bytes.Buffer
    code := Run(context.Background(), []string{"version"}, strings.NewReader(""), &stdout, io.Discard, Dependencies{Version: "0.1.0-test"})
    if code != 0 {
        t.Fatalf("Run() code = %d, want 0", code)
    }
    if got, want := stdout.String(), "singbox-web 0.1.0-test\n"; got != want {
        t.Fatalf("stdout = %q, want %q", got, want)
    }
}
```

Create `web/src/appMeta.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { appTitle } from "./appMeta";

describe("appTitle", () => {
  it("returns the production console title", () => {
    expect(appTitle()).toBe("sing-box Web 管理");
  });
});
```

- [ ] **Step 3: 运行测试确认失败**

Run:

```bash
go test ./internal/cli
npm --prefix web run test -- src/appMeta.test.ts
```

Expected: Go 因 `Run`、`Dependencies` 未定义而 FAIL；Vitest 因 `./appMeta` 不存在而 FAIL。

- [ ] **Step 4: 实现最小 CLI 和前端元数据**

Create `internal/cli/cli.go`:

```go
package cli

import (
    "context"
    "fmt"
    "io"
)

type ServeOptions struct {
    Listen      string
    DataDir     string
    ConfigRoot  string
    SingBoxPath string
    ConfigGroup string
}

type Dependencies struct {
    Version     string
    ReadPassword func() ([]byte, error)
    SetPassword  func(context.Context, string, []byte) error
    Serve        func(context.Context, ServeOptions) error
}

func Run(_ context.Context, args []string, _ io.Reader, stdout, stderr io.Writer, deps Dependencies) int {
    if len(args) == 1 && args[0] == "version" {
        fmt.Fprintf(stdout, "singbox-web %s\n", deps.Version)
        return 0
    }
    fmt.Fprintln(stderr, "用法: singbox-web <version|admin|serve>")
    return 2
}
```

Create `cmd/singbox-web/main.go`:

```go
package main

import (
    "context"
    "os"

    "singbox-web/internal/cli"
)

var version = "dev"

func main() {
    deps := cli.Dependencies{Version: version}
    os.Exit(cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, deps))
}
```

Create `web/src/appMeta.ts`:

```ts
export function appTitle(): string {
  return "sing-box Web 管理";
}
```

Configure TypeScript, Vite and Vitest with `@vitejs/plugin-vue`, `environment: "jsdom"`, and alias `@` to `web/src`. Add these generated directories to `.gitignore` without changing the existing `temp/` rule:

```text
web/node_modules/
web/dist/
dist/
internal/webui/dist/*
!internal/webui/dist/placeholder.txt
```

- [ ] **Step 5: 运行基础验证**

Run:

```bash
go test ./internal/cli
npm --prefix web run test -- src/appMeta.test.ts
npm run typecheck
go run ./cmd/singbox-web version
```

Expected: 全部 PASS；最后输出 `singbox-web dev`。

- [ ] **Step 6: 提交工程骨架**

```bash
git add go.mod cmd/singbox-web/main.go internal/cli web package.json .gitignore
git commit -m "建立 Go 与 Vue 工程骨架"
```

---

### Task 2: 实现原子 JSON 文件和单进程锁

**Files:**
- Create: `internal/atomicfile/json.go`
- Create: `internal/atomicfile/json_test.go`
- Create: `internal/atomicfile/lock_linux.go`
- Create: `internal/atomicfile/lock_linux_test.go`
- Modify: `go.mod`
- Create: `go.sum`

**Interfaces:**
- Consumes: Go 1.24 文件系统 API。
- Produces: `atomicfile.WriteJSON(path string, mode fs.FileMode, value any) error`、`atomicfile.ReadJSON(path string, value any) error`、`atomicfile.Acquire(path string) (*atomicfile.Lock, error)`、`(*atomicfile.Lock).Close() error`。

- [ ] **Step 1: 写入原子写入和锁的失败测试**

Create `internal/atomicfile/json_test.go` with these cases:

```go
package atomicfile

import (
    "os"
    "path/filepath"
    "testing"
)

func TestWriteJSONReplacesWholeFileAndSetsMode(t *testing.T) {
    path := filepath.Join(t.TempDir(), "state.json")
    if err := os.WriteFile(path, []byte(`{"old":true}`), 0o666); err != nil {
        t.Fatal(err)
    }
    if err := WriteJSON(path, 0o600, map[string]any{"new": true}); err != nil {
        t.Fatal(err)
    }
    var got map[string]any
    if err := ReadJSON(path, &got); err != nil {
        t.Fatal(err)
    }
    if got["new"] != true || got["old"] != nil {
        t.Fatalf("decoded state = %#v", got)
    }
    info, err := os.Stat(path)
    if err != nil {
        t.Fatal(err)
    }
    if info.Mode().Perm() != 0o600 {
        t.Fatalf("mode = %o, want 600", info.Mode().Perm())
    }
}

func TestReadJSONRejectsTrailingContent(t *testing.T) {
    path := filepath.Join(t.TempDir(), "state.json")
    if err := os.WriteFile(path, []byte(`{"ok":true}{"extra":true}`), 0o600); err != nil {
        t.Fatal(err)
    }
    var got map[string]any
    if err := ReadJSON(path, &got); err == nil {
        t.Fatal("ReadJSON() error = nil, want trailing-content error")
    }
}
```

Create `internal/atomicfile/lock_linux_test.go`:

```go
//go:build linux

package atomicfile

import (
    "path/filepath"
    "testing"
)

func TestAcquireRejectsSecondOwner(t *testing.T) {
    path := filepath.Join(t.TempDir(), "app.lock")
    first, err := Acquire(path)
    if err != nil {
        t.Fatal(err)
    }
    defer first.Close()
    if second, err := Acquire(path); err == nil {
        second.Close()
        t.Fatal("second Acquire() succeeded")
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/atomicfile
```

Expected: FAIL，提示 `WriteJSON`、`ReadJSON` 和 `Acquire` 未定义。

- [ ] **Step 3: 实现原子 JSON 写入**

Implement `internal/atomicfile/json.go` with this exact write order:

```go
package atomicfile

import (
    "encoding/json"
    "errors"
    "io"
    "io/fs"
    "os"
    "path/filepath"
)

func WriteJSON(path string, mode fs.FileMode, value any) (retErr error) {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return err
    }
    temp, err := os.CreateTemp(dir, ".json-*")
    if err != nil {
        return err
    }
    tempName := temp.Name()
    defer func() {
        temp.Close()
        if retErr != nil {
            os.Remove(tempName)
        }
    }()
    if err := temp.Chmod(mode); err != nil {
        return err
    }
    encoder := json.NewEncoder(temp)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(value); err != nil {
        return err
    }
    if err := temp.Sync(); err != nil {
        return err
    }
    if err := temp.Close(); err != nil {
        return err
    }
    if err := os.Rename(tempName, path); err != nil {
        return err
    }
    dirHandle, err := os.Open(dir)
    if err != nil {
        return err
    }
    defer dirHandle.Close()
    return dirHandle.Sync()
}

func ReadJSON(path string, value any) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()
    decoder := json.NewDecoder(file)
    decoder.UseNumber()
    if err := decoder.Decode(value); err != nil {
        return err
    }
    var trailing any
    if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
        if err == nil {
            return errors.New("JSON contains trailing value")
        }
        return err
    }
    return nil
}
```

- [ ] **Step 4: 实现 Linux 独占锁**

Add `golang.org/x/sys/unix`, then implement `internal/atomicfile/lock_linux.go` using `unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)`. `Close` must unlock before closing and be safe to call once. The exported shape is:

```go
//go:build linux

package atomicfile

import "os"

type Lock struct {
    file *os.File
}

func Acquire(path string) (*Lock, error)
func (lock *Lock) Close() error
```

- [ ] **Step 5: 运行测试和竞态检查**

Run:

```bash
go test ./internal/atomicfile
go test -race ./internal/atomicfile
```

Expected: 全部 PASS，竞态检测无报告。

- [ ] **Step 6: 提交原子存储基础**

```bash
git add internal/atomicfile go.mod go.sum
git commit -m "实现原子 JSON 存储与进程锁"
```

---

### Task 3: 实现管理员密码存储与 CLI 初始化

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/store.go`
- Create: `internal/auth/store_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Create: `cmd/singbox-web/deps.go`
- Modify: `cmd/singbox-web/main.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: `atomicfile.WriteJSON`、`atomicfile.ReadJSON`、Task 1 的 `cli.Dependencies.SetPassword`。
- Produces: `auth.NewFileStore(path string, now func() time.Time, random io.Reader) *auth.FileStore`、`(*auth.FileStore).SetPassword(password []byte) error`、`(*auth.FileStore).Authenticate(password []byte) (auth.Record, bool, error)`、`(*auth.FileStore).Load() (auth.Record, error)`；CLI `admin set-password --data-dir`。

- [ ] **Step 1: 写入密码哈希和持久化失败测试**

Create `internal/auth/store_test.go`:

```go
package auth

import (
    "bytes"
    "path/filepath"
    "testing"
    "time"
)

func TestSetPasswordStoresHashAndRotatesSessions(t *testing.T) {
    now := func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
    randomBytes := append(bytes.Repeat([]byte{1}, 48), bytes.Repeat([]byte{2}, 48)...)
    store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), now, bytes.NewReader(randomBytes))
    if err := store.SetPassword([]byte("first-strong-password")); err != nil {
        t.Fatal(err)
    }
    first, ok, err := store.Authenticate([]byte("first-strong-password"))
    if err != nil || !ok {
        t.Fatalf("Authenticate() ok = %v, err = %v", ok, err)
    }
    if first.PasswordHash == "first-strong-password" || first.SessionKey == "" {
        t.Fatalf("unsafe record = %#v", first)
    }
    if err := store.SetPassword([]byte("second-strong-password")); err != nil {
        t.Fatal(err)
    }
    second, ok, err := store.Authenticate([]byte("second-strong-password"))
    if err != nil || !ok {
        t.Fatalf("second Authenticate() ok = %v, err = %v", ok, err)
    }
    if second.SessionEpoch != first.SessionEpoch+1 || second.SessionKey == first.SessionKey {
        t.Fatalf("session rotation: first=%#v second=%#v", first, second)
    }
}
```

Add password tests that accept the correct password, reject a wrong password, reject passwords shorter than 12 Unicode code points, and parse the stored PHC string after reopening the file.

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/auth
```

Expected: FAIL，提示 `NewFileStore`、`Record` 和密码函数未定义。

- [ ] **Step 3: 实现 Argon2id 和 auth.json**

Add `golang.org/x/crypto/argon2`. Define `Record` in `store.go`:

```go
type Record struct {
    Version       int       `json:"version"`
    PasswordHash  string    `json:"passwordHash"`
    SessionKey    string    `json:"sessionKey"`
    SessionEpoch  uint64    `json:"sessionEpoch"`
    UpdatedAt     time.Time `json:"updatedAt"`
}
```

Use Argon2id parameters `memory=65536 KiB`、`iterations=3`、`parallelism=2`、`salt=16 bytes`、`key=32 bytes` and encode a standard `$argon2id$v=19$...` PHC string. `SetPassword` must:

```go
func (store *FileStore) SetPassword(password []byte) error {
    if utf8.RuneCount(password) < 12 {
        return ErrWeakPassword
    }
    previous, err := store.Load()
    if err != nil && !errors.Is(err, os.ErrNotExist) {
        return err
    }
    hash, err := HashPassword(password, store.random)
    if err != nil {
        return err
    }
    sessionKey := make([]byte, 32)
    if _, err := io.ReadFull(store.random, sessionKey); err != nil {
        return err
    }
    record := Record{
        Version: 1, PasswordHash: hash,
        SessionKey: hex.EncodeToString(sessionKey),
        SessionEpoch: previous.SessionEpoch + 1,
        UpdatedAt: store.now().UTC(),
    }
    return atomicfile.WriteJSON(store.path, 0o600, record)
}
```

`Authenticate` must use constant-time key comparison and return a generic false result for wrong passwords.

- [ ] **Step 4: 为 CLI 写入失败测试并实现交互式初始化**

Extend `internal/cli/cli_test.go`:

```go
func TestRunAdminSetPassword(t *testing.T) {
    var gotDir string
    var gotPassword []byte
    deps := Dependencies{
        Version: "test",
        ReadPassword: func() ([]byte, error) { return []byte("a-strong-password"), nil },
        SetPassword: func(_ context.Context, dataDir string, password []byte) error {
            gotDir = dataDir
            gotPassword = append([]byte(nil), password...)
            return nil
        },
    }
    code := Run(context.Background(), []string{"admin", "set-password", "--data-dir", "/var/lib/singbox-web"}, strings.NewReader(""), io.Discard, io.Discard, deps)
    if code != 0 || gotDir != "/var/lib/singbox-web" || string(gotPassword) != "a-strong-password" {
        t.Fatalf("code=%d dir=%q password=%q", code, gotDir, gotPassword)
    }
}
```

Run `go test ./internal/cli` and verify it fails, then parse the command with a dedicated `flag.FlagSet`. Read and confirm the password twice through `Dependencies.ReadPassword`, zero both byte slices after `SetPassword`, and never write the password to stdout or stderr.

Create `cmd/singbox-web/deps.go` using `golang.org/x/term.ReadPassword` and `auth.NewFileStore(filepath.Join(dataDir, "auth.json"), time.Now, rand.Reader).SetPassword`.

- [ ] **Step 5: 验证认证存储和 CLI**

Run:

```bash
go test ./internal/auth ./internal/cli
go test -race ./internal/auth
tmp_dir=$(mktemp -d)
printf 'a-strong-password\na-strong-password\n' | go run ./cmd/singbox-web admin set-password --data-dir "$tmp_dir"
jq -e '.version == 1 and (.passwordHash | startswith("$argon2id$")) and (.sessionKey | length == 64)' "$tmp_dir/auth.json"
```

Expected: tests PASS，`jq` 返回成功；终端输出不包含明文密码。交互命令的管道测试需要 `ReadPassword` 在非 TTY 时安全回退到逐行读取，仅用于部署脚本和测试。

- [ ] **Step 6: 提交管理员初始化**

```bash
git add internal/auth internal/cli cmd/singbox-web go.mod go.sum
git commit -m "实现管理员密码初始化"
```

---

### Task 4: 实现签名会话、CSRF 与会话失效

**Files:**
- Create: `internal/auth/session.go`
- Create: `internal/auth/session_test.go`
- Modify: `internal/auth/store.go`
- Modify: `internal/auth/store_test.go`

**Interfaces:**
- Consumes: `(*auth.FileStore).Authenticate` 和 `(*auth.FileStore).Load`。
- Produces: `auth.NewSessionManager(store *auth.FileStore, now func() time.Time, random io.Reader) *auth.SessionManager`、`(*auth.SessionManager).Login(username string, password []byte) (auth.Session, error)`、`(*auth.SessionManager).Validate(token string) (auth.Claims, error)`、`(*auth.SessionManager).Refresh(token string) (auth.Session, bool, error)`；固定用户名 `auth.Username == "admin"`。

- [ ] **Step 1: 写入会话成功、篡改、过期和密码轮换测试**

Create `internal/auth/session_test.go`:

```go
package auth

import (
    "bytes"
    "path/filepath"
    "testing"
    "time"
)

func TestSessionLifecycle(t *testing.T) {
    current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
    random := bytes.NewReader(bytes.Repeat([]byte{7}, 1024))
    store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
    if err := store.SetPassword([]byte("a-strong-password")); err != nil {
        t.Fatal(err)
    }
    manager := NewSessionManager(store, func() time.Time { return current }, random)
    session, err := manager.Login("admin", []byte("a-strong-password"))
    if err != nil {
        t.Fatal(err)
    }
    claims, err := manager.Validate(session.Token)
    if err != nil || claims.Username != "admin" || claims.CSRFToken != session.CSRFToken {
        t.Fatalf("claims=%#v err=%v", claims, err)
    }
    if _, err := manager.Validate(session.Token + "x"); err == nil {
        t.Fatal("tampered token accepted")
    }
    current = current.Add(24*time.Hour + time.Second)
    if _, err := manager.Validate(session.Token); err == nil {
        t.Fatal("expired token accepted")
    }
}

func TestPasswordChangeInvalidatesExistingSession(t *testing.T) {
    current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
    random := bytes.NewReader(bytes.Repeat([]byte{9}, 2048))
    store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
    if err := store.SetPassword([]byte("first-password")); err != nil {
        t.Fatal(err)
    }
    manager := NewSessionManager(store, func() time.Time { return current }, random)
    session, err := manager.Login("admin", []byte("first-password"))
    if err != nil {
        t.Fatal(err)
    }
    if err := store.SetPassword([]byte("second-password")); err != nil {
        t.Fatal(err)
    }
    if _, err := manager.Validate(session.Token); err == nil {
        t.Fatal("pre-change session accepted")
    }
}

func TestSessionRefreshIsThrottledAndSlidesIdleExpiry(t *testing.T) {
    current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
    random := bytes.NewReader(bytes.Repeat([]byte{11}, 4096))
    store := NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
    if err := store.SetPassword([]byte("a-strong-password")); err != nil {
        t.Fatal(err)
    }
    manager := NewSessionManager(store, func() time.Time { return current }, random)
    original, err := manager.Login("admin", []byte("a-strong-password"))
    if err != nil {
        t.Fatal(err)
    }
    current = current.Add(14 * time.Minute)
    if _, renewed, err := manager.Refresh(original.Token); err != nil || renewed {
        t.Fatalf("14 minute refresh: renewed=%v err=%v", renewed, err)
    }
    current = current.Add(2 * time.Minute)
    refreshed, renewed, err := manager.Refresh(original.Token)
    if err != nil || !renewed {
        t.Fatalf("16 minute refresh: renewed=%v err=%v", renewed, err)
    }
    if refreshed.CSRFToken != original.CSRFToken || !refreshed.ExpiresAt.Equal(current.Add(24*time.Hour)) {
        t.Fatalf("refreshed=%#v original=%#v", refreshed, original)
    }
    current = original.ExpiresAt.Add(time.Second)
    if _, err := manager.Validate(original.Token); err == nil {
        t.Fatal("original idle token remained valid")
    }
    if _, err := manager.Validate(refreshed.Token); err != nil {
        t.Fatalf("active token expired: %v", err)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/auth -run 'TestSession|TestPasswordChange'
```

Expected: FAIL，提示 `NewSessionManager`、`Session` 和 `Claims` 未定义。

- [ ] **Step 3: 实现 HMAC 会话格式**

Define exact public types:

```go
const Username = "admin"

type Claims struct {
    Username  string `json:"username"`
    Epoch     uint64 `json:"epoch"`
    IssuedAt  int64  `json:"issuedAt"`
    ExpiresAt int64  `json:"expiresAt"`
    CSRFToken string `json:"csrfToken"`
}

type Session struct {
    Token     string    `json:"-"`
    CSRFToken string    `json:"csrfToken"`
    ExpiresAt time.Time `json:"expiresAt"`
}
```

Encode the token as `base64url(JSON claims) + "." + base64url(HMAC-SHA256(payload))`. `Login` must authenticate only fixed username `admin`, generate a 32-byte CSRF token, and set expiry to `now + 24h`. `Validate` must verify signature with `hmac.Equal`, reload `auth.json`, compare `Epoch`, reject expiration and return only `ErrInvalidSession` to callers。

`Refresh` first performs the same validation. If `now - IssuedAt < 15m`, return an empty `Session`、`renewed=false` and no error；otherwise issue a newly signed token with `IssuedAt=now`、`ExpiresAt=now+24h`, preserve the existing CSRF token, and return `renewed=true`. This makes the cookie a 24-hour sliding idle session without persisting per-session state or writing on every request.

- [ ] **Step 4: 运行认证测试和模糊输入测试**

Add a fuzz target that passes arbitrary tokens to `Validate` and asserts it never panics. Run:

```bash
go test ./internal/auth
go test -fuzz=FuzzValidateSession -fuzztime=5s ./internal/auth
```

Expected: PASS，无 panic。

- [ ] **Step 5: 提交会话安全基础**

```bash
git add internal/auth
git commit -m "实现管理员签名会话"
```

---

### Task 5: 嵌入默认模板并生成每档案独立凭据

**Files:**
- Create: `internal/assets/assets.go`
- Create: `internal/assets/config.default.json`
- Create: `internal/configdoc/template.go`
- Create: `internal/configdoc/template_test.go`

**Interfaces:**
- Consumes: `temp/config.default.json` 的完整内容。
- Produces: `assets.DefaultConfig []byte`、`configdoc.LoadTemplate(raw []byte) (*configdoc.Template, error)`、`(*configdoc.Template).New(random io.Reader) (json.RawMessage, error)`。

- [ ] **Step 1: 将默认模板纳入版本控制并写入失败测试**

Use `apply_patch` to add `internal/assets/config.default.json` from `temp/config.default.json`, changing only `experimental.clash_api.secret` and `experimental.cache_file.cache_id` to the literal sentinel `__GENERATE_AT_PROFILE_CREATION__`. Do not commit the source file's fixed credential-like values, and do not edit or stage the ignored source file.

Create `internal/assets/assets.go`:

```go
package assets

import _ "embed"

//go:embed config.default.json
var DefaultConfig []byte
```

Create `internal/configdoc/template_test.go`:

```go
package configdoc

import (
    "bytes"
    "encoding/json"
    "testing"

    "singbox-web/internal/assets"
)

func TestTemplateNewPreservesConfigAndRotatesCredentials(t *testing.T) {
    template, err := LoadTemplate(assets.DefaultConfig)
    if err != nil {
        t.Fatal(err)
    }
    first, err := template.New(bytes.NewReader(bytes.Repeat([]byte{1}, 128)))
    if err != nil {
        t.Fatal(err)
    }
    second, err := template.New(bytes.NewReader(bytes.Repeat([]byte{2}, 128)))
    if err != nil {
        t.Fatal(err)
    }
    var a, b map[string]any
    if err := json.Unmarshal(first, &a); err != nil {
        t.Fatal(err)
    }
    if err := json.Unmarshal(second, &b); err != nil {
        t.Fatal(err)
    }
    aClash := a["experimental"].(map[string]any)["clash_api"].(map[string]any)
    bClash := b["experimental"].(map[string]any)["clash_api"].(map[string]any)
    aCache := a["experimental"].(map[string]any)["cache_file"].(map[string]any)
    bCache := b["experimental"].(map[string]any)["cache_file"].(map[string]any)
    if aClash["secret"] == bClash["secret"] || len(aClash["secret"].(string)) != 64 {
        t.Fatalf("secrets were not independently rotated")
    }
    if aCache["cache_id"] == bCache["cache_id"] {
        t.Fatalf("cache IDs were not independently rotated")
    }
    for _, key := range []string{"log", "experimental", "inbounds", "outbounds", "route", "dns"} {
        if a[key] == nil {
            t.Fatalf("missing top-level key %q", key)
        }
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/configdoc -run TestTemplateNew
```

Expected: FAIL，提示 `LoadTemplate` 未定义。

- [ ] **Step 3: 实现模板实例化**

Implement `template.go` with `json.Decoder.UseNumber`, require a top-level object, require both embedded values to equal `__GENERATE_AT_PROFILE_CREATION__`, deep-clone through JSON, replace `experimental.clash_api.secret` with `hex.EncodeToString(32 random bytes)`, and replace `experimental.cache_file.cache_id` with `"ID_" + base64.RawURLEncoding.EncodeToString(9 random bytes)`.

The public shape must remain:

```go
type Template struct {
    raw json.RawMessage
}

func LoadTemplate(raw []byte) (*Template, error)
func (template *Template) New(random io.Reader) (json.RawMessage, error)
```

Return an error rather than creating missing `experimental.clash_api` or `experimental.cache_file` containers; a broken bundled template must fail tests and startup visibly.

- [ ] **Step 4: 验证模板语义只改变两个随机字段**

Run:

```bash
go test ./internal/configdoc
jq -S 'del(.experimental.clash_api.secret, .experimental.cache_file.cache_id)' temp/config.default.json > /tmp/template-source.json
jq -S 'del(.experimental.clash_api.secret, .experimental.cache_file.cache_id)' internal/assets/config.default.json > /tmp/template-embedded.json
diff -u /tmp/template-source.json /tmp/template-embedded.json
```

Expected: tests PASS，`diff` 无输出。

- [ ] **Step 5: 提交默认模板资源**

```bash
git add internal/assets/assets.go internal/assets/config.default.json internal/configdoc/template.go internal/configdoc/template_test.go
git commit -m "嵌入默认 sing-box 配置模板"
```

---

### Task 6: 实现配置档案元数据和创建服务

**Files:**
- Create: `internal/profile/model.go`
- Create: `internal/profile/repository.go`
- Create: `internal/profile/repository_test.go`
- Create: `internal/profile/service.go`
- Create: `internal/profile/service_test.go`

**Interfaces:**
- Consumes: `atomicfile.ReadJSON`、`atomicfile.WriteJSON`、`(*configdoc.Template).New`。
- Produces: `profile.OpenRepository(dataDir string, now func() time.Time) (*profile.Repository, error)`、`profile.NewService(repo *profile.Repository, template *configdoc.Template, random io.Reader) *profile.Service`、`(*profile.Service).Create(ctx context.Context, input profile.CreateInput) (profile.Profile, profile.Draft, error)`、`(*profile.Service).List(ctx context.Context) ([]profile.Profile, error)`、`(*profile.Service).Get(ctx context.Context, id string) (profile.Profile, error)`、`(*profile.Service).Rename(ctx context.Context, id, name string) (profile.Profile, error)`、`(*profile.Service).RemoveMetadata(ctx context.Context, id string) error`。

- [ ] **Step 1: 定义模型并写入档案持久化失败测试**

Create `internal/profile/model.go` with these exact API-facing models:

```go
package profile

import (
    "encoding/json"
    "time"
)

type ValidationSummary struct {
    Revision  uint64    `json:"revision"`
    Status    string    `json:"status"`
    Stage     string    `json:"stage"`
    Message   string    `json:"message"`
    CheckedAt time.Time `json:"checkedAt"`
}

type Profile struct {
    ID                string             `json:"id"`
    Name              string             `json:"name"`
    Slug              string             `json:"slug"`
    DraftRevision     uint64             `json:"draftRevision"`
    PublishedRevision *uint64            `json:"publishedRevision"`
    PublishedDigest   string             `json:"publishedDigest"`
    PublishedAt       *time.Time         `json:"publishedAt"`
    LastValidation    *ValidationSummary `json:"lastValidation"`
}

type EditorBuffer struct {
    Text    string `json:"text"`
    Valid   bool   `json:"valid"`
    Applied bool   `json:"applied"`
    Error   string `json:"error,omitempty"`
}

type DraftContent struct {
    Config        json.RawMessage         `json:"config"`
    EditorBuffers map[string]EditorBuffer `json:"editorBuffers"`
}

type Draft struct {
    ProfileID string       `json:"profileId"`
    Revision  uint64       `json:"revision"`
    Content   DraftContent `json:"content"`
    SavedAt   time.Time    `json:"savedAt"`
}

type CreateInput struct {
    Name string `json:"name"`
    Slug string `json:"slug"`
}
```

Create repository tests that open an empty directory, create two records, close/reopen, preserve list order, reject duplicate IDs/slugs, rename only `Name`, and reject slug values outside `^[a-z0-9][a-z0-9-]{0,62}$`.

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/profile -run 'TestRepository|TestSlug'
```

Expected: FAIL，提示 `OpenRepository` 未定义。

- [ ] **Step 3: 实现 profiles.json 仓库**

Use this disk envelope:

```go
type indexFile struct {
    Version  int       `json:"version"`
    Profiles []Profile `json:"profiles"`
}
```

`OpenRepository` must create `data-dir/drafts` with mode `0700`; if `profiles.json` is absent, atomically write `{version:1, profiles:[]}`. Protect all methods with one `sync.RWMutex`. Reject unknown schema versions and malformed IDs/slugs instead of repairing them.

Expose these exact methods:

```go
func (repo *Repository) List(ctx context.Context) ([]Profile, error)
func (repo *Repository) Get(ctx context.Context, id string) (Profile, error)
func (repo *Repository) Insert(ctx context.Context, item Profile, draft Draft) error
func (repo *Repository) Rename(ctx context.Context, id, name string) (Profile, error)
func (repo *Repository) RemoveMetadata(ctx context.Context, id string) error
func (repo *Repository) DraftPath(id string) string
```

`Insert` writes the draft first, commits the index second, and removes the new draft if the index commit fails. `RemoveMetadata` removes the index entry first and then its draft; a leftover draft is harmless and cleaned on next open.

- [ ] **Step 4: 写入配置档案服务失败测试**

Create `internal/profile/service_test.go`:

```go
func TestServiceCreateUsesTemplateAndStableSlug(t *testing.T) {
    dir := t.TempDir()
    now := func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
    repo, err := OpenRepository(dir, now)
    if err != nil {
        t.Fatal(err)
    }
    template, err := configdoc.LoadTemplate(assets.DefaultConfig)
    if err != nil {
        t.Fatal(err)
    }
    service := NewService(repo, template, bytes.NewReader(bytes.Repeat([]byte{3}, 256)))
    item, draft, err := service.Create(context.Background(), CreateInput{Name: "主配置", Slug: "primary"})
    if err != nil {
        t.Fatal(err)
    }
    if item.Slug != "primary" || item.DraftRevision != 1 || item.PublishedRevision != nil {
        t.Fatalf("profile = %#v", item)
    }
    if draft.Revision != 1 || draft.ProfileID != item.ID || len(draft.Content.Config) == 0 {
        t.Fatalf("draft = %#v", draft)
    }
}
```

- [ ] **Step 5: 实现创建、列表、读取和重命名服务**

Generate profile IDs as 16 random bytes encoded with lowercase hexadecimal. Validate `Name` after trimming to 1–80 Unicode code points. `Create` calls the template, creates revision 1, and does not touch `config-root`. Add:

```go
func (service *Service) Create(ctx context.Context, input CreateInput) (Profile, Draft, error)
func (service *Service) List(ctx context.Context) ([]Profile, error)
func (service *Service) Get(ctx context.Context, id string) (Profile, error)
func (service *Service) Rename(ctx context.Context, id, name string) (Profile, error)
func (service *Service) RemoveMetadata(ctx context.Context, id string) error
```

- [ ] **Step 6: 验证重启恢复和竞态安全**

Run:

```bash
go test ./internal/profile
go test -race ./internal/profile
```

Expected: 全部 PASS；重开仓库后档案与草稿元数据一致。

- [ ] **Step 7: 提交档案模型**

```bash
git add internal/profile
git commit -m "实现配置档案元数据存储"
```

---

### Task 7: 实现服务器端草稿修订与无效 JSON 缓冲

**Files:**
- Modify: `internal/profile/repository.go`
- Modify: `internal/profile/repository_test.go`
- Modify: `internal/profile/service.go`
- Modify: `internal/profile/service_test.go`
- Create: `internal/profile/draft.go`
- Create: `internal/profile/draft_test.go`

**Interfaces:**
- Consumes: Task 6 的 `Draft`、`DraftContent` 和档案仓库。
- Produces: `(*profile.Service).GetDraft(ctx context.Context, id string) (profile.Draft, error)`、`(*profile.Service).SaveDraft(ctx context.Context, id string, expected uint64, content profile.DraftContent) (profile.Draft, error)`、`(*profile.Service).DraftStatus(ctx context.Context, id string) profile.DraftStatus`、`profile.IsPublishable(draft profile.Draft) bool`、`profile.ErrRevisionConflict`、`profile.ErrDraftCorrupt` 和 `profile.RevisionConflict.Current`。

- [ ] **Step 1: 写入修订冲突和无效缓冲失败测试**

Create `internal/profile/draft_test.go`:

```go
package profile

import (
    "context"
    "encoding/json"
    "errors"
    "testing"
)

func TestSaveDraftRejectsStaleRevision(t *testing.T) {
    service, item := newTestServiceWithProfile(t)
    content := DraftContent{Config: json.RawMessage(`{"log":{"level":"debug"}}`), EditorBuffers: map[string]EditorBuffer{}}
    saved, err := service.SaveDraft(context.Background(), item.ID, 1, content)
    if err != nil {
        t.Fatal(err)
    }
    if saved.Revision != 2 {
        t.Fatalf("revision = %d, want 2", saved.Revision)
    }
    _, err = service.SaveDraft(context.Background(), item.ID, 1, content)
    var conflict *RevisionConflict
    if !errors.As(err, &conflict) || conflict.Current != 2 {
        t.Fatalf("error = %#v", err)
    }
}

func TestSaveDraftPersistsInvalidEditorBufferButBlocksPublish(t *testing.T) {
    service, item := newTestServiceWithProfile(t)
    content := DraftContent{
        Config: json.RawMessage(`{"log":{"level":"info"}}`),
        EditorBuffers: map[string]EditorBuffer{"log": {Text: `{"log":`, Valid: true}},
    }
    saved, err := service.SaveDraft(context.Background(), item.ID, 1, content)
    if err != nil {
        t.Fatal(err)
    }
    if saved.Content.EditorBuffers["log"].Valid || IsPublishable(saved) {
        t.Fatalf("invalid buffer was publishable: %#v", saved)
    }
}
```

The `newTestServiceWithProfile` helper must create a real temporary repository and profile from the embedded template, not a mock.

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/profile -run 'TestSaveDraft|TestGetDraft'
```

Expected: FAIL，提示 `SaveDraft`、`RevisionConflict` 和 `IsPublishable` 未定义。

- [ ] **Step 3: 实现草稿读取、规范化和乐观并发**

Add:

```go
var ErrRevisionConflict = errors.New("draft revision conflict")

type RevisionConflict struct {
    Current uint64
}

func (conflict *RevisionConflict) Error() string {
    return fmt.Sprintf("%v: current revision is %d", ErrRevisionConflict, conflict.Current)
}

func (conflict *RevisionConflict) Unwrap() error { return ErrRevisionConflict }
```

Before saving, decode `DraftContent.Config` with `UseNumber` and require exactly one top-level JSON object. Recompute every editor buffer's `Valid`、`Applied` and `Error` on the server；ignore all three client-supplied values. Valid text must decode to exactly one JSON value. Buffer keys are slash-separated safe paths such as `log`、`outbounds/0` or `route/rules/2`; reject empty、`.`、`..`、`__proto__`、`prototype` and `constructor` segments. A top-level buffer contains its one-key module object；a nested buffer contains the value at its config path. Set `Applied=true` only when the parsed text is semantically equal to the corresponding canonical config value；otherwise retain the text with fixed error `缓冲与配置对象不同步`. Invalid JSON retains its parser error class/offset but never echoes source text in the error.

Under the repository mutex, compare `expected` with both the draft file revision and `Profile.DraftRevision`, write revision `expected+1`, then update `profiles.json`. If the index update fails, restore the previous draft file before returning.

- [ ] **Step 4: 实现可发布判断和恢复测试**

`IsPublishable` returns true only when the config is a valid top-level object and all editor buffers are both valid and applied. Add tests that a syntactically valid but divergent module/nested buffer is retained with `Applied=false` and blocks publication, and that reopening after an invalid buffer preserves the text and false state.

Define the read-only isolation contract:

```go
var ErrDraftCorrupt = errors.New("draft JSON is corrupt")

type DraftStatus struct {
    ReadOnly bool   `json:"readOnly"`
    Message  string `json:"message"`
}

func (service *Service) DraftStatus(ctx context.Context, id string) DraftStatus
```

On `OpenRepository`, decode every indexed draft. A malformed or missing individual draft records an in-memory `ErrDraftCorrupt` status for that profile but does not fail repository or application startup. If a valid draft is exactly one revision ahead of `Profile.DraftRevision`, treat it as a crash between the draft rename and index commit and atomically advance only the index revision. If the profile is ahead, the gap exceeds one, IDs differ, or the draft revision is zero, isolate it as corrupt rather than guessing. `GetDraft` and `SaveDraft` return `ErrDraftCorrupt` for an isolated profile and must never rewrite its bytes；`List` and other healthy profiles remain usable. The public status message is fixed to `草稿 JSON 损坏，需要管理员在服务器修复` and never contains an absolute path or decoder input.

Add `TestCorruptDraftIsReadOnlyWithoutBlockingOtherProfiles`: create two profiles, corrupt only the first draft with `{`, reopen the repository, assert both profiles still list, the first status is read-only, both read/save operations on it return `ErrDraftCorrupt`, the corrupt bytes remain exactly `{`, and the second profile can still save a new revision. Add `TestOpenRepairsOneStepDraftCommit`: manually persist a valid revision-2 draft while metadata remains revision 1, reopen, and assert metadata becomes 2 without changing config bytes；table-test the unsafe mismatch cases above and assert they become read-only.

- [ ] **Step 5: 运行草稿测试和竞态检测**

Run:

```bash
go test ./internal/profile
go test -race ./internal/profile
```

Expected: PASS；同一 expected revision 的两个并发保存最多一个成功。

- [ ] **Step 6: 提交服务器草稿**

```bash
git add internal/profile
git commit -m "实现服务器草稿修订控制"
```

---

### Task 8: 实现顶层模块拆合、稳定摘要与脱敏

**Files:**
- Create: `internal/configdoc/modules.go`
- Create: `internal/configdoc/modules_test.go`
- Create: `internal/configdoc/redact.go`
- Create: `internal/configdoc/redact_test.go`

**Interfaces:**
- Consumes: 完整 `json.RawMessage` 配置对象。
- Produces: `configdoc.Split(config json.RawMessage) ([]configdoc.ModuleFile, error)`、`configdoc.Merge(files []configdoc.ModuleFile) (json.RawMessage, error)`、`configdoc.Digest(files []configdoc.ModuleFile) string`、`configdoc.Redact(config json.RawMessage) (json.RawMessage, error)`。

- [ ] **Step 1: 写入模块映射和往返失败测试**

Create `internal/configdoc/modules_test.go`:

```go
package configdoc

import (
    "encoding/json"
    "testing"
)

func TestSplitMergeAndDigestAreDeterministic(t *testing.T) {
    input := json.RawMessage(`{"outbounds":[{"type":"direct","tag":"direct"}],"log":{"level":"info"},"future":{"enabled":true}}`)
    files, err := Split(input)
    if err != nil {
        t.Fatal(err)
    }
    if len(files) != 3 || files[0].Name != "10-log.json" || files[1].Name != "70-outbounds.json" || files[2].Name != "98-future-ebb3de8a.json" {
        t.Fatalf("files = %#v", files)
    }
    merged, err := Merge(files)
    if err != nil {
        t.Fatal(err)
    }
    if !jsonSemanticEqual(input, merged) {
        t.Fatalf("merged = %s", merged)
    }
    again, err := Split(merged)
    if err != nil {
        t.Fatal(err)
    }
    if Digest(files) != Digest(again) {
        t.Fatal("digest changed across round trip")
    }
}
```

Unknown future top-level keys must be assigned deterministic names `98-<sanitized-key>-<first 8 SHA-256 hex of original UTF-8 key>.json`, sorted by original key. Build `sanitized-key` by lowercasing ASCII letters, replacing every run outside `[a-z0-9]` with one hyphen, trimming hyphens, truncating to 32 bytes, and using `module` when empty. The original-key hash makes truncation/sanitization collisions unable to overwrite files.

- [ ] **Step 2: 写入敏感字段脱敏失败测试**

Create `internal/configdoc/redact_test.go`:

```go
func TestRedactKeepsShapeAndMasksSensitiveValues(t *testing.T) {
    input := json.RawMessage(`{"experimental":{"clash_api":{"secret":"abc"}},"outbounds":[{"type":"vless","uuid":"u1","password":"p1","server":"example.com"}]}`)
    got, err := Redact(input)
    if err != nil {
        t.Fatal(err)
    }
    want := json.RawMessage(`{"experimental":{"clash_api":{"secret":"***REDACTED***"}},"outbounds":[{"type":"vless","uuid":"***REDACTED***","password":"***REDACTED***","server":"example.com"}]}`)
    if !jsonSemanticEqual(got, want) {
        t.Fatalf("Redact() = %s", got)
    }
}
```

- [ ] **Step 3: 运行测试确认失败**

Run:

```bash
go test ./internal/configdoc -run 'TestSplit|TestRedact'
```

Expected: FAIL，提示 `Split`、`Merge`、`Digest` 和 `Redact` 未定义。

- [ ] **Step 4: 实现固定模块映射和稳定 JSON**

Define:

```go
type ModuleFile struct {
    Name    string
    TopKey  string
    Content []byte
}
```

Use the exact mapping from the spec: `$schema`→`00-schema.json`、`log`→`10-log.json`、`dns`→`20-dns.json`、`ntp`→`30-ntp.json`、`certificate`→`40-certificate.json`、`certificate_providers`→`41-certificate-providers.json`、`http_clients`→`42-http-clients.json`、`network_namespaces`→`43-network-namespaces.json`、`endpoints`→`50-endpoints.json`、`inbounds`→`60-inbounds.json`、`outbounds`→`70-outbounds.json`、`route`→`80-route.json`、`services`→`90-services.json`、`experimental`→`99-experimental.json`。

Decode with `UseNumber`, emit each file as one-key indented JSON plus final newline, and calculate SHA-256 over repeated `name + NUL + content + NUL` in sorted file order. `Merge` must reject duplicate top-level keys and any file whose content contains zero or multiple top-level keys.

- [ ] **Step 5: 实现递归脱敏**

Mask case-insensitive exact keys `password`、`passwd`、`secret`、`token`、`access_token`、`refresh_token`、`authorization`、`api_key`、`client_secret`、`uuid`、`private_key`、`pre_shared_key` with the string `***REDACTED***`, recursively through maps and arrays. Preserve all other keys and JSON number values.

- [ ] **Step 6: 验证默认模板拆合**

Run:

```bash
go test ./internal/configdoc
go test -run TestSplitMergeAndDigestAreDeterministic -count=50 ./internal/configdoc
```

Expected: PASS；50 次摘要完全一致。

- [ ] **Step 7: 提交配置文档编译器**

```bash
git add internal/configdoc
git commit -m "实现配置模块拆合与脱敏"
```

---

### Task 9: 固定最新版 Schema 并实现 Schema/语义预检

> **稳定版 Schema 固定说明（2026-08-19）：** 目标版本固定为 sing-box `1.13.19`。官方 live URL `https://sing-box.sagernet.org/schema.json` 已提前部署 1.14 字段，不能作为 1.13.19 的发布资产；本任务改为固定官方仓库 deployment commit `583a690e888cdf6e8b024bf017ebf6297e08338d` 的根目录 `schema.json`，预期 SHA-256 为 `ee03dcaffe9282100418de280c61d3f0c8022712af50bc79c072994247133c41`。维护脚本必须拒绝其他本机版本，并 guard 1.13 字段 `dns.independent_cache` 与 remote rule-set `download_detour`，Task 17/18 继续使用这两个字段，不迁移到 live Schema 的 1.14 写法。

**Files:**
- Create: `scripts/update-singbox-schema.sh`
- Create: `internal/assets/sing-box.schema.json`
- Create: `internal/assets/schema.version.json`
- Modify: `internal/assets/assets.go`
- Create: `internal/validation/types.go`
- Create: `internal/validation/schema.go`
- Create: `internal/validation/schema_test.go`
- Create: `internal/validation/semantic.go`
- Create: `internal/validation/semantic_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: 嵌入的最新版 Schema 和完整配置 JSON。
- Produces: `validation.Issue`、`validation.Result`、`validation.NewSchemaValidator(schema []byte) (*validation.SchemaValidator, error)`、`(*validation.SchemaValidator).Validate(config json.RawMessage) []validation.Issue`、`validation.ValidateSemantics(config json.RawMessage) []validation.Issue`、`validation.NewService(schema *validation.SchemaValidator, targetVersion string, now func() time.Time) *validation.Service`、`(*validation.Service).Validate(config json.RawMessage) validation.Result`、`(*validation.Service).ValidateDraft(draft profile.Draft) validation.Result`。

- [ ] **Step 1: 创建可重复的 Schema 更新脚本**

Create `scripts/update-singbox-schema.sh` with `set -eu`. It must:

```sh
#!/bin/sh
set -eu

singbox_bin=${SING_BOX_BIN:-sing-box}
target_version=1.13.19
schema_commit=583a690e888cdf6e8b024bf017ebf6297e08338d
schema_url=https://raw.githubusercontent.com/SagerNet/sing-box/$schema_commit/schema.json
expected_sha256=ee03dcaffe9282100418de280c61d3f0c8022712af50bc79c072994247133c41
asset_dir=internal/assets
schema_temp=$(mktemp "$asset_dir/.sing-box.schema.json.XXXXXX")
metadata_temp=$(mktemp "$asset_dir/.schema.version.json.XXXXXX")
trap 'rm -f "$schema_temp" "$metadata_temp"' EXIT

version=$($singbox_bin version | sed -n '1s/.* //p')
test -n "$version"
test "$version" = "$target_version"
curl -fsSL "$schema_url" -o "$schema_temp"
jq -e . "$schema_temp" >/dev/null
sha256=$(sha256sum "$schema_temp" | sed 's/ .*//')
test "$sha256" = "$expected_sha256"
jq -e '
  . as $schema
  | ($schema["$defs"].DNS.properties.independent_cache.type == "boolean")
  and ($schema["$defs"].RuleSet.oneOf | any(.properties.type.const == "remote" and .properties.download_detour.type == "string"))
' "$schema_temp" >/dev/null
fetched_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
jq -n --arg targetVersion "$target_version" --arg schemaURL "$schema_url" --arg schemaCommit "$schema_commit" --arg sha256 "$sha256" --arg fetchedAt "$fetched_at" \
  '{targetVersion:$targetVersion,schemaURL:$schemaURL,schemaCommit:$schemaCommit,sha256:$sha256,fetchedAt:$fetchedAt}' \
  > "$metadata_temp"
mv "$schema_temp" "$asset_dir/sing-box.schema.json"
mv "$metadata_temp" "$asset_dir/schema.version.json"
```

Run only after installing the latest stable sing-box and confirming network approval:

```bash
chmod +x scripts/update-singbox-schema.sh
./scripts/update-singbox-schema.sh
```

Expected: both JSON files parse with `jq`; metadata target version equals `sing-box version` output.

Web 本身不下载、编译或调度规则资源；它只保存并发布配置。生成配置中的 remote `route.rule_set` URL 会由 sing-box 在执行检查或运行配置时按其原生行为下载，这不表示 Web 提供了规则下载能力。

- [ ] **Step 2: 写入 Schema 错误路径失败测试**

Create `internal/validation/schema_test.go` using an inline schema that requires `/log/level` to be one of `debug`, `info`, `warn`, `error`. Assert invalid input produces one issue with `Stage == "schema"` and `Path == "/log/level"`; valid input produces none.

Define shared types in `types.go`:

```go
type Issue struct {
    Stage   string `json:"stage"`
    Path    string `json:"path"`
    Code    string `json:"code"`
    Message string `json:"message"`
}

type Result struct {
    Valid            bool      `json:"valid"`
    Issues           []Issue   `json:"issues"`
    TargetVersion    string    `json:"targetVersion"`
    InstalledVersion string    `json:"installedVersion"`
    CheckedAt        time.Time `json:"checkedAt"`
}
```

- [ ] **Step 3: 写入语义引用失败测试**

Create table-driven tests covering these exact failures:

```go
tests := []struct {
    name string
    json string
    code string
    path string
}{
    {"duplicate outbound tag", `{"outbounds":[{"tag":"x","type":"direct"},{"tag":"x","type":"block"}]}`, "duplicate_tag", "/outbounds/1/tag"},
    {"missing selector member", `{"outbounds":[{"tag":"select","type":"selector","outbounds":["missing"]}]}`, "missing_outbound", "/outbounds/0/outbounds/0"},
    {"missing route final", `{"outbounds":[{"tag":"direct","type":"direct"}],"route":{"final":"missing"}}`, "missing_outbound", "/route/final"},
    {"missing DNS server", `{"dns":{"servers":[{"tag":"local","type":"udp","server":"1.1.1.1"}],"final":"missing"}}`, "missing_dns_server", "/dns/final"},
    {"missing rule set", `{"route":{"rule_set":[],"rules":[{"rule_set":["missing"],"outbound":"direct"}]},"outbounds":[{"tag":"direct","type":"direct"}]}`, "missing_rule_set", "/route/rules/0/rule_set/0"},
}
```

- [ ] **Step 4: 运行测试确认失败**

Run:

```bash
go test ./internal/validation
```

Expected: FAIL，提示 Schema 和语义校验函数未定义。

- [ ] **Step 5: 实现 Schema 校验**

Add `github.com/santhosh-tekuri/jsonschema/v5`. Compile the embedded or injected schema once in `NewSchemaValidator`. Decode instances with `UseNumber`; flatten `*jsonschema.ValidationError` causes into stable `Issue` values using `InstanceLocation` as JSON Pointer and never include the rejected field value in `Message`.

- [ ] **Step 6: 实现跨模块语义校验**

Decode a top-level object and build tag sets for inbounds, outbounds, DNS servers and `route.rule_set`. Check duplicate nonempty tags plus references in:

- selector/urltest `outbounds` members;
- outbound `detour`;
- `route.final` and route rule `outbound`;
- `route.rules[].rule_set`;
- `dns.final`、`dns.rules[].server` and DNS `domain_resolver` string/object server references;
- `route.default_domain_resolver` string/object server references.

Sort issues first by path and then code so API and tests remain deterministic.

Semantic issue messages are fixed by code (for example `引用的出站不存在`) and never interpolate the missing/duplicate tag or any rejected JSON value；the JSON Pointer path is sufficient for UI navigation.

Add `validation.Service` as the single preflight entry point. Its `Validate` concatenates Schema and semantic issues, sets `Valid` from the empty/nonempty issue list, sets the embedded target version and UTC check time, and leaves `InstalledVersion` empty because only the publication checker knows the host binary version.

`ValidateDraft` first sorts nonpublishable `EditorBuffers` by key. For `Valid=false`, emit `stage="json"`、`code="invalid_json"` and fixed message `原始 JSON 尚未成功解析`; for `Valid=true, Applied=false`, emit `stage="json"`、`code="buffer_out_of_sync"` and fixed message `原始 JSON 尚未应用到配置`. Derive an escaped JSON Pointer from the buffer key and never copy text/decoder errors into issues. If any such issue exists, return that result without Schema/semantic work；otherwise delegate to `Validate(draft.Content.Config)`. Add tests proving invalid and divergent saved buffers block preflight even though the last canonical config remains Schema-valid.

- [ ] **Step 7: 验证嵌入 Schema 和默认模板**

Extend `assets.go`:

```go
//go:embed sing-box.schema.json
var SingBoxSchema []byte

//go:embed schema.version.json
var SchemaVersion []byte
```

Run:

```bash
go test ./internal/validation
go test ./internal/configdoc
jq -e '.targetVersion != "" and (.sha256 | length == 64)' internal/assets/schema.version.json
```

Expected: all PASS。若默认模板不符合下载到的最新版 Schema，停止执行并向用户报告具体官方字段差异，不静默放宽 Schema。

- [ ] **Step 8: 提交最新版配置预检**

```bash
git add scripts/update-singbox-schema.sh internal/assets internal/validation go.mod go.sum
git commit -m "实现最新版配置预检"
```

---

### Task 10: 实现受限 sing-box version/check 执行器

**Files:**
- Create: `internal/singbox/runner.go`
- Create: `internal/singbox/runner_test.go`
- Create: `internal/singbox/sanitize.go`
- Create: `internal/singbox/sanitize_test.go`

**Interfaces:**
- Consumes: 本机 sing-box 二进制路径和配置目录。
- Produces: `singbox.NewRunner(path string, timeout time.Duration, outputLimit int) *singbox.Runner`、`(*singbox.Runner).Version(ctx context.Context) (string, error)`、`(*singbox.Runner).Check(ctx context.Context, dir string) (singbox.CheckResult, error)`、`singbox.Sanitize(text string) string`。

- [ ] **Step 1: 写入命令参数、超时和输出上限失败测试**

Create a temporary executable fixture in `runner_test.go` that records its arguments and supports three modes selected by its filename: success prints `sing-box version 1.test` or exits 0 for `check -C`; failure prints `secret=do-not-leak` to stderr and exits 1; timeout sleeps longer than the runner timeout.

Assert:

```go
func TestRunnerCheckUsesOnlyCheckAndConfigDirectory(t *testing.T) {
    binary, argsFile := writeFakeSingBox(t, "success")
    runner := NewRunner(binary, time.Second, 64*1024)
    dir := t.TempDir()
    result, err := runner.Check(context.Background(), dir)
    if err != nil || !result.Valid {
        t.Fatalf("result=%#v err=%v", result, err)
    }
    args, err := os.ReadFile(argsFile)
    if err != nil {
        t.Fatal(err)
    }
    if got, want := string(args), "check\n-C\n"+dir+"\n"; got != want {
        t.Fatalf("args=%q want=%q", got, want)
    }
}
```

Also assert timeout returns `ErrTimeout`, output truncates at exactly the configured byte cap, and errors expose sanitized output only.

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
go test ./internal/singbox
```

Expected: FAIL，提示 `NewRunner` 和 `CheckResult` 未定义。

- [ ] **Step 3: 实现无 shell 子进程和有界缓冲**

Define:

```go
type CheckResult struct {
    Valid    bool          `json:"valid"`
    Stdout   string        `json:"stdout"`
    Stderr   string        `json:"stderr"`
    Duration time.Duration `json:"duration"`
}
```

Use `exec.CommandContext(childCtx, runner.path, "check", "-C", dir)` directly, set `cmd.Dir = dir`, and attach independent capped writers to stdout/stderr. Never build a command string or invoke `/bin/sh`. Distinguish executable-not-found, timeout and nonzero exit with sentinel errors `ErrUnavailable`、`ErrTimeout`、`ErrCheckFailed`.

`Version` runs exactly `<binary> version`, caps output, and returns the first nonempty trimmed line.

- [ ] **Step 4: 实现敏感输出清理**

`Sanitize` must replace values following case-insensitive keys `password`、`passwd`、`secret`、`token`、`access_token`、`refresh_token`、`authorization`、`api_key`、`client_secret`、`uuid`、`private_key` and `pre_shared_key` in JSON-like or `key=value` text with `***REDACTED***`. It must also replace the configured data/config absolute roots before HTTP exposure; the caller passes those roots through `strings.NewReplacer` after generic sanitization.

- [ ] **Step 5: 运行执行器测试和真实只读冒烟测试**

Run:

```bash
go test ./internal/singbox
sing-box version
```

Expected: unit tests PASS；真实命令输出最新版。此任务不运行 `sing-box run`。

- [ ] **Step 6: 提交受限命令执行器**

```bash
git add internal/singbox
git commit -m "实现受限 sing-box 配置检查"
```

---

### Task 11: 实现正式配置原子发布、漂移检测与恢复

**Files:**
- Create: `internal/publication/service.go`
- Create: `internal/publication/service_test.go`
- Create: `internal/publication/filesystem.go`
- Modify: `internal/profile/model.go`
- Modify: `internal/profile/repository.go`
- Modify: `internal/profile/repository_test.go`
- Modify: `internal/profile/service.go`
- Modify: `internal/profile/service_test.go`

**Interfaces:**
- Consumes: `profile.Repository`、`profile.IsPublishable`、`configdoc.Split`、`configdoc.Digest`、`(*validation.Service).ValidateDraft`、`singbox.Runner.Version`、`singbox.Runner.Check`。
- Produces: `(*profile.Service).RecordValidation(ctx context.Context, id string, expectedDraftRevision uint64, summary profile.ValidationSummary) (profile.Profile, error)`、`publication.NewService(options publication.Options) *publication.Service`、`(*publication.Service).Publish(ctx context.Context, request publication.Request) (publication.Receipt, error)`、`(*publication.Service).ReadPublished(ctx context.Context, profileID string) (json.RawMessage, error)`、`(*publication.Service).Recover(ctx context.Context) error`、`(*publication.Service).Delete(ctx context.Context, profileID, confirmName string) error`。

- [ ] **Step 1: 扩展档案发布元数据并写入仓库测试**

Add to `profile/model.go`:

```go
type PublicationState struct {
    Revision   uint64            `json:"revision"`
    Digest     string            `json:"digest"`
    PublishedAt time.Time        `json:"publishedAt"`
    Validation ValidationSummary `json:"validation"`
}
```

Add repository methods:

```go
func (repo *Repository) RecordValidation(ctx context.Context, id string, expectedDraftRevision uint64, summary ValidationSummary) (Profile, error)
func (repo *Repository) RecordPublication(ctx context.Context, id string, expectedDraftRevision uint64, state PublicationState) (Profile, error)
func (repo *Repository) RepairPublication(ctx context.Context, id string, state *PublicationState) (Profile, error)
```

Tests must prove `RecordValidation` rejects a stale draft revision and changes only `LastValidation`；`RecordPublication` rejects a stale draft revision, sets all four publication fields plus `LastValidation` together, and `RepairPublication` changes only publication fields during startup recovery. A nil repair state clears `PublishedRevision`、digest and time while preserving draft and validation state. Each summary records the exact draft revision so the UI can distinguish a current result from a stale one.

Expose the repository operation through `profile.Service.RecordValidation` with the same arguments；HTTP handlers never receive the repository directly.

- [ ] **Step 2: 写入发布成功、检查失败和漂移失败测试**

Create `internal/publication/service_test.go` with a real temporary profile repository and fake checker:

```go
type fakeChecker struct {
    version string
    result  singbox.CheckResult
    err     error
}

func (checker fakeChecker) Version(context.Context) (string, error) {
    return checker.version, checker.err
}

func (checker fakeChecker) Check(context.Context, string) (singbox.CheckResult, error) {
    return checker.result, checker.err
}

func TestPublishCreatesStableSymlinkOnlyAfterCheck(t *testing.T) {
    fixture := newPublicationFixture(t, fakeChecker{version: "sing-box version latest", result: singbox.CheckResult{Valid: true}})
    receipt, err := fixture.service.Publish(context.Background(), Request{ProfileID: fixture.profile.ID, ExpectedRevision: 1})
    if err != nil {
        t.Fatal(err)
    }
    target, err := os.Readlink(filepath.Join(fixture.configRoot, fixture.profile.Slug))
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(target, receipt.Digest[:12]) || receipt.Revision != 1 {
        t.Fatalf("target=%q receipt=%#v", target, receipt)
    }
    assertOnlyExpectedModuleFiles(t, filepath.Join(fixture.configRoot, target))
}
```

Add tests that a failed checker creates no first symlink, records only a count/value-free failed validation summary, a failed second publish leaves the first target and publication fields unchanged, and modifying one current module file causes `ErrExternalDrift` unless `ConfirmExternalOverwrite` is true. If the stable slug path is a pre-existing regular file or a symlink outside `.generations/<profile-id>`, first publish returns `ErrPathOccupied` and later publish returns `ErrExternalDrift`; neither confirmation may replace an unmanaged path.

On a successful fixture, stat every generated directory/file and assert configured 0750/0640 modes. When the test process may change to its own current group, set `GroupID` and assert the generated entries have that numeric GID without changing the owner UID.

- [ ] **Step 3: 运行测试确认失败**

Run:

```bash
go test ./internal/profile ./internal/publication
```

Expected: profile tests initially fail on missing publication methods；publication package fails on missing service.

- [ ] **Step 4: 实现发布服务公共类型和预检**

Define:

```go
type Checker interface {
    Version(context.Context) (string, error)
    Check(context.Context, string) (singbox.CheckResult, error)
}

type Options struct {
    ConfigRoot string
    Profiles   *profile.Repository
    Preflight  *validation.Service
    Checker    Checker
    Now        func() time.Time
    Random     io.Reader
    FileMode   fs.FileMode
    DirMode    fs.FileMode
    GroupID    *int
}

type Request struct {
    ProfileID               string `json:"-"`
    ExpectedRevision        uint64 `json:"-"`
    ConfirmExternalOverwrite bool  `json:"confirmExternalOverwrite"`
}

type Receipt struct {
    ProfileID  string            `json:"profileId"`
    Revision   uint64            `json:"revision"`
    Digest     string            `json:"digest"`
    Directory  string            `json:"directory"`
    PublishedAt time.Time        `json:"publishedAt"`
    Validation validation.Result `json:"validation"`
}
```

`Publish` must load the profile and exact expected draft, call `options.Preflight.ValidateDraft`, require both that result and `profile.IsPublishable`, and return all JSON/Schema/semantic issues before writing a staging directory. A JSON/Schema/semantic failure calls `RecordValidation` with the exact revision and fixed count-only message. Call `Checker.Version` before creating staging so an unavailable binary cannot touch the formal root. After `Checker.Check`, copy the installed version into the receipt's validation result without changing the preflight target version. A checker failure records `status="invalid"`、`stage="singbox"`、`message="sing-box check 失败"` while leaving publication fields untouched and removes/syncs the staging entry before returning. On success, persist a `ValidationSummary` with the exact revision、`status="valid"`、`stage="singbox"` and fixed message `sing-box check 通过`; never persist checker stdout/stderr.

- [ ] **Step 5: 实现 generation 目录和原子符号链接切换**

Generate publication ID as `<revision>-<first 12 digest hex characters>`. Under `config-root/.generations/<profile-id>/`:

1. create `.staging-<16 random hex>` with `DirMode`;
2. write every `configdoc.ModuleFile` with `FileMode`, sync each file and the directory;
3. optionally `chown` group while preserving the service owner;
4. run `checker.Check` against staging;
5. rename staging to publication ID and sync its profile generation parent;
6. create a relative temporary symlink in `config-root`, atomically rename it over the stable slug link, and sync `config-root`;
7. call `RecordPublication`;
8. remove the old generation only after metadata succeeds, then sync the generation parent again.

If metadata fails after link switch, atomically restore the captured old link. Set an unexported test hook `service.failpoint func(stage string) error`; production constructor leaves it nil.

Before Step 1, inspect the stable slug with `Lstat`: absence is valid only for a never-published profile；an existing entry must be a relative symlink whose cleaned target is inside `.generations/<profile-id>` and whose digest matches tracked metadata. Never follow or overwrite a regular file, directory, absolute link or link belonging to another profile. `ConfirmExternalOverwrite` permits rewriting contents only through a validated managed link；it never claims an unmanaged path.

Serialize publication per profile ID. If metadata commit and subsequent link restoration both fail, set an in-memory recovery-required flag, return `ErrRecoveryRequired`, and reject every later `Publish` or `Delete` call without touching disk until `Recover` reconciles the stable link and clears the flag.

- [ ] **Step 6: 实现启动恢复和删除**

`Recover` must inspect every profile's stable link. Parse publication ID, recompute the target directory digest, and call `RepairPublication` when metadata disagrees. Reconstructed state uses the parsed revision、full recomputed digest、generation directory mtime in UTC as `PublishedAt`, and a fixed validation summary (`status="valid"`、`stage="recovery"`、`message="由磁盘发布状态恢复"`、same revision/time). If metadata says published but the stable link is absent, pass nil to clear only publication fields；a never-published profile remains unchanged. Reject links that escape `config-root/.generations/<profile-id>`. After reconciliation, delete staging directories and unreferenced generations and sync every directory whose entries changed.

`ReadPublished` resolves only the profile's validated stable link, reads its module JSON files, reconstructs `configdoc.ModuleFile` values, verifies their digest against metadata, and returns `configdoc.Merge`; it returns `ErrNotPublished` before the first publication. `Delete` requires exact display-name confirmation, removes the stable link only after validating that it belongs to this profile, removes generations for that ID, then calls `RemoveMetadata`. It must reject an empty or mismatched confirmation, return `ErrExternalDrift` without deleting an unmanaged/replaced slug path, and never follow a client-provided path.

- [ ] **Step 7: 添加故障点恢复测试**

For stages `after_generation_rename`、`after_link_swap`、`after_metadata`, inject an error or simulate process interruption, reopen repository and publication service, call `Recover`, then assert the stable link contains either the complete prior set or the complete new set. Assert no mixture of module contents and no referenced generation is deleted. Add a double-failure test where metadata commit and link rollback both fail；assert the service enters `ErrRecoveryRequired`, blocks a second publish, and permits publishing again only after successful `Recover`.

- [ ] **Step 8: 运行发布测试和竞态检测**

Run:

```bash
go test ./internal/profile ./internal/publication
go test -race ./internal/publication
```

Expected: PASS；两个并发发布请求最多一个成功，稳定链接始终指向完整目录。

- [ ] **Step 9: 提交原子发布服务**

```bash
git add internal/profile internal/publication
git commit -m "实现多文件配置原子发布"
```

---

### Task 12: 实现 capabilities、认证 API 与 HTTP 安全中间件

**Files:**
- Create: `internal/capabilities/service.go`
- Create: `internal/capabilities/service_test.go`
- Create: `internal/httpapi/server.go`
- Create: `internal/httpapi/server_test.go`
- Create: `internal/httpapi/errors.go`
- Create: `internal/httpapi/middleware.go`
- Create: `internal/httpapi/middleware_test.go`
- Create: `internal/httpapi/auth_handlers.go`
- Create: `internal/httpapi/auth_handlers_test.go`
- Modify: `internal/auth/session.go`
- Modify: `internal/auth/session_test.go`

**Interfaces:**
- Consumes: `auth.SessionManager`、`auth.FileStore`、`singbox.Runner.Version`。
- Produces: `capabilities.New(targetVersion string, checker capabilities.VersionChecker) *capabilities.Service`、`(*capabilities.Service).Snapshot(ctx context.Context) capabilities.Snapshot`、`(*capabilities.Service).Refresh(ctx context.Context) capabilities.Snapshot`；`httpapi.New(deps httpapi.Dependencies) http.Handler`、`httpapi.WithSecurityHeaders(next http.Handler) http.Handler`，先实现 health/auth/capabilities 路由。

- [ ] **Step 1: 写入能力声明失败测试**

Define the exact response model:

```go
type SingBoxInfo struct {
    Available     bool   `json:"available"`
    Version       string `json:"version"`
    TargetVersion string `json:"targetVersion"`
    Warning       string `json:"warning"`
}

type Snapshot struct {
    Capabilities map[string]bool `json:"capabilities"`
    SingBox       SingBoxInfo    `json:"singBox"`
}
```

Tests must assert keys and values exactly:

```go
want := map[string]bool{
    "config.manage": true,
    "config.publish": true,
    "subscription.sync": false,
    "ruleset.sync": false,
    "core.lifecycle": false,
    "core.metrics": false,
}
```

When `Version` returns `singbox.ErrUnavailable`, only `config.publish` becomes false and the warning is nonempty. When the installed version differs from `targetVersion`, keep `config.publish=true`, return both exact versions, and set a mismatch warning；一期仍以真实 `sing-box check` 结果作为发布决定。

- [ ] **Step 2: 实现 capabilities 服务并验证**

Implement a mutex-protected cached snapshot. `Refresh(ctx)` performs one bounded probe and returns/stores the new snapshot；`Snapshot(ctx)` returns the cache without running a process. `app.Run` calls `Refresh` once before serving；一期 exposes no automatic polling endpoint. Run:

```bash
go test ./internal/capabilities
```

Expected: PASS。

- [ ] **Step 3: 写入登录 Cookie、Origin、CSRF 和安全头失败测试**

Create `httptest` cases asserting:

- `GET /healthz` is unauthenticated and returns `{"status":"ok"}`.
- successful `POST /api/v1/auth/login` for `admin` sets cookie `sb_session` with `Path=/`、no Domain、`Secure`、`HttpOnly`、`SameSite=Strict` and returns `csrfToken` plus `expiresAt`;
- an authenticated request made 14 minutes after issue does not rewrite the Cookie；a request after 16 minutes sets a refreshed Cookie whose expiry is 24 hours from that request and whose CSRF Token is unchanged;
- `POST /api/v1/auth/logout` returns an expired `sb_session` Cookie with the same security attributes;
- `PUT /api/v1/auth/password` rotates the key/epoch, returns an expired session Cookie, and the old token immediately fails `GET /api/v1/auth/session`;
- bad username and bad password both return status 401 and code `invalid_credentials` with identical body length;
- protected API without cookie returns 401;
- state-changing API without `X-CSRF-Token` or with a foreign `Origin` returns 403;
- every response contains `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'`、`X-Content-Type-Options: nosniff` and `Referrer-Policy: no-referrer`;
- every `/api/v1/**` response, including errors, contains `Cache-Control: no-store`;
- a body larger than 2,000,000 bytes returns 413 before JSON decoding.

Use this error envelope throughout:

```go
type ErrorBody struct {
    Error APIError `json:"error"`
}

type APIError struct {
    Code            string             `json:"code"`
    Message         string             `json:"message"`
    FieldIssues     []validation.Issue `json:"fieldIssues,omitempty"`
    CurrentRevision *uint64            `json:"currentRevision,omitempty"`
}
```

- [ ] **Step 4: 运行 HTTP 测试确认失败**

Run:

```bash
go test ./internal/httpapi
```

Expected: FAIL，提示 `New`、`Dependencies` 和 handlers 未定义。

- [ ] **Step 5: 实现认证 HTTP 依赖和路由**

Define:

```go
type AuthService interface {
    Login(username string, password []byte) (auth.Session, error)
    Validate(token string) (auth.Claims, error)
    Refresh(token string) (auth.Session, bool, error)
    ChangePassword(password []byte) error
}

type CapabilityService interface {
    Snapshot(context.Context) capabilities.Snapshot
}

type Dependencies struct {
    Auth         AuthService
    Capabilities CapabilityService
    PublicOrigin string
    TrustedProxy func(net.IP) bool
    Now          func() time.Time
}
```

Use Go 1.24 `http.ServeMux` routes. Add `POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`GET /api/v1/auth/session`、`PUT /api/v1/auth/password`、`GET /api/v1/capabilities` and `GET /healthz`. After authentication, call `Refresh` once per request；when it returns `renewed=true`, overwrite `sb_session` with the refreshed token and expiry before dispatching the handler. Keep the CSRF token stable across refresh so an active SPA never needs a second token channel.

Add `SessionManager.ChangePassword(password []byte) error` delegating to `FileStore.SetPassword`, so password changes rotate all sessions.

- [ ] **Step 6: 实现安全中间件和内存限速**

Middleware order must be: panic recovery → request ID → security headers → 2,000,000-byte body limit → trusted proxy normalization → Origin check → authentication → CSRF → handler. Never trust `X-Forwarded-*` unless `RemoteAddr` is loopback or `TrustedProxy` returns true.

Expose the idempotent security-header layer as `WithSecurityHeaders` so Task 20 can wrap the combined API + embedded SPA mux. `httpapi.New` uses it for API-only tests；wrapping twice must overwrite to the same exact values rather than append duplicate headers.

Implement a bounded in-memory login limiter keyed by normalized IP and fixed username: five failures in 10 minutes produce 429 for 10 minutes; successful login clears the key. Do not persist attempts to JSON.

- [ ] **Step 7: 运行 HTTP 安全测试**

Run:

```bash
go test ./internal/auth ./internal/capabilities ./internal/httpapi
go test -race ./internal/httpapi
```

Expected: PASS，响应和日志不包含测试密码。

- [ ] **Step 8: 提交认证 API**

```bash
git add internal/auth internal/capabilities internal/httpapi
git commit -m "实现认证 API 与 HTTP 安全边界"
```

---

### Task 13: 实现档案与草稿 REST API

**Files:**
- Create: `internal/httpapi/profile_handlers.go`
- Create: `internal/httpapi/profile_handlers_test.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/server_test.go`
- Modify: `internal/httpapi/errors.go`

**Interfaces:**
- Consumes: Task 6/7 的 `profile.Service` 和 Task 12 的认证中间件。
- Produces: `/api/v1/profiles`、`/api/v1/profiles/{id}`、`/api/v1/profiles/{id}/draft`；草稿 ETag 格式固定为 `"draft-<revision>"`；每个档案响应包含只读 `directoryHint` 和 `checkCommand`。

- [ ] **Step 1: 写入档案创建、列表和重命名 API 失败测试**

Use a real temporary profile service behind an authenticated test server. Assert:

```http
POST /api/v1/profiles
Content-Type: application/json
X-CSRF-Token: <session token>

{"name":"主配置","slug":"primary"}
```

returns 201 with profile and revision-1 draft; duplicate slug returns 409 code `duplicate_slug`; a pre-existing filesystem entry at `<config-root>/<slug>` returns 409 code `slug_path_occupied` before metadata creation；`GET /api/v1/profiles` preserves creation order; `PATCH /api/v1/profiles/{id}` with `{"name":"新名称"}` changes only name and leaves slug unchanged. Every profile DTO returns `directoryHint=<config-root>/<slug>` and a shell-quoted `checkCommand=sing-box check -C <directoryHint>`, but never exposes `data-dir`.

The profile DTO also returns `draftReadOnly` and `draftError`. A healthy profile returns `false`/empty string；an individually corrupt draft remains listable, returns `true` and the fixed safe repair message, and its draft GET/PUT endpoints return 409 code `draft_corrupt` without changing the file.

- [ ] **Step 2: 写入 ETag 草稿并发 API 失败测试**

Test exact exchange:

```http
GET /api/v1/profiles/{id}/draft
→ ETag: "draft-1"

PUT /api/v1/profiles/{id}/draft
If-Match: "draft-1"
{"config":{"log":{"level":"debug"}},"editorBuffers":{}}
→ ETag: "draft-2"
```

Repeating with `If-Match: "draft-1"` must return 409:

```json
{
  "error": {
    "code": "draft_conflict",
    "message": "草稿已被其他页面更新",
    "currentRevision": 2
  }
}
```

Missing or malformed `If-Match` returns 428 code `revision_required`.

Add size-boundary cases under the 2,000,000-byte request limit: a top-level config member or editor buffer of exactly 1,000,000 UTF-8 bytes is accepted；one byte over returns 413 code `module_too_large`. The error body identifies only the module/buffer key and byte count, never its content.

- [ ] **Step 3: 运行 handler 测试确认失败**

Run:

```bash
go test ./internal/httpapi -run 'TestProfile|TestDraft'
```

Expected: FAIL，路由返回 404。

- [ ] **Step 4: 扩展 HTTP 依赖并实现档案路由**

Add:

```go
type ProfileService interface {
    Create(context.Context, profile.CreateInput) (profile.Profile, profile.Draft, error)
    List(context.Context) ([]profile.Profile, error)
    Get(context.Context, string) (profile.Profile, error)
    Rename(context.Context, string, string) (profile.Profile, error)
    GetDraft(context.Context, string) (profile.Draft, error)
    SaveDraft(context.Context, string, uint64, profile.DraftContent) (profile.Draft, error)
    DraftStatus(context.Context, string) profile.DraftStatus
    RecordValidation(context.Context, string, uint64, profile.ValidationSummary) (profile.Profile, error)
}
```

Add `Profiles ProfileService` and `ConfigRoot string` to `Dependencies`. Introduce an HTTP-only `ProfileResponse` that embeds the domain profile fields and adds `directoryHint`、`checkCommand`、`draftReadOnly` and `draftError`; build path strings exclusively from the configured root and validated immutable slug, and build draft health from `DraftStatus`. Decode requests with `DisallowUnknownFields`, enforce 2,000,000 bytes for the full body and 1,000,000 UTF-8 bytes for every top-level config value/editor buffer, map oversized modules to 413, validation errors to 422, unknown IDs to 404, duplicate slug to 409, corrupt drafts to 409 and revision conflict to the exact body above.

- [ ] **Step 5: 验证路径和任意字段防护**

Add tests that IDs such as `../../etc/passwd` return 404 without touching disk, an absolute `slug` is rejected with 422, JSON fields named `path` or `directory` in create/rename requests are rejected as unknown, and `Lstat` checks only the single validated slug child without following an existing symlink. Publication still repeats this ownership check to close the create/publish race.

- [ ] **Step 6: 运行档案 API 全量测试**

Run:

```bash
go test ./internal/httpapi
go test -race ./internal/httpapi
```

Expected: PASS。

- [ ] **Step 7: 提交档案 API**

```bash
git add internal/httpapi
git commit -m "实现配置档案与草稿 API"
```

---

### Task 14: 实现校验、发布、导出、删除 API 与 serve 装配

**Files:**
- Create: `internal/httpapi/config_handlers.go`
- Create: `internal/httpapi/config_handlers_test.go`
- Create: `internal/audit/logger.go`
- Create: `internal/audit/logger_test.go`
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/errors.go`
- Modify: `internal/httpapi/auth_handlers.go`
- Modify: `internal/httpapi/auth_handlers_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `cmd/singbox-web/deps.go`
- Modify: `cmd/singbox-web/main.go`

**Interfaces:**
- Consumes: profile、validation、publication、configdoc、auth、capabilities 和 Task 12/13 HTTP 服务。
- Produces: `/validate`、`/publish`、`/export`、档案删除；`app.Run(ctx context.Context, options app.Options) error`；完整 `singbox-web serve`。

- [ ] **Step 1: 写入校验、发布和导出 API 失败测试**

Test authenticated requests with exact semantics:

- `POST /api/v1/profiles/{id}/validate` returns Schema/semantic result, never invokes the directory checker, and atomically records a safe `lastValidation` summary for that exact draft revision whether valid or invalid.
- `POST /api/v1/profiles/{id}/publish` requires `If-Match: "draft-N"`, invokes the publication service once, and returns 200 receipt.
- publication check failure returns 422 code `singbox_check_failed` with sanitized stderr and leaves the previous digest unchanged.
- unavailable/timeout sing-box maps to 503 `singbox_unavailable` / 504 `singbox_check_timeout`, leaves the stable link unchanged, and returns only capped sanitized output.
- `GET /api/v1/profiles/{id}/export?mode=full&source=draft&download=1` returns `application/json`、`Cache-Control: no-store` and `Content-Disposition: attachment; filename="<slug>-config.json"` with the full secret.
- `GET /api/v1/profiles/{id}/export?mode=redacted&source=draft` replaces the secret and sets `Cache-Control: no-store`.
- `GET /api/v1/profiles/{id}/export?mode=full&source=published` uses `ReadPublished`, returns inline JSON without `Content-Disposition`, and returns 404 `not_published` before first publish.
- unknown export mode returns 400.
- captured logs for validate/publish/export contain operation、profile ID、revision、digest/duration when applicable and result code, but do not contain the request body, checker stdout/stderr, absolute roots or a known Secret.

- [ ] **Step 2: 写入删除确认失败测试**

Assert `DELETE /api/v1/profiles/{id}` accepts only body `{"confirmName":"当前显示名称"}`. Wrong text returns 409 `confirmation_mismatch`; success removes metadata, draft, stable link and generations. The response warning must state that一期无法判断外部核心是否正在使用该路径.

- [ ] **Step 3: 运行 handler 测试确认失败**

Run:

```bash
go test ./internal/httpapi -run 'TestValidate|TestPublish|TestExport|TestDelete'
```

Expected: FAIL，路由返回 404 或依赖接口缺失。

- [ ] **Step 4: 实现配置 HTTP 依赖和 handlers**

Add exact interfaces:

```go
type PreflightService interface {
    ValidateDraft(draft profile.Draft) validation.Result
}

type PublicationService interface {
    Publish(context.Context, publication.Request) (publication.Receipt, error)
    ReadPublished(context.Context, string) (json.RawMessage, error)
    Delete(context.Context, string, string) error
}

type Auditor interface {
    Record(audit.Event)
}
```

Define `audit.Event` with only `Operation`、`ProfileID`、`Revision`、`Digest`、`DurationMS`、`Result` and `RequestID`; do not add a free-form message, error or payload field. `audit.New(writer io.Writer, now func() time.Time)` JSON-encodes one line per event under a mutex. Tests pass secrets and absolute paths only through failed downstream results, then prove they cannot enter the event API or emitted JSON.

Add `Auditor` to `Dependencies`. The validate handler loads the current draft and calls only `PreflightService.ValidateDraft`, then creates a `profile.ValidationSummary` carrying the checked revision. A valid result stores `status="valid"`、`stage="preflight"`、`message="校验通过"`; an invalid result stores `status="invalid"`、the first issue's stage and `message="校验失败，共 N 项"`, never an issue value or process output. Call `Profiles.RecordValidation` with that same expected revision. If the draft advances before the summary commit, return 409 `draft_conflict` and leave the newer profile unchanged. Publish parses ETag into expected revision and body into `publication.Request`. Export accepts only `mode=full|redacted`、`source=draft|published` and `download=0|1`; draft source loads canonical draft, published source calls `ReadPublished`, redacted mode calls `configdoc.Redact`, every export sets `Cache-Control: no-store`, and only `download=1` adds `Content-Disposition`. Authentication and configuration handlers record only stable result codes；middleware and handlers must never pass internal error strings to the auditor.

- [ ] **Step 5: 写入 app 启动失败测试**

Create `internal/app/app_test.go` cases:

- missing `auth.json` returns an initialization error before opening a listener;
- malformed/unknown-version `auth.json` or `profiles.json` rejects startup, preserves the exact bytes, and reports only the affected path plus a safe parse/version class;
- unsafe non-loopback listen address is rejected unless `AllowPublicListen` is true;
- `PublicOrigin` must be one canonical HTTPS origin with host and no credentials/path/query/fragment；lookalike or trailing-slash values are rejected rather than normalized silently；only explicit development mode permits an HTTP loopback origin;
- a held `app.lock` rejects a second app instance;
- data-dir with group/other permission bits, config-root writable by group/other, symlink roots, `auth.json` broader than 0600, or a root not owned by the effective service user is rejected before listening;
- a single corrupt draft does not block startup；the affected profile is read-only while another profile remains editable;
- startup calls `publication.Recover` before exposing the handler;
- unavailable sing-box starts the editor API but capabilities report `config.publish=false`.

Define:

```go
type Options struct {
    Listen            string
    DataDir           string
    ConfigRoot        string
    SingBoxPath       string
    ConfigGroup       string
    AllowPublicListen bool
    Development      bool
    PublicOrigin      string
    Version           string
    LogOutput         io.Writer
}

func Run(ctx context.Context, options Options) error
```

- [ ] **Step 6: 装配后端依赖和安全 HTTP 生命周期**

`app.Run` must:

1. validate listen/public-origin rules, resolve `ConfigGroup` with `os/user.LookupGroup` and parse its numeric GID；require absolute cleaned roots and walk every existing path component with `Lstat` to reject symlinks；create data-dir as 0700 and config-root as 0750 with that group, require both roots to be owned by the effective UID, require config-root to have the configured GID, reject group/other bits on data-dir, reject group/other write bits on config-root, require `auth.json` to be a regular 0600 file, and apply the same regular-file/mode check to `profiles.json` when it already exists;
2. acquire `data-dir/app.lock`;
3. require and load `auth.json`;
4. load embedded template, Schema and schema metadata;
5. open profile repository;
6. create runner、capabilities and publication service with the resolved `GroupID`, call `capabilities.Refresh` once, then call publication `Recover`;
7. default nil `LogOutput` to `io.Discard`, create the constrained audit logger, and build `httpapi.New` with all dependencies;
8. create `http.Server` with `ReadHeaderTimeout=5s`、`ReadTimeout=15s`、`WriteTimeout=60s`、`IdleTimeout=60s`;
9. serve until context cancellation, then call `Shutdown` with 10-second timeout;
10. release the app lock last.

- [ ] **Step 7: 实现 serve CLI**

Extend `cli.ServeOptions` with `AllowPublicListen bool`、`Development bool` and `PublicOrigin string`. Parse:

```text
singbox-web serve \
  --listen 127.0.0.1:8080 \
  --data-dir /var/lib/singbox-web \
  --config-root /etc/sing-box/web-profiles \
  --sing-box /usr/local/bin/sing-box \
  --config-group sing-box \
  --public-origin https://example.com
```

`--data-dir`、`--config-root` and `--public-origin` are mandatory in production; local defaults are resolved to absolute paths only when `--dev` sets `Development=true`. Development may use an HTTP loopback origin but still may not listen publicly unless the separate danger flag is present. Parse `--allow-public-listen` as the sole mapping to `AllowPublicListen`; it is false by default and the production example never uses it. `cmd/singbox-web/deps.go` maps CLI options to `app.Options`, sets `LogOutput` to the process stderr, and never logs parsed passwords or environment values. Handle SIGINT/SIGTERM with a cancelable context.

- [ ] **Step 8: 运行后端全量验证**

Run:

```bash
go test ./...
go test -race ./internal/app ./internal/httpapi ./internal/publication
go vet ./...
```

Expected: PASS；未运行 sing-box 核心。

- [ ] **Step 9: 提交完整后端一期 API**

```bash
git add internal/audit internal/httpapi internal/app internal/cli cmd/singbox-web
git commit -m "装配一期配置管理后端"
```

---

### Task 15: 建立 Vue 应用外壳、类型化 API、登录与 capabilities

**Files:**
- Create: `web/index.html`
- Create: `web/src/main.ts`
- Create: `web/src/App.vue`
- Create: `web/src/router.ts`
- Create: `web/src/api/types.ts`
- Create: `web/src/api/client.ts`
- Create: `web/src/api/client.test.ts`
- Create: `web/src/stores/auth.ts`
- Create: `web/src/stores/auth.test.ts`
- Create: `web/src/stores/capabilities.ts`
- Create: `web/src/features/auth/LoginView.vue`
- Create: `web/src/features/auth/LoginView.test.ts`
- Create: `web/src/features/shell/AppShell.vue`
- Create: `web/src/features/shell/AppShell.test.ts`
- Create: `web/src/features/placeholders/CapabilityUnavailable.vue`
- Create: `web/src/test/setup.ts`
- Create: `web/src/styles/prototype.css`
- Create: `web/src/styles/app.css`
- Modify: `web/vite.config.ts`
- Modify: `web/package.json`
- Modify: `web/package-lock.json`

**Interfaces:**
- Consumes: Task 12–14 的 HTTP JSON、Cookie、CSRF、ETag 和 capabilities 契约；现有 `src/styles.css` 仅作为视觉源。
- Produces: `apiClient`、后端 DTO 类型、`useAuthStore`、`useCapabilitiesStore`、受保护路由和一期主外壳。

- [ ] **Step 1: 定义与后端一致的 TypeScript 契约**

Create `web/src/api/types.ts` with exact camelCase fields matching Go JSON tags:

```ts
export interface ValidationIssue {
  stage: string;
  path: string;
  code: string;
  message: string;
}

export interface APIError {
  code: string;
  message: string;
  fieldIssues?: ValidationIssue[];
  currentRevision?: number;
}

export interface SessionResponse {
  username: "admin";
  csrfToken: string;
  expiresAt: string;
}

export interface CapabilitySnapshot {
  capabilities: Record<
    | "config.manage"
    | "config.publish"
    | "subscription.sync"
    | "ruleset.sync"
    | "core.lifecycle"
    | "core.metrics",
    boolean
  >;
  singBox: {
    available: boolean;
    version: string;
    targetVersion: string;
    warning: string;
  };
}

export interface Profile {
  id: string;
  name: string;
  slug: string;
  draftRevision: number;
  publishedRevision: number | null;
  publishedDigest: string;
  publishedAt: string | null;
  lastValidation: null | {
    revision: number;
    status: string;
    stage: string;
    message: string;
    checkedAt: string;
  };
  directoryHint: string;
  checkCommand: string;
  draftReadOnly: boolean;
  draftError: string;
}

export interface EditorBuffer {
  text: string;
  valid: boolean;
  applied: boolean;
  error?: string;
}

export interface Draft {
  profileId: string;
  revision: number;
  content: {
    config: Record<string, unknown>;
    editorBuffers: Record<string, EditorBuffer>;
  };
  savedAt: string;
}

export interface ValidationResult {
  valid: boolean;
  issues: ValidationIssue[];
  targetVersion: string;
  installedVersion: string;
  checkedAt: string;
}

export interface PublicationReceipt {
  profileId: string;
  revision: number;
  digest: string;
  directory: string;
  publishedAt: string;
  validation: ValidationResult;
}
```

- [ ] **Step 2: 写入 API 客户端和登录失败测试**

Use Vitest `vi.stubGlobal("fetch", ...)` to assert:

- login sends `credentials:"same-origin"` and `Origin` is left to the browser;
- the returned CSRF token is added as `X-CSRF-Token` only on non-GET API calls;
- an ETag is returned alongside draft data;
- an API error throws typed `APIClientError` with code and current revision;
- a 401 clears in-memory auth and emits one `unauthorized` event.

Create the public shape:

```ts
export interface APIResponse<T> {
  data: T;
  etag: string | null;
}

export class APIClientError extends Error {
  constructor(public readonly status: number, public readonly detail: APIError) {
    super(detail.message);
  }
}

export class APIClient {
  setCSRFToken(token: string): void;
  clearCSRFToken(): void;
  request<T>(path: string, init?: RequestInit): Promise<APIResponse<T>>;
}

export const apiClient = new APIClient();
```

- [ ] **Step 3: 运行前端测试确认失败**

Run:

```bash
npm --prefix web run test -- src/api/client.test.ts src/stores/auth.test.ts src/features/auth/LoginView.test.ts
```

Expected: FAIL，相关模块不存在。

- [ ] **Step 4: 实现 API 客户端、认证和 capabilities store**

`useAuthStore` exposes:

```ts
interface AuthState {
  session: SessionResponse | null;
  loading: boolean;
}

async function restore(): Promise<void>;
async function login(password: string): Promise<void>;
async function logout(): Promise<void>;
async function changePassword(password: string): Promise<void>;
```

Login always sends username `admin`; the UI does not offer username editing. `restore` calls `/auth/session`, updates `apiClient` CSRF, and treats 401 as a normal logged-out state. `useCapabilitiesStore.load()` calls `/capabilities` only after authentication.

`LoginView` exposes a read-only field labelled `用户名` with value `admin`, a password input labelled `密码`, and a submit button named `登录`; clear the password ref/input after every completed attempt. `AppShell` navigation uses the accessible names `仪表盘`、`配置`、`运行状态`、`订阅`、`规则集`、`设置` and `退出登录`; component tests query these roles/names rather than CSS classes.

- [ ] **Step 5: 实现登录路由和主外壳**

Create routes:

```ts
const routes = [
  { path: "/login", name: "login", component: LoginView, meta: { public: true } },
  { path: "/", component: AppShell, children: [
    { path: "", redirect: "/dashboard" },
    { path: "dashboard", name: "dashboard", component: CapabilityUnavailable, props: { title: "配置仪表盘", description: "配置摘要将在档案功能完成后显示。" } },
    { path: "config/:profileId?/:module?", name: "config", component: CapabilityUnavailable, props: { title: "配置工作区", description: "请先创建或选择配置档案。" } },
    { path: "runtime", name: "runtime", component: CapabilityUnavailable, props: { title: "运行状态", description: "核心状态与生命周期控制将在二期接入。" } },
    { path: "subscriptions", name: "subscriptions", component: CapabilityUnavailable, props: { title: "订阅配置", description: "代理订阅同步不在一期范围内。" } },
    { path: "rulesets", name: "rulesets", component: CapabilityUnavailable, props: { title: "规则集中心", description: "规则资源下载与调度不在一期范围内。" } },
    { path: "settings", name: "settings", component: CapabilityUnavailable, props: { title: "系统设置", description: "管理员与界面设置将在后续前端任务接入。" } },
  ]},
];
```

`CapabilityUnavailable.vue` takes required string props `title` and `description`, renders them with an `尚未接入` status badge, and never renders metrics or action buttons. The router guard awaits `auth.restore()` once and redirects unauthenticated requests to `/login`.

Copy the existing `src/styles.css` into `web/src/styles/prototype.css` as the visual reference, without editing the original. Add only Vue mounting, focus and unavailable-state overrides to `app.css`.

- [ ] **Step 6: 验证登录和外壳**

Run:

```bash
npm run test:web
npm run typecheck
npm run build:web
```

Expected: PASS；Vite build succeeds；tests assert no runtime metric text such as `MB/s` appears in the shell.

- [ ] **Step 7: 提交 Vue 登录外壳**

```bash
git add web
git commit -m "实现 Vue 登录与控制台外壳"
```

---

### Task 16: 实现多档案侧栏、服务器草稿和冲突恢复

**Files:**
- Create: `web/src/stores/profiles.ts`
- Create: `web/src/stores/profiles.test.ts`
- Create: `web/src/stores/draft.ts`
- Create: `web/src/stores/draft.test.ts`
- Create: `web/src/features/profiles/ProfileSidebar.vue`
- Create: `web/src/features/profiles/ProfileSidebar.test.ts`
- Create: `web/src/features/profiles/CreateProfileDialog.vue`
- Create: `web/src/features/profiles/RenameProfileDialog.vue`
- Create: `web/src/features/config/useDraftAutosave.ts`
- Create: `web/src/features/config/useDraftAutosave.test.ts`
- Modify: `web/src/features/shell/AppShell.vue`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: `/profiles`、`/profiles/{id}/draft`、ETag `"draft-N"`、409 `draft_conflict`。
- Produces: `useProfilesStore`、`useDraftStore`、`useDraftAutosave` and profile sidebar actions；draft store is the only frontend owner of canonical config and editor buffers。

- [ ] **Step 1: 写入 profile store 失败测试**

Mock `apiClient.request` and assert:

- `load()` stores profiles in server order;
- `create({name,slug})` appends the returned profile and routes to `/config/<id>/general`;
- `rename(id,name)` changes name only;
- setting active profile fetches its draft and ETag;
- selecting a profile with `draftReadOnly=true` shows its safe repair message, does not issue a draft PUT, and disables editors without blocking healthy profiles;
- no code labels the active editor profile as “核心当前配置”.

Expose:

```ts
async function load(): Promise<void>;
async function create(input: { name: string; slug: string }): Promise<Profile>;
async function rename(id: string, name: string): Promise<void>;
async function select(id: string): Promise<void>;
```

- [ ] **Step 2: 写入草稿防抖和冲突失败测试**

Use fake timers to assert a change waits 600 ms, coalesces multiple mutations, sends the current ETag, and updates both ETag and revision after success. On 409 it must enter this exact state without retrying:

```ts
export interface DraftConflictState {
  localContent: Draft["content"];
  serverRevision: number;
}
```

`reloadAfterConflict()` discards local changes only after user confirmation. `overwriteAfterConflict()` first GETs the latest draft/ETag and then PUTs the preserved local content with that latest ETag.

- [ ] **Step 3: 运行测试确认失败**

Run:

```bash
npm --prefix web run test -- src/stores/profiles.test.ts src/stores/draft.test.ts src/features/config/useDraftAutosave.test.ts
```

Expected: FAIL，stores/composable 未定义。

- [ ] **Step 4: 实现档案和草稿 store**

`useDraftStore` state:

```ts
interface DraftState {
  draft: Draft | null;
  etag: string | null;
  status: "idle" | "loading" | "dirty" | "saving" | "saved" | "offline" | "conflict" | "error";
  lastError: APIError | null;
  conflict: DraftConflictState | null;
}
```

Expose `load(profileId)`、`replaceConfig(config)`、`setEditorBuffer(module,text)`、`save()`、`reloadAfterConflict()`、`overwriteAfterConflict()`. Keep a deep-cloned last server snapshot so network failure does not erase local content. Before load/save, consult the selected profile's `draftReadOnly`; retain status `error` with `draftError`, reject local mutation, and never attempt to heal the file from browser state.

- [ ] **Step 5: 实现档案侧栏和创建/重命名弹窗**

Reuse prototype sidebar classes. Show display name, immutable slug, draft/published revision badges and draft save status. Create requires name and matching slug; rename edits only name. Add copy buttons that use the server-provided `directoryHint` and `checkCommand` verbatim; never construct or guess an absolute server root in the browser.

Use stable accessible names: `新建档案` opens a dialog named `创建配置档案` with fields `显示名称`、`slug` and submit button `创建`; the selected profile action is `重命名档案`. The conflict dialog is named `草稿冲突` and offers `重新载入服务器版本` and `用本地内容覆盖`. Render save state in an element with `role="status"` using the exact copy from Task 19.

- [ ] **Step 6: 实现 unload 保护和离线恢复**

Register `beforeunload` only while status is `dirty`、`saving`、`offline` or `conflict`. Listen for `online` and retry once using current ETag; a 409 transitions to conflict. Never retry a publish request automatically.

- [ ] **Step 7: 验证档案与草稿交互**

Run:

```bash
npm --prefix web run test -- src/stores src/features/profiles src/features/config/useDraftAutosave.test.ts
npm run typecheck
go test ./internal/httpapi
```

Expected: PASS。

- [ ] **Step 8: 提交多档案草稿 UI**

```bash
git add web/src/stores web/src/features/profiles web/src/features/config/useDraftAutosave.ts web/src/features/config/useDraftAutosave.test.ts web/src/features/shell/AppShell.vue web/src/router.ts
git commit -m "实现多档案草稿编辑流程"
```

---

### Task 17: 实现配置工作区、通用/DNS/高级表单和 JSON 兜底

**Files:**
- Create: `web/src/features/config/model.ts`
- Create: `web/src/features/config/model.test.ts`
- Create: `web/src/features/config/modules.ts`
- Create: `web/src/features/config/ConfigWorkspace.vue`
- Create: `web/src/features/config/ConfigWorkspace.test.ts`
- Create: `web/src/features/config/components/ModuleTabs.vue`
- Create: `web/src/features/config/components/RawModuleEditor.vue`
- Create: `web/src/features/config/components/RawModuleEditor.test.ts`
- Create: `web/src/features/config/components/ValidationIssueList.vue`
- Create: `web/src/features/config/editors/GeneralEditor.vue`
- Create: `web/src/features/config/editors/GeneralEditor.test.ts`
- Create: `web/src/features/config/editors/DNSEditor.vue`
- Create: `web/src/features/config/editors/DNSEditor.test.ts`
- Create: `web/src/features/config/editors/AdvancedEditor.vue`
- Create: `web/src/features/config/editors/AdvancedEditor.test.ts`
- Modify: `web/src/stores/draft.ts`
- Modify: `web/src/styles/app.css`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: `useDraftStore` canonical config/editor buffers and backend validation issue JSON Pointer paths。
- Produces: immutable path helpers `getAtPath`、`setAtPath`、`deleteAtPath`; all module editors emit a new canonical object and never own a second configuration model。

- [ ] **Step 1: 写入未知字段无损更新失败测试**

Create `model.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { setAtPath } from "./model";

describe("setAtPath", () => {
  it("changes one known field without dropping unknown fields", () => {
    const source = {
      log: { level: "info", future_option: { enabled: true } },
      future_top_level: { value: 7 },
    };
    const result = setAtPath(source, ["log", "level"], "debug");
    expect(result).toEqual({
      log: { level: "debug", future_option: { enabled: true } },
      future_top_level: { value: 7 },
    });
    expect(result).not.toBe(source);
  });
});
```

Also test array-index replacement, deletion of an optional field, creation of a missing object container, and rejection of `__proto__`、`prototype`、`constructor` path segments.

- [ ] **Step 2: 写入 raw JSON 双状态失败测试**

Mount `RawModuleEditor` with a valid `log` object. Enter invalid text and assert it emits buffer text with `valid=false, applied=false` but does not emit `update:config`. Enter valid `{"log":{"level":"warn","future":1}}`, assert it emits the new canonical config first and then buffer state with `valid=true, applied=true`, preserving other top-level modules. Stores must send both changes in one debounced draft payload so the server never persists a valid-but-divergent pair.

Mount an `experimental` module containing secret `known-secret`. Before explicit reveal, assert the DOM contains only `***REDACTED***`, the full text/buffer is not mounted, and editing is disabled. Accept the `显示完整 JSON 并编辑` confirmation, assert the real value becomes editable, then switch modules and assert it is masked again. An invalid buffer that may contain a secret must be wholly hidden behind the same reveal gate rather than heuristically redacted.

- [ ] **Step 3: 写入结构化表单失败测试**

Tests must cover:

- General: `log.disabled`、`level`、`output`、`timestamp`.
- DNS: add/edit/remove/reorder `dns.servers` and `dns.rules`; edit `disable_cache`、`disable_expire`、`independent_cache`、`final`.
- Advanced: `experimental.clash_api` controller/UI/detour/default mode/CORS/private-network fields and masked secret; `experimental.cache_file` enabled/path/cache ID/store fake IP/RDRC timeout.

Every test starts with an unknown sibling field and asserts it remains after the form action.

- [ ] **Step 4: 运行测试确认失败**

Run:

```bash
npm --prefix web run test -- src/features/config
```

Expected: FAIL，model/components 不存在。

- [ ] **Step 5: 实现不可变配置模型和模块目录**

Define module descriptors matching backend keys and filenames:

```ts
export interface ModuleDescriptor {
  key: string;
  label: string;
  shortLabel: string;
  group: "common" | "network" | "advanced";
}

export const modules: ModuleDescriptor[] = [
  { key: "general", label: "配置概览", shortLabel: "概览", group: "common" },
  { key: "dns", label: "DNS", shortLabel: "DNS", group: "network" },
  { key: "inbounds", label: "入站", shortLabel: "入站", group: "network" },
  { key: "outbounds", label: "出站", shortLabel: "出站", group: "network" },
  { key: "endpoints", label: "端点", shortLabel: "端点", group: "network" },
  { key: "routing", label: "路由", shortLabel: "路由", group: "network" },
  { key: "services", label: "服务", shortLabel: "服务", group: "advanced" },
  { key: "advanced", label: "高级", shortLabel: "高级", group: "advanced" },
];
```

`general` maps to `log`; `routing` maps to `route`; `advanced` owns `$schema`、`ntp`、`certificate`、`certificate_providers`、`http_clients`、`network_namespaces` and `experimental`.

The draft store owns buffer synchronization: a structured mutation that touches a valid/applied buffer regenerates that buffer's formatted text in the same mutation；an invalid/unapplied buffer disables structured controls for its overlapping path until the user chooses `放弃未应用 JSON` in a confirmation dialog, which replaces the text from canonical config and marks it valid/applied. This prevents autosave from creating a canonical/buffer mismatch.

Replace only the config route's intermediate `CapabilityUnavailable` with `ConfigWorkspace`; leave dashboard, subscriptions, rulesets and settings on truthful unavailable views until Task 19.

- [ ] **Step 6: 实现 RawModuleEditor 和错误导航**

Use a plain monospace `<textarea>` to avoid Monaco-size dependencies. Format only valid JSON through `JSON.stringify(value, null, 2)`. Preserve invalid text from `editorBuffers`. Detect case-insensitive exact keys `password`、`passwd`、`secret`、`token`、`access_token`、`refresh_token`、`authorization`、`api_key`、`client_secret`、`uuid`、`private_key` and `pre_shared_key`. Sensitive modules start as a read-only redacted preview；only an explicit confirmation mounts the real canonical/buffer text, and leaving the module or five minutes without input clears the reveal state. Never place the full text in a hidden DOM node. `ValidationIssueList` converts JSON Pointer first segment to a module route and focuses the closest element with matching `data-json-path`.

- [ ] **Step 7: 实现通用、DNS 和高级表单**

Use controlled inputs and `setAtPath`; numeric fields must remain numbers or be deleted when optional and empty. Secret inputs render `type="password"` by default, expose a temporary reveal button, and never copy values into toast/log text.

The Clash API secret control has accessible label `Clash API Secret`; its reveal action is named `临时显示 Clash API Secret` and re-masks on blur or after 30 seconds.

The General editor labels the log-level control `日志级别`; `ModuleTabs` uses `role="tablist"`/`role="tab"` with each descriptor's `shortLabel`; the action `编辑<模块标签> JSON` reveals a raw editor with accessible name `<模块标签> JSON`; validation issue links use `转到字段 <JSON Pointer>`. These names are the browser-test contract and must not depend on generated IDs or CSS selectors.

- [ ] **Step 8: 验证配置工作区**

Run:

```bash
npm --prefix web run test -- src/features/config
npm run typecheck
```

Expected: PASS；未知字段测试全部通过；invalid JSON survives component remount through draft store。

- [ ] **Step 9: 提交基础配置编辑器**

```bash
git add web/src/features/config web/src/stores/draft.ts web/src/styles/app.css
git commit -m "实现配置工作区与 JSON 兜底编辑"
```

---

### Task 18: 实现入站、出站、路由和低频模块编辑

**Files:**
- Create: `web/src/features/config/editors/InboundsEditor.vue`
- Create: `web/src/features/config/editors/InboundsEditor.test.ts`
- Create: `web/src/features/config/editors/OutboundsEditor.vue`
- Create: `web/src/features/config/editors/OutboundsEditor.test.ts`
- Create: `web/src/features/config/editors/RouteEditor.vue`
- Create: `web/src/features/config/editors/RouteEditor.test.ts`
- Create: `web/src/features/config/editors/CollectionJSONEditor.vue`
- Create: `web/src/features/config/editors/CollectionJSONEditor.test.ts`
- Create: `web/src/features/config/references.ts`
- Create: `web/src/features/config/references.test.ts`
- Modify: `web/src/features/config/ConfigWorkspace.vue`
- Modify: `web/src/features/config/modules.ts`

**Interfaces:**
- Consumes: Task 17 canonical path helpers and draft store。
- Produces: structured common sing-box object editors with per-object full JSON fallback；`collectReferences(config)` returns existing tag sets and missing-reference annotations without mutating config。

- [ ] **Step 1: 写入入站编辑失败测试**

Test structured round trips for:

- `mixed`: `tag`、`listen`、`listen_port`、`tcp_fast_open`、`tcp_multi_path`、`udp_fragment`.
- `tun`: `tag`、`interface_name`、`address[]`、`mtu`、`stack`、`auto_route`、`strict_route`、`endpoint_independent_nat`.
- custom object: paste any valid latest-version inbound JSON, preserve every key, and derive list title from `tag` then `type`.

Deleting or reordering an item must preserve untouched objects byte-semantically after parse/stringify.

- [ ] **Step 2: 写入出站和引用失败测试**

Test:

- `direct` and `block` with tag;
- `selector` and `urltest` with ordered `outbounds` references, `interrupt_exist_connections`, plus URL/interval/tolerance for urltest;
- custom full outbound JSON for protocols not structurally supported;
- missing references are shown as errors but never removed automatically;
- duplicate tags are highlighted using client-side preview while backend remains authoritative.

- [ ] **Step 3: 写入路由/规则集失败测试**

Test ordered add/edit/remove/reorder for `route.rules` and `route.rule_set`; common rule fields include `action`、`outbound`、`invert` and one condition payload. Test top-level `auto_detect_interface`、`find_process`、`default_interface`、`default_domain_resolver` and `final`. Remote rule sets support `tag`、`type`、`format`、`url`、`download_detour`、`update_interval`; local/inline variants fall back to full object JSON.

- [ ] **Step 4: 运行测试确认失败**

Run:

```bash
npm --prefix web run test -- InboundsEditor OutboundsEditor RouteEditor CollectionJSONEditor references
```

Expected: FAIL，editors 不存在。

- [ ] **Step 5: 实现集合编辑基础和对象级 JSON 兜底**

`CollectionJSONEditor` accepts `modelValue: unknown[]` and emits immutable arrays. Each row can switch between a supported structured editor and full JSON text. Invalid object text is retained with `valid=false, applied=false` in an index-scoped editor buffer such as `outbounds/0` or `route/rules/2`; valid text updates the array and marks the same buffer valid/applied in one draft mutation. Reorder/delete operations remap affected buffer keys in that same mutation. Buffer keys never enter sing-box objects or exported JSON, and any unapplied buffer blocks publication.

- [ ] **Step 6: 实现入站和出站编辑器**

Use exact structured fields from Steps 1–2. Adding an item starts with the minimum object:

```ts
const defaults = {
  mixed: { type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 7890 },
  tun: { type: "tun", tag: "tun-in", interface_name: "singtun0", address: ["172.19.0.1/30"], auto_route: true, strict_route: true },
  direct: { type: "direct", tag: "direct" },
  block: { type: "block", tag: "block" },
  selector: { type: "selector", tag: "selector", outbounds: [] },
  urltest: { type: "urltest", tag: "urltest", outbounds: [], url: "https://www.gstatic.com/generate_204", interval: "3m", tolerance: 150 },
};
```

Do not add defaults to existing imported/raw objects unless the user edits that field.

- [ ] **Step 7: 实现路由和低频模块编辑器**

Route rules preserve array order. Provide structured condition selection for inbound/network/protocol/domain/domain_suffix/ip_cidr/rule_set/clash_mode and actions route/reject/hijack-dns/sniff/resolve; any unsupported combination uses full rule JSON.

Use `CollectionJSONEditor` for `endpoints` and `services`. Expose raw module JSON for `ntp`、`certificate`、`certificate_providers`、`http_clients` and `network_namespaces`; no field is hidden from raw editing.

- [ ] **Step 8: 验证所有模块与默认模板**

Run:

```bash
npm --prefix web run test -- src/features/config
npm run typecheck
npm run build:web
```

Expected: PASS；load default template → edit one log field → serialize keeps all DNS/outbound/route unknown fields。

- [ ] **Step 9: 提交完整配置表单**

```bash
git add web/src/features/config
git commit -m "完善入站出站与路由编辑"
```

---

### Task 19: 实现校验发布、差异导出、删除、仪表盘与一期占位

**Files:**
- Create: `web/src/stores/publication.ts`
- Create: `web/src/stores/publication.test.ts`
- Create: `web/src/features/publication/PublicationBar.vue`
- Create: `web/src/features/publication/PublicationBar.test.ts`
- Create: `web/src/features/publication/ModuleDiff.vue`
- Create: `web/src/features/publication/ModuleDiff.test.ts`
- Create: `web/src/features/profiles/DeleteProfileDialog.vue`
- Create: `web/src/features/profiles/DeleteProfileDialog.test.ts`
- Create: `web/src/features/dashboard/DashboardView.vue`
- Create: `web/src/features/dashboard/DashboardView.test.ts`
- Modify: `web/src/features/placeholders/CapabilityUnavailable.vue`
- Create: `web/src/features/placeholders/SubscriptionPlaceholderView.vue`
- Create: `web/src/features/placeholders/RulesetPlaceholderView.vue`
- Create: `web/src/features/placeholders/RuntimePlaceholderView.vue`
- Create: `web/src/features/placeholders/RuntimePlaceholder.test.ts`
- Create: `web/src/features/settings/SettingsView.vue`
- Create: `web/src/features/settings/SettingsView.test.ts`
- Modify: `web/src/features/config/ConfigWorkspace.vue`
- Modify: `web/src/features/profiles/ProfileSidebar.vue`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: validate/publish/export/delete APIs、draft ETag、capabilities and current/published merged JSON。
- Produces: explicit preflight/publish actions, nonpersistent module diff, full/redacted export, irreversible profile deletion and no-fake-data product shell。

- [ ] **Step 1: 写入发布 store 失败测试**

Assert:

- `validate()` calls `/validate` and maps field issues without changing draft status;
- `publish()` first flushes autosave, requires current ETag, never retries automatically, then updates profile `publishedRevision` and digest;
- capability `config.publish=false` disables publish and shows `singBox.warning`;
- a failed check preserves draft and displays sanitized server message;
- full export requires an explicit confirmation callback returning true;
- redacted export never invokes full mode.

Expose:

```ts
async function validate(): Promise<ValidationResult>;
async function publish(confirmExternalOverwrite?: boolean): Promise<PublicationReceipt>;
async function downloadFull(confirm: () => Promise<boolean>): Promise<void>;
async function copyRedacted(): Promise<void>;
```

- [ ] **Step 2: 写入临时模块差异失败测试**

Given draft and published objects, `ModuleDiff` must report top-level keys as `added`、`changed`、`removed` or `unchanged`, compare semantic stable JSON, and never persist the prior body after a successful publish. Secret values must not render in the diff; changed sensitive paths show only `已修改敏感字段`.

- [ ] **Step 3: 写入无模拟数据和删除确认失败测试**

Assert dashboard renders profile count、draft status、latest validation、target/installed versions, but does not render `MB/s`、fake connection counts or start/restart buttons. Subscription/ruleset/runtime views render `CapabilityUnavailable` based on capability keys. Delete dialog enables only when typed text exactly matches current display name and warns that external core usage is unknown.

- [ ] **Step 4: 运行测试确认失败**

Run:

```bash
npm --prefix web run test -- publication Dashboard RuntimePlaceholder DeleteProfile Settings
```

Expected: FAIL，stores/components 不存在。

- [ ] **Step 5: 实现发布、导出和差异 UI**

`PublicationBar` labels states exactly as `草稿保存中`、`草稿已保存`、`有冲突`、`与磁盘一致`、`有待发布修改`. Rename the prototype action to `校验并写入`; never use `保存并应用` in一期.

Expose buttons with exact accessible names `仅校验`、`校验并写入`、`复制脱敏配置` and `下载完整配置`. Full download calls `window.confirm("完整配置包含密码、Token、Secret、UUID 和私钥，确认下载？")` before issuing the request；the redacted copy announces `脱敏配置已复制` through `role="status"`.

After preflight, announce exactly `校验通过` or `校验失败，共 N 项`; after a successful publication announce `正式配置已写入`. These messages also use `role="status"` and contain no checker output or secret value.

Fetch the published merged config only for diff display; backend must support `GET /api/v1/profiles/{id}/export?mode=full&source=published` for authenticated inline JSON with `Cache-Control: no-store`, returning 404 before first publish. Default `source=draft`; downloads use `download=1` to add `Content-Disposition`.

- [ ] **Step 6: 实现档案删除和设置**

Delete sends `confirmName`, removes the route/store entry on success, and routes to the next profile or dashboard. Settings supports theme preference in localStorage and password change through `/auth/password`; a successful password change clears stores and routes to login because all sessions are invalidated.

The password form uses fields `新密码` and `确认新密码` plus button `修改管理员密码`. The E2E suite changes to a temporary password, proves the old session is gone, then changes back to the fixture password so other tests remain isolated.

- [ ] **Step 7: 实现仪表盘和能力占位**

Replace all prototype demo runtime cards with configuration health cards or `CapabilityUnavailable`. Keep navigation destinations and explanatory copy for二期. Route rule-set configuration remains under config workspace; the standalone ruleset hub is disabled. No timer may synthesize latency, traffic or service state.

- [ ] **Step 8: 验证完整前端**

Run:

```bash
npm run test:web
npm run typecheck
npm run build:web
rg -n '8\.72|MB/s|206 ms|核心重启完成|配置已保存并应用' web/src && exit 1 || true
```

Expected: tests/build PASS；`rg` 无输出。

- [ ] **Step 9: 提交一期完整前端**

```bash
git add web/src
git commit -m "完成一期配置发布前端"
```

---

### Task 20: 嵌入 Vue 产物并添加可复现构建和生产部署文件

**Files:**
- Create: `internal/webui/dist/placeholder.txt`
- Create: `internal/webui/embed.go`
- Create: `internal/webui/handler.go`
- Create: `internal/webui/handler_test.go`
- Create: `scripts/build.sh`
- Create: `deploy/caddy/Caddyfile.example`
- Create: `deploy/systemd/singbox-web.service`
- Modify: `web/vite.config.ts`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: built Vue SPA and `httpapi.New`。
- Produces: `webui.Handler() http.Handler`、single-binary build at `dist/singbox-web`、Caddy/systemd production examples。

- [ ] **Step 1: 写入 SPA handler 失败测试**

Create an in-memory `fstest.MapFS` with `index.html` and `assets/app.js`. Assert:

- `/assets/app.js` returns the exact asset and immutable cache header;
- `/config/id/general` falls back to `index.html` with `Cache-Control: no-cache`;
- the combined app handler adds the exact CSP/nosniff/referrer headers to index and asset responses without changing their cache policy;
- missing file with a dot such as `/assets/missing.js` returns 404, not index;
- `/placeholder.txt` always returns 404 and the placeholder is never linked from `index.html`;
- `/api/v1/unknown` remains API 404 and is never served by SPA fallback;
- path traversal attempts return 400/404.

Expose:

```go
func NewHandler(files fs.FS) http.Handler
func Handler() http.Handler
```

- [ ] **Step 2: 运行 handler 测试确认失败**

Run:

```bash
go test ./internal/webui
```

Expected: FAIL，package/function 不存在。

- [ ] **Step 3: 实现嵌入和 SPA fallback**

Create `placeholder.txt` so clean checkout compiles. Use:

```go
//go:embed all:dist
var embedded embed.FS

func Handler() http.Handler {
    files, err := fs.Sub(embedded, "dist")
    if err != nil {
        panic(err)
    }
    return NewHandler(files)
}
```

Configure Vite `build.outDir` to `../internal/webui/dist` and explicit `emptyOutDir: true`. Add a small `closeBundle` plugin that rewrites `placeholder.txt` with the exact tracked content `singbox-web embedded UI build sentinel\n` after every successful bundle. This removes stale hashed assets while leaving a clean Git worktree and a compileable clean checkout. `NewHandler` reserves `placeholder.txt` as a nonservable build sentinel. Mount API routes before SPA handler in `app.Run`, then wrap the combined mux once with `httpapi.WithSecurityHeaders` so HTML/assets receive the same CSP/nosniff/referrer policy while API responses retain `no-store`.

Use this plugin shape in `web/vite.config.ts` (merge it with the existing Vue/Vitest configuration):

```ts
import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const embeddedOutDir = resolve(import.meta.dirname, "../internal/webui/dist");

const keepEmbedSentinel = {
  name: "keep-embed-sentinel",
  async closeBundle() {
    await writeFile(resolve(embeddedOutDir, "placeholder.txt"), "singbox-web embedded UI build sentinel\n", "utf8");
  },
};

// in defineConfig:
// plugins: [vue(), keepEmbedSentinel]
// build: { outDir: embeddedOutDir, emptyOutDir: true }
```

- [ ] **Step 4: 创建可复现单文件构建脚本**

Create executable `scripts/build.sh`:

```sh
#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
npm ci --prefix web
npm run build:web
go test ./...
mkdir -p dist
version=$(git rev-parse --short HEAD)
CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$version" -o dist/singbox-web ./cmd/singbox-web
```

Vite clears only its explicit output directory and recreates the tracked sentinel；the script disables VCS build metadata and records the exact commit through `-X`. Reproducibility is checked with the concrete two-build commands in Step 7.

- [ ] **Step 5: 创建 systemd 单元**

`deploy/systemd/singbox-web.service` must include:

```ini
[Unit]
Description=sing-box Web configuration manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=singbox-web
Group=singbox-web
SupplementaryGroups=sing-box
ExecStart=/usr/local/bin/singbox-web serve --listen 127.0.0.1:8080 --data-dir /var/lib/singbox-web --config-root /etc/sing-box/web-profiles --sing-box /usr/local/bin/sing-box --config-group sing-box --public-origin https://singbox.example.com
Restart=on-failure
RestartSec=3s
UMask=0027
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/singbox-web /etc/sing-box/web-profiles
RestrictSUIDSGID=true
LockPersonality=true

[Install]
WantedBy=multi-user.target
```

Do not add network/TUN capabilities or run as root.

- [ ] **Step 6: 创建 Caddy 示例**

`deploy/caddy/Caddyfile.example`:

```caddyfile
singbox.example.com {
    request_body {
        max_size 2MB
    }
    header {
        Strict-Transport-Security "max-age=31536000"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "no-referrer"
    }
    reverse_proxy 127.0.0.1:8080
}
```

Caddy does not add a second authentication layer; application login remains authoritative.

- [ ] **Step 7: 验证构建、静态路由和部署语法**

Run:

```bash
chmod +x scripts/build.sh
./scripts/build.sh
first_sha=$(sha256sum dist/singbox-web | sed 's/ .*//')
./scripts/build.sh
second_sha=$(sha256sum dist/singbox-web | sed 's/ .*//')
test "$first_sha" = "$second_sha"
./dist/singbox-web version
go test ./internal/webui ./internal/app
caddy adapt --config deploy/caddy/Caddyfile.example --pretty >/dev/null
systemd-analyze verify deploy/systemd/singbox-web.service
```

Expected: two SHA-256 values match，build/tests PASS，binary 输出版本，Caddy configuration adapts successfully，and systemd reports no unknown/invalid directives. A host without the production `singbox-web` user or `/usr/local/bin/singbox-web` may emit only those two prerequisite warnings；any other systemd diagnostic fails the step. 安装/运行 Caddy 需要用户批准；本步骤不启动长期服务。

- [ ] **Step 8: 提交单文件部署能力**

```bash
git add internal/webui internal/app scripts/build.sh deploy web/vite.config.ts .gitignore
git commit -m "添加单文件构建与生产部署"
```

---

### Task 21: 完成真实契约、端到端、安全回归和交付文档

**Files:**
- Create: `tests/contract/config_contract_test.go`
- Create: `tests/contract/testdata/expected-modules.json`
- Create: `web/playwright.config.ts`
- Create: `web/e2e/auth-config-publish.spec.ts`
- Create: `web/e2e/capability-placeholders.spec.ts`
- Create: `scripts/e2e-server.sh`
- Modify: `web/package.json`
- Modify: `web/package-lock.json`
- Modify: `README.md:1-44`

**Interfaces:**
- Consumes: complete binary、latest sing-box、Caddy、default template and all public HTTP/UI contracts。
- Produces: finite real-binary contract tests, browser E2E suite and production/developer handoff documentation。

- [ ] **Step 1: 写入真实最新版 sing-box 契约测试**

Create `tests/contract/testdata/expected-modules.json`:

```json
[
  "10-log.json",
  "20-dns.json",
  "60-inbounds.json",
  "70-outbounds.json",
  "80-route.json",
  "99-experimental.json"
]
```

Create `tests/contract/config_contract_test.go`. Only the real-binary test is guarded by `SING_BOX_BIN`; normal unit runs skip it with an explicit message, while the module round-trip always runs:

```go
package contract_test

import (
    "bytes"
    "context"
    "encoding/json"
    "os"
    "path/filepath"
    "reflect"
    "strings"
    "testing"
    "time"

    "singbox-web/internal/assets"
    "singbox-web/internal/configdoc"
    "singbox-web/internal/singbox"
)

type schemaMetadata struct {
    TargetVersion string `json:"targetVersion"`
}

func readSchemaMetadata(t *testing.T) schemaMetadata {
    t.Helper()
    var metadata schemaMetadata
    if err := json.Unmarshal(assets.SchemaVersion, &metadata); err != nil {
        t.Fatal(err)
    }
    if metadata.TargetVersion == "" {
        t.Fatal("schema metadata has empty targetVersion")
    }
    return metadata
}

func newTemplateConfig(t *testing.T) json.RawMessage {
    t.Helper()
    template, err := configdoc.LoadTemplate(assets.DefaultConfig)
    if err != nil {
        t.Fatal(err)
    }
    config, err := template.New(bytes.NewReader(bytes.Repeat([]byte{5}, 128)))
    if err != nil {
        t.Fatal(err)
    }
    return config
}

func writeModuleFixture(t *testing.T, files []configdoc.ModuleFile) string {
    t.Helper()
    dir := t.TempDir()
    for _, file := range files {
        if filepath.Base(file.Name) != file.Name {
            t.Fatalf("unsafe module name %q", file.Name)
        }
        if err := os.WriteFile(filepath.Join(dir, file.Name), file.Content, 0o600); err != nil {
            t.Fatal(err)
        }
    }
    return dir
}

func decodeJSON(t *testing.T, raw []byte) any {
    t.Helper()
    decoder := json.NewDecoder(bytes.NewReader(raw))
    decoder.UseNumber()
    var value any
    if err := decoder.Decode(&value); err != nil {
        t.Fatal(err)
    }
    return value
}

func TestDefaultTemplatePassesPinnedLatestSingBox(t *testing.T) {
    binary := os.Getenv("SING_BOX_BIN")
    if binary == "" {
        t.Skip("set SING_BOX_BIN for release contract verification")
    }
    metadata := readSchemaMetadata(t)
    runner := singbox.NewRunner(binary, 30*time.Second, 64*1024)
    version, err := runner.Version(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    fields := strings.Fields(version)
    if len(fields) == 0 || fields[len(fields)-1] != metadata.TargetVersion {
        t.Fatalf("installed=%q target=%q", version, metadata.TargetVersion)
    }
    config := newTemplateConfig(t)
    files, err := configdoc.Split(config)
    if err != nil {
        t.Fatal(err)
    }
    dir := writeModuleFixture(t, files)
    result, err := runner.Check(context.Background(), dir)
    if err != nil || !result.Valid {
        t.Fatalf("result=%#v err=%v", result, err)
    }
}

func TestDefaultTemplateModuleContract(t *testing.T) {
    expectedRaw, err := os.ReadFile("testdata/expected-modules.json")
    if err != nil {
        t.Fatal(err)
    }
    var expected []string
    if err := json.Unmarshal(expectedRaw, &expected); err != nil {
        t.Fatal(err)
    }
    config := newTemplateConfig(t)
    files, err := configdoc.Split(config)
    if err != nil {
        t.Fatal(err)
    }
    actual := make([]string, len(files))
    for index, file := range files {
        actual[index] = file.Name
    }
    if !reflect.DeepEqual(actual, expected) {
        t.Fatalf("modules=%q expected=%q", actual, expected)
    }
    merged, err := configdoc.Merge(files)
    if err != nil {
        t.Fatal(err)
    }
    if !reflect.DeepEqual(decodeJSON(t, merged), decodeJSON(t, config)) {
        t.Fatalf("merged config differs from template\nmerged=%s\ntemplate=%s", merged, config)
    }
}
```

Run `go test ./tests/contract` now. Expected: module contract PASS；real-binary contract SKIP with the exact environment hint until the latest sing-box is installed.

- [ ] **Step 2: 安装 Playwright 并写入浏览器失败测试**

Run with network approval:

```bash
npm install --prefix web --save-dev @playwright/test@1
npm --prefix web exec -- playwright install chromium
```

Add `"test:e2e": "playwright test"` to `web/package.json`. Create `web/e2e/auth-config-publish.spec.ts` with this executable flow, using only the accessible-name contracts established in Tasks 15–19:

```ts
import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";

const fixturePassword = "phase1-e2e-password";
const temporaryPassword = "phase1-e2e-password-2";

async function login(page: Page, password = fixturePassword): Promise<void> {
  await page.goto("/login");
  await expect(page.getByLabel("用户名")).toHaveValue("admin");
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
}

async function waitForDraft(page: Page): Promise<void> {
  await expect(page.getByText("草稿已保存", { exact: true }).last()).toBeVisible();
}

test("login, draft conflict, validation, publication and secret-safe export", async ({ page, context }) => {
  test.setTimeout(120_000);
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await login(page);

  const cookies = await context.cookies();
  const session = cookies.find((cookie) => cookie.name === "sb_session");
  expect(session).toMatchObject({ secure: true, httpOnly: true, sameSite: "Strict" });

  const health = await page.request.get("/healthz");
  expect(health.headers()["strict-transport-security"]).toContain("max-age=31536000");
  expect(health.headers()["content-security-policy"]).toBeTruthy();

  await page.getByRole("link", { name: "配置" }).click();
  await page.getByRole("button", { name: "新建档案" }).click();
  const createDialog = page.getByRole("dialog", { name: "创建配置档案" });
  await createDialog.getByLabel("显示名称").fill("主配置");
  await createDialog.getByLabel("slug").fill("primary");
  await createDialog.getByRole("button", { name: "创建" }).click();
  await expect(page).toHaveURL(/\/config\/[a-f0-9]{32}\/general$/);

  await page.getByRole("tab", { name: "高级" }).click();
  await expect(page.getByLabel("Clash API Secret")).toHaveAttribute("type", "password");
  await page.getByRole("tab", { name: "概览" }).click();
  await expect(page.getByLabel("日志级别")).toHaveValue("info");

  const secondPage = await context.newPage();
  await secondPage.goto(page.url());
  await expect(secondPage.getByLabel("日志级别")).toHaveValue("info");
  await page.getByLabel("日志级别").selectOption("debug");
  await waitForDraft(page);
  await secondPage.getByLabel("日志级别").selectOption("warn");
  const conflict = secondPage.getByRole("dialog", { name: "草稿冲突" });
  await expect(conflict).toBeVisible();
  await conflict.getByRole("button", { name: "重新载入服务器版本" }).click();
  await expect(secondPage.getByLabel("日志级别")).toHaveValue("debug");
  await secondPage.close();

  await page.reload();
  await expect(page.getByLabel("日志级别")).toHaveValue("debug");
  await page.getByRole("button", { name: "编辑配置概览 JSON" }).click();
  const rawGeneral = page.getByRole("textbox", { name: "配置概览 JSON" });
  await rawGeneral.fill('{"log":');
  await waitForDraft(page);
  await page.reload();
  await page.getByRole("button", { name: "编辑配置概览 JSON" }).click();
  await expect(page.getByRole("textbox", { name: "配置概览 JSON" })).toHaveValue('{"log":');
  await page.getByRole("textbox", { name: "配置概览 JSON" }).fill('{"log":{"level":"invalid"}}');
  await waitForDraft(page);
  await page.getByRole("button", { name: "仅校验" }).click();
  const issue = page.getByRole("link", { name: "转到字段 /log/level" });
  await expect(issue).toBeVisible();
  await issue.click();
  await expect(page.getByLabel("日志级别")).toBeFocused();
  await page.getByLabel("日志级别").selectOption("info");
  await waitForDraft(page);
  await page.getByRole("button", { name: "仅校验" }).click();
  await expect(page.getByText("校验通过", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "校验并写入" }).click();
  await expect(page.getByText("正式配置已写入", { exact: true })).toBeVisible();

  let fullExportRequests = 0;
  page.on("request", (request) => {
    if (request.url().includes("/export?mode=full")) fullExportRequests += 1;
  });
  page.once("dialog", (dialog) => dialog.dismiss());
  await page.getByRole("button", { name: "下载完整配置" }).click();
  await page.waitForTimeout(700);
  expect(fullExportRequests).toBe(0);

  page.once("dialog", (dialog) => dialog.accept());
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "下载完整配置" }).click(),
  ]);
  expect(download.suggestedFilename()).toBe("primary-config.json");
  const downloadPath = await download.path();
  if (!downloadPath) throw new Error("download has no local path");
  const fullConfig = JSON.parse(await readFile(downloadPath, "utf8"));
  const knownSecret = fullConfig.experimental.clash_api.secret as string;
  expect(knownSecret).toHaveLength(64);

  await page.getByRole("button", { name: "复制脱敏配置" }).click();
  await expect(page.getByText("脱敏配置已复制", { exact: true })).toBeVisible();
  const redacted = await page.evaluate(() => navigator.clipboard.readText());
  expect(redacted).toContain("***REDACTED***");
  expect(redacted).not.toContain(knownSecret);

  await page.getByRole("link", { name: "设置" }).click();
  await page.getByLabel("新密码").fill(temporaryPassword);
  await page.getByLabel("确认新密码").fill(temporaryPassword);
  await page.getByRole("button", { name: "修改管理员密码" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("密码").fill(fixturePassword);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("密码").fill(temporaryPassword);
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await page.getByRole("link", { name: "设置" }).click();
  await page.getByLabel("新密码").fill(fixturePassword);
  await page.getByLabel("确认新密码").fill(fixturePassword);
  await page.getByRole("button", { name: "修改管理员密码" }).click();
  await login(page);
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect((await context.cookies()).some((cookie) => cookie.name === "sb_session")).toBe(false);
});
```

Create `web/e2e/capability-placeholders.spec.ts`:

```ts
import { expect, test, type Page } from "@playwright/test";

async function login(page: Page): Promise<void> {
  await page.goto("/login");
  await page.getByLabel("密码").fill("phase1-e2e-password");
  await page.getByRole("button", { name: "登录" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
}

test("phase-one capability pages are truthful and core API is absent", async ({ page }) => {
  await login(page);
  for (const item of [
    { nav: "运行状态", url: "/runtime" },
    { nav: "订阅", url: "/subscriptions" },
    { nav: "规则集", url: "/rulesets" },
  ]) {
    await page.getByRole("link", { name: item.nav }).click();
    await expect(page).toHaveURL(new RegExp(`${item.url}$`));
    await expect(page.getByText("尚未接入", { exact: true })).toBeVisible();
    await expect(page.locator("body")).not.toContainText(/MB\/s|206 ms|核心重启完成|启动核心|重启核心/);
  }
  const core = await page.request.get("/api/v1/core");
  expect(core.status()).toBe(404);
});
```

Run before implementing any E2E-specific fix:

```bash
npm --prefix web run test:e2e
```

Expected: FAIL because the finite Caddy/backend fixture does not exist yet；do not point these tests at a manually running or production service.

- [ ] **Step 3: 创建有限生命周期的 Caddy E2E 启动器**

Create executable `scripts/e2e-server.sh`:

```sh
#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
runtime_root=$(mktemp -d)
backend_pid=
caddy_pid=

cleanup() {
    if [ -n "$caddy_pid" ]; then
        kill "$caddy_pid" 2>/dev/null || true
        wait "$caddy_pid" 2>/dev/null || true
    fi
    if [ -n "$backend_pid" ]; then
        kill "$backend_pid" 2>/dev/null || true
        wait "$backend_pid" 2>/dev/null || true
    fi
    if [ -f "$runtime_root/.singbox-web-e2e-root" ]; then
        rm -rf -- "$runtime_root"
    fi
}
trap cleanup EXIT
trap 'exit 0' HUP INT TERM
touch "$runtime_root/.singbox-web-e2e-root"

for required_command in caddy curl grep id jq ss; do
    command -v "$required_command" >/dev/null
done
for required_port in 18080 18443; do
    if ss -H -ltn "sport = :$required_port" | grep -q .; then
        echo "E2E port already occupied: $required_port" >&2
        exit 1
    fi
done

data_dir=$runtime_root/data
config_root=$runtime_root/config
caddy_data=$runtime_root/caddy-data
caddy_config=$runtime_root/caddy-config
args_log=$runtime_root/fake-sing-box.args
backend_log=$runtime_root/backend.log
caddy_log=$runtime_root/caddy.log
mkdir -p "$data_dir" "$config_root" "$caddy_data" "$caddy_config"
chmod 0700 "$data_dir"
chmod 0750 "$config_root"

fake_sing_box=$runtime_root/fake-sing-box
cat >"$fake_sing_box" <<'FAKE_SING_BOX'
#!/bin/sh
set -eu

: "${FAKE_SING_BOX_ARGS_LOG:?}"
printf '%s\n' "$@" >>"$FAKE_SING_BOX_ARGS_LOG"
case "${1:-}" in
    version)
        test "$#" -eq 1
        printf '%s\n' 'sing-box version e2e-latest'
        ;;
    check)
        test "$#" -eq 3
        test "$2" = '-C'
        test -d "$3"
        test ! -e "$3/config.json"
        module_count=0
        for module_path in "$3"/*.json; do
            test -f "$module_path" || continue
            jq -e 'type == "object" and length == 1' "$module_path" >/dev/null
            module_count=$((module_count + 1))
        done
        test "$module_count" -gt 0
        ;;
    run)
        echo 'sing-box run is forbidden in phase-one E2E' >&2
        exit 90
        ;;
    *)
        echo 'unsupported fake sing-box command' >&2
        exit 64
        ;;
esac
FAKE_SING_BOX
chmod 0755 "$fake_sing_box"
export FAKE_SING_BOX_ARGS_LOG=$args_log

printf '%s\n%s\n' 'phase1-e2e-password' 'phase1-e2e-password' \
    | "$repo_dir/dist/singbox-web" admin set-password --data-dir "$data_dir"

config_group=$(id -gn)
"$repo_dir/dist/singbox-web" serve \
    --listen 127.0.0.1:18080 \
    --data-dir "$data_dir" \
    --config-root "$config_root" \
    --sing-box "$fake_sing_box" \
    --config-group "$config_group" \
    --public-origin https://127.0.0.1:18443 \
    >"$backend_log" 2>&1 &
backend_pid=$!

attempt=0
until curl -fsS http://127.0.0.1:18080/healthz >/dev/null; do
    if ! kill -0 "$backend_pid" 2>/dev/null; then
        sed -n '1,160p' "$backend_log" >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        echo 'backend health timeout' >&2
        exit 1
    fi
    sleep 0.1
done

caddyfile=$runtime_root/Caddyfile
cat >"$caddyfile" <<'CADDYFILE'
{
    admin off
    skip_install_trust
}

https://127.0.0.1:18443 {
    tls internal
    request_body {
        max_size 2MB
    }
    header {
        Strict-Transport-Security "max-age=31536000"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "no-referrer"
    }
    reverse_proxy 127.0.0.1:18080
}
CADDYFILE
XDG_DATA_HOME="$caddy_data" XDG_CONFIG_HOME="$caddy_config" \
    caddy run --config "$caddyfile" --adapter caddyfile >"$caddy_log" 2>&1 &
caddy_pid=$!

attempt=0
until curl -kfsS https://127.0.0.1:18443/healthz >/dev/null; do
    if ! kill -0 "$caddy_pid" 2>/dev/null; then
        sed -n '1,160p' "$caddy_log" >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        echo 'Caddy health timeout' >&2
        exit 1
    fi
    sleep 0.1
done

while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$caddy_pid" 2>/dev/null; do
    if grep -qx 'run' "$args_log" 2>/dev/null; then
        echo 'forbidden sing-box run invocation detected' >&2
        exit 1
    fi
    sleep 1
done
echo 'E2E backend or Caddy exited unexpectedly' >&2
sed -n '1,160p' "$backend_log" >&2
sed -n '1,160p' "$caddy_log" >&2
exit 1
```

Create `web/playwright.config.ts`:

```ts
import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "@playwright/test";

const webDir = path.dirname(fileURLToPath(import.meta.url));
const repoDir = path.resolve(webDir, "..");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 90_000,
  expect: { timeout: 10_000 },
  reporter: "line",
  use: {
    baseURL: "https://127.0.0.1:18443",
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "./scripts/e2e-server.sh",
    cwd: repoDir,
    url: "https://127.0.0.1:18443/healthz",
    ignoreHTTPSErrors: true,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
```

Run `chmod +x scripts/e2e-server.sh`. The launcher uses only a validated `mktemp -d` root, fails before starting if either fixed port is occupied, waits for both health endpoints, and traps every exit to stop both child processes. It exits if the fake ever observes `run`; it accepts only `version` and exact `check -C` calls.

Per repository AGENTS constraints, when using subagent-driven execution the subagent may write/review these files but the primary agent must run the Playwright command because it manages short-lived server processes.

- [ ] **Step 4: 运行端到端与安全回归**

Run as the primary agent:

```bash
./scripts/build.sh
npm --prefix web run test:e2e
go test ./...
go test -race ./...
go vet ./...
test -n "$(command -v caddy)"
test -n "$(command -v sing-box)"
SING_BOX_BIN="$(command -v sing-box)" go test -v ./tests/contract
```

Expected: all PASS。The rebuilt binary contains the just-built Vue assets；Playwright starts and stops its own backend/Caddy children；contract output records the exact latest version；the fake rejects any `sing-box run` invocation。

- [ ] **Step 5: 执行敏感信息和伪数据扫描**

Run:

```bash
rg -n '7de7c8560ded5834574fe93a117795dc78a5a89cf602c11a04f58bbf358f2c17|change-me|do-not-leak' --glob '!**/*_test.go' --glob '!web/e2e/**' cmd internal web scripts deploy README.md && exit 1 || true
rg -n '8\.72|MB/s|206 ms|核心重启完成|保存并应用' web/src && exit 1 || true
```

Expected: both scans produce no matches outside deliberate test fixtures。

- [ ] **Step 6: 重写 README 作为完整交付手册**

Document exact commands for:

- required Go/Node/Caddy/latest sing-box versions;
- `npm ci --prefix web`、unit tests、`scripts/build.sh`;
- interactive `admin set-password`;
- creation/ownership/modes for data and config roots;
- installing the binary and systemd/Caddy examples;
- updating bundled Schema and recording target version/SHA-256;
- full vs redacted export warning;
- stable profile directory and external `sing-box check -C` use;
- no history/rollback/import behavior;
-一期 unavailable features and the phase-two design handoff path;
- the exact new-context prompt from design section 20 for restarting `superpowers:brainstorming` on phase two A；
- recovery behavior for corrupt JSON, external drift and interrupted publication.

Do not restore the currently deleted testing documents; link only to the committed design and implementation plan.

- [ ] **Step 7: 运行最终干净构建验证**

Run:

```bash
git status --short
npm ci --prefix web
npm run test:web
npm run typecheck
./scripts/build.sh
npm --prefix web run test:e2e
go test ./...
go test -race ./...
go vet ./...
test -n "$(command -v caddy)"
test -n "$(command -v sing-box)"
SING_BOX_BIN="$(command -v sing-box)" go test -v ./tests/contract
git diff --check
```

Expected: all commands PASS。Before staging, `git status` shows the Task 21 source/doc changes plus only the user's pre-existing `index.html` modification and two deleted docs；no generated secret、binary、embedded asset output or config directory is listed or staged. After the Task 21 commit, only those three user changes remain.

- [ ] **Step 8: 提交一期验收与文档**

```bash
git add tests/contract web/playwright.config.ts web/e2e web/package.json web/package-lock.json scripts/e2e-server.sh README.md
git commit -m "完成一期端到端验收"
```

---

## Spec Coverage Matrix

| Spec section | Implemented by |
|---|---|
| 1–4 背景、目标、非目标、技术方案 | Global Constraints、Tasks 1、15、21 |
| 5 总体架构与 capability 边界 | Tasks 12、14、15、19、20 |
| 6 JSON 应用数据、锁、损坏处理 | Tasks 2、6、7、14 |
| 7 管理员初始化、Argon2id、会话、CSRF | Tasks 3、4、12、15 |
| 8 档案模型、slug、默认模板、唯一事实来源 | Tasks 5、6、7、13、16、17 |
| 9 顶层模块文件、稳定链接、权限 | Tasks 8、11、20 |
| 10 草稿自动保存、ETag、冲突、离线 | Tasks 7、13、16 |
| 11 Schema、语义、真实 check、漂移与恢复 | Tasks 9、10、11、14、21 |
| 12 完整/脱敏导出和敏感字段 | Tasks 8、14、17、19、21 |
| 13 前端信息架构和全部一期页面 | Tasks 15–19 |
| 14 HTTP API 和稳定错误体 | Tasks 12–14 |
| 15 公网、主机、Caddy、systemd 安全 | Tasks 12、14、20、21 |
| 16 异常处理和敏感日志 | Tasks 2、3、7、10–14、16、19 |
| 17 测试策略 | Every task's red/green cycle、Task 21 contract/E2E |
| 18 一期验收 | Task 21、Final Verification Gate |
| 19–20 二期边界与交接 | capability keys in Tasks 12/15/19；README and committed spec in Task 21；no runtime implementation in一期 |

Self-review result: all spec sections map to at least one testable task; Phase 2 remains an explicit interface boundary rather than hidden一期 work.

---

## Final Verification Gate

在宣称一期完成前，执行者必须显式调用 `superpowers:verification-before-completion`，重新运行 Task 21 Step 7 的全部命令，并检查以下事实：

- `git diff --check` 无错误。
- 所有测试使用的真实 sing-box 版本与 `schema.version.json` 一致。
- 发布失败、超时、崩溃恢复和外部漂移均有通过的自动化证据。
- 正式配置目录中不存在合并后的 `config.json`。
- 应用日志、HTTP 错误和脱敏输出不包含已知测试秘密。
- `/api/v1/core` 没有伪实现，运行页面没有伪数据。
- 当前用户原有的 `index.html` 修改与两项文档删除仍未被任何一期提交夹带。
