# sing-box Web 一期 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将零依赖浏览器原型实现为 Linux 单机可运行的 Go 单体服务与 Vue 3 管理页面，完整交付一期认证、档案、规则资源、配置应用、受管 sing-box 核心和手动更新能力。

**Architecture:** 计划按四个可独立测试的子项目顺序交付：先建立固定工作目录、可靠持久化、认证和档案 API，再实现规则资源与确定性配置生成，随后实现唯一子进程、应用事务和核心更新，最后迁移 Vue 界面并完成跨层验收。Go 业务包通过小接口协作，只有 `core.Manager` 可以拥有 sing-box 子进程；Vue 只通过同源 `/api/v1` 获取服务器状态，不从草稿或模拟数据推断运行状态。

**Tech Stack:** Go 1.26、标准库 `net/http`、`golang.org/x/crypto/argon2`、`golang.org/x/term`、Vue 3、TypeScript、Vite、Vitest、Vue Test Utils、Playwright、npm。

## Global Constraints

- 目标平台是 Linux 单机；Web 程序自身的安装、启动、守护、systemd、Caddy、HTTPS、域名和公网部署不属于实现或验收范围。
- Go 服务默认只监听 `127.0.0.1`，静态资源与 `/api/v1` 同源，不开放宽松 CORS。
- 应用数据目录固定为 `data`，正式配置目录固定为 `config`；所有路径相对于 Web 启动工作目录，且命令行、环境变量、应用配置、页面和 API 均不得覆盖这些目录。
- `data/bin/sing-box` 是唯一受管程序，`config` 是唯一正式配置目录，同一时刻最多存在一个由 Web 直接管理的 sing-box 子进程。
- 所有持久化 JSON、规则文件和程序下载先写同一文件系统临时文件，通过检查后原子重命名；核心 JSON 文件损坏时拒绝写入，单个草稿损坏时隔离对应档案。
- 生产运行和普通构建不得依赖 `temp/`；`temp/config.default.json` 只作为生成受版本控制嵌入模板的输入基准。
- 登录只接受密码，不存在用户名、注册或找回密码；密码使用 Argon2id，所有写请求同时检查会话 Cookie 与 CSRF Token。
- 会话 Cookie 固定为 `HttpOnly`、`SameSite=Strict`、不设置 `Secure`，默认 24 小时无操作过期；修改密码使所有既有会话立即失效。
- 不维护固定 JSON Schema；Web 语义检查之后必须由当前 `data/bin/sing-box check -C <staging-dir>` 决定能否应用。
- 用户未结构化支持的合法配置字段必须在草稿、模块 JSON 编辑、预览和应用中无损往返；字段值、检查错误和核心输出不做遮罩或脱敏。
- 产品现在和以后都不提供代理节点订阅、URL 拉取、解析、同步或定时刷新；代理节点只允许在出站模块 JSON 编辑器中手工粘贴或编辑。
- 不提供合并配置下载、完整或脱敏配置复制，以及其他配置导出接口或入口。
- 一期不展示模拟流量、连接、策略组、测速或完整核心日志，也不提供独立高级核心控制；未接入能力必须明确显示不可用。
- 自动化测试和示例不得包含真实节点密码、证书私钥、生产端点或其他运营数据。

---

## Scope Decomposition

本规格包含四个具有不同故障模型和验收边界的子系统。为保证每次评审都能得到可运行、可测试的软件，本总览只锁定全局结构与接口，具体步骤拆到以下顺序计划：

1. [`2026-08-21-singbox-web-phase1-foundation.md`](2026-08-21-singbox-web-phase1-foundation.md)：工程骨架、固定路径、原子持久化、独占锁、默认模板、单密码认证、会话和档案草稿。
2. [`2026-08-21-singbox-web-phase1-rules-config.md`](2026-08-21-singbox-web-phase1-rules-config.md)：规则资源下载与调度、绑定关系、确定性多文件生成、语义检查和实际二进制检查。
3. [`2026-08-21-singbox-web-phase1-core-operations.md`](2026-08-21-singbox-web-phase1-core-operations.md)：唯一子进程、日志、应用回滚事务、启动恢复、官方核心下载与手动更新。
4. [`2026-08-21-singbox-web-phase1-spa-acceptance.md`](2026-08-21-singbox-web-phase1-spa-acceptance.md)：Vue 功能界面、自动保存与冲突恢复、应用/核心状态、响应式视觉迁移、端到端及真实二进制契约验收。

四份计划必须按顺序执行。每份计划结束时均运行当期所有 Go 与前端测试并创建中文 Git 提交；后续计划只能消费本文件定义的稳定接口，不绕过服务直接读写其他功能包的数据文件。

## Locked File Structure

### 构建入口与共享基础设施

- `go.mod`、`go.sum`：根 Go 模块 `singbox-web`，仅直接引入 Argon2id 所需的 `golang.org/x/crypto` 与终端密码读取所需的 `golang.org/x/term`；新增依赖必须有规格中的直接用途。
- `cmd/singbox-web/main.go`：唯一二进制入口；只把进程参数、标准输入输出、信号和当前工作目录交给 `internal/app`。
- `internal/app/command.go`：实现 `admin set-password` 与 `serve` 命令分派，不提供数据目录参数。
- `internal/app/server.go`：组合服务、获取 `data/app.lock`、恢复调度/核心、启动同源 HTTP 服务并协调优雅关闭。
- `internal/paths/layout.go`：定义固定相对路径和包含性检查；其他包不能手工拼接用户提供的路径。
- `internal/storage/atomic.go`：文件同步、原子重命名、目录同步和严格 JSON 读写。
- `internal/storage/lock_linux.go`：Linux 独占实例锁；锁失败返回稳定错误，不等待第二实例。
- `internal/storage/cleanup.go`：只枚举并验证 staging、rollback 和下载临时目录；具体业务恢复者结合持久状态决定恢复或删除。
- `internal/apierror/error.go`：稳定错误码、中文消息、阶段、字段路径和最新修订号的统一表示。
- `internal/httpapi/router.go`：`/api/v1` 路由、请求体限制、严格 JSON 解码、安全响应头、认证和 CSRF 中间件。
- `internal/webui/embed.go`、`internal/webui/assets/`、`internal/webui/fallback/index.html`：嵌入 Vue 构建产物；未构建时返回明确 fallback，构建后为未知非 API 路由返回 SPA 入口；`/api/` 从不回退到 HTML。
- `scripts/build.sh`：先构建 Vue，再构建 Go；保持 `internal/webui/assets/_placeholder`，使干净仓库中的 `go test ./...` 也能编译嵌入包。

### 认证、档案与模板

- `internal/auth/model.go`：`auth.json` 版本、Argon2id 参数、会话签名密钥和失效代次。
- `internal/auth/store.go`：严格读取和原子写入 `data/auth.json`。
- `internal/auth/password.go`：密码哈希、常量时间校验和会话密钥轮换。
- `internal/auth/session.go`：HMAC 会话、24 小时滑动过期、CSRF Token 和 Cookie 属性。
- `internal/auth/handler.go`：登录、退出、会话查询和修改密码 API；拒绝未知字段与用户名字段。
- `internal/auth/limiter.go`：按请求来源限制登录失败，不泄露身份或密码状态。
- `internal/templates/embed.go`：嵌入默认档案、默认规则资源和默认绑定。
- `internal/templates/assets/default-profile.json`：从输入中移除原生远程规则集后的普通 sing-box 配置。
- `internal/templates/assets/default-rule-resources.json`：远程规则的 tag、format、URL 和周期模板。
- `internal/templates/assets/default-bindings.json`：默认资源模板键到档案 tag 的绑定。
- `tools/templategen/main.go`：显式读取 `temp/config.default.json` 并确定性生成三个受版本控制资源；运行时不导入此工具。
- `internal/profile/model.go`：档案元数据、草稿包、模块缓冲、规则绑定和检查摘要。
- `internal/profile/store.go`：`profiles.json` 与 `data/drafts/<profile-id>.json` 的一致写入、损坏隔离和修订更新。
- `internal/profile/service.go`：创建、改名、删除、读取、`If-Match` 保存和 `needsApply` 状态转换。
- `internal/profile/handler.go`：档案与草稿 HTTP API、ETag 和 `409 Conflict`。

### 规则资源、配置生成与检查

- `internal/rules/model.go`：资源配置、固定本地路径、下载状态、下一次调度和引用摘要。
- `internal/rules/store.go`：`rule-resources.json` 严格读写及资源 ID 到安全文件名的唯一映射。
- `internal/rules/service.go`：CRUD、引用保护、资源级锁、手动更新和档案脏状态传播。
- `internal/rules/downloader.go`：受限 HTTP 客户端、逐次重定向检查、大小/超时限制和 source/binary 内容检查。
- `internal/rules/scheduler.go`：服务端定时队列、重启重建、禁用和失败后的下一次调度。
- `internal/rules/handler.go`：规则资源 CRUD 与立即更新 API。
- `internal/configgen/model.go`：12 个受管顶层键、文件名映射、预览文件和综合摘要类型。
- `internal/configgen/generator.go`：把草稿和资源快照生成确定性模块文件，注入本地 `route.rule_set`。
- `internal/configgen/semantic.go`：JSON 缓冲、tag、出站、DNS、规则集、资源绑定与条件必填检查。
- `internal/configgen/checker.go`：直接执行当前二进制的 `check -C`，30 秒超时，stdout/stderr 各 64 KiB。
- `internal/configgen/handler.go`：最终模块预览与完整检查端点。

### 核心生命周期、应用与更新

- `internal/core/model.go`：`core.json`、进程状态、期望运行、当前档案、版本、PID 和最近错误。
- `internal/core/log.go`：限制单条与总大小的 `data/logs/sing-box.log` 写入。
- `internal/core/process_linux.go`：进程组、优雅信号、10 秒超时强杀和 Web 退出时防孤儿设置。
- `internal/core/manager.go`：唯一的 `Start`、`Stop`、`Status` 和退出观察实现。
- `internal/core/operation_lock.go`：应用、核心更新和生命周期共享的全局操作锁。
- `internal/application/coordinator.go`：停止、生成、检查、目录切换、启动、提交状态和失败回滚的完整事务。
- `internal/application/drift.go`：正式 `config` 摘要计算与外部漂移确认令牌。
- `internal/application/handler.go`：预览、检查和应用状态 API。
- `internal/updater/releases.go`：官方最新稳定版元数据、Linux 架构资产和 SHA-256 资产选择。
- `internal/updater/archive.go`：下载、摘要校验、压缩包路径/符号链接检查和目标提取。
- `internal/updater/service.go`：首次安装、唯一 `.bak`、现有配置预检、停止/安装/启动和失败恢复。
- `internal/updater/handler.go`：最新版本、开始更新和更新状态 API。

### Vue SPA 与验收

- `web/package.json`、`web/vite.config.ts`、`web/tsconfig*.json`：Vue 构建、类型检查、Vitest 与 Playwright 命令。
- `web/src/main.ts`、`web/src/router.ts`：应用入口、认证守卫和产品路由。
- `web/src/api/client.ts`、`web/src/api/types.ts`：同源 API 客户端、CSRF、错误与所有 DTO；组件不直接调用 `fetch`。
- `web/src/features/auth/`：仅密码登录与修改密码。
- `web/src/features/profiles/`：档案侧栏、创建/改名/删除/切换和草稿自动保存状态机。
- `web/src/features/config-editor/`：七个配置标签、结构化路径编辑、模块 JSON 缓冲和出站节点 JSON。
- `web/src/features/rule-resources/`：资源 CRUD、禁用、更新、引用保护与调度状态。
- `web/src/features/application/`：模块预览、检查、应用阶段、冲突、漂移确认和恢复结果。
- `web/src/features/core/`：安装版本、最新稳定版、运行状态、PID、当前档案和确认式手动更新。
- `web/src/features/capabilities/`：稳定能力开关与二期不可用说明。
- `web/src/features/runtime/`：只显示不可用说明，不包含模拟数据或可执行按钮。
- `web/src/features/settings/`：主题、界面偏好与密码修改；仅主题/界面偏好使用 `localStorage`。
- `web/src/styles/`：从 `prototype/src/styles.css` 提炼出的 tokens、布局、组件和响应式样式。
- `web/tests/`：Vitest 单元/组件测试和共享工厂。
- `e2e/`：Playwright 测试、假 sing-box、受控规则/Release HTTP 服务和隔离工作目录。
- `internal/contracttest/`：真实稳定版只执行 `version` 与 `check` 的可显式启用契约测试。

## Stable Cross-Plan Interfaces

以下签名在首次引入后不得在后续计划中另起别名；需要演进时先同时修改消费方测试。

```go
// internal/paths
func Resolve(workDir string) (Layout, error)
func (l Layout) RuleSetPath(resourceID, extension string) (absolute string, relative string, err error)

// internal/storage
func ReadJSON[T any](path string) (T, error)
func WriteJSON(path string, value any, perm fs.FileMode) error
func AcquireInstanceLock(path string) (*InstanceLock, error)

// internal/profile
type RuleBinding struct {
    ResourceID string `json:"resourceId"`
    Tag        string `json:"tag"`
    Enabled    bool   `json:"enabled"`
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
    Buffers  map[string]string `json:"buffers"`
    Bindings []RuleBinding     `json:"bindings"`
}
func (s *Service) GetDraft(ctx context.Context, profileID string) (Draft, string, error)
func (s *Service) SaveDraft(ctx context.Context, profileID, ifMatch string, input DraftInput) (Draft, string, error)
func (s *Service) RecordValidation(ctx context.Context, profileID string, revision uint64, result ValidationSummary) error
func (s *Service) MarkResourcesChanged(ctx context.Context, resourceIDs []string) error
func (s *Service) MarkApplied(ctx context.Context, result AppliedResult) error

type AppliedResult struct {
    ProfileID       string
    Revision        uint64
    Digest          string
    ResourceDigests map[string]string
    AppliedAt       time.Time
    Validation      ValidationSummary
}
type ValidationSummary struct {
    Valid     bool
    Revision  uint64
    CheckedAt time.Time
    Stage     string
    Message   string
}

// internal/rules
type Snapshot struct {
    Resources map[string]Resource
    Digests   map[string]string
}
func (s *Service) SnapshotForBindings(ctx context.Context, bindings []profile.RuleBinding) (Snapshot, func(), error)

// internal/configgen
type Output struct {
    Files  []File  `json:"files"`
    Digest string  `json:"digest"`
}
func (g *Generator) Generate(ctx context.Context, draft profile.Draft, snapshot rules.Snapshot) (Output, error)
func (g *Generator) WriteDirectory(ctx context.Context, output Output, dir string) error
func (c *Checker) Check(ctx context.Context, binaryPath, configDir string) (CheckResult, error)

// internal/core
func (m *Manager) Start(ctx context.Context, request StartRequest) error
func (m *Manager) Stop(ctx context.Context) error
func (m *Manager) Status(ctx context.Context) Status
func (l *OperationLock) Enter(ctx context.Context, kind OperationKind) (operation *Operation, release func(), err error)

// internal/application
func (c *Coordinator) Validate(ctx context.Context, request ValidateRequest) (ValidateResult, error)
func (c *Coordinator) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error)
```

HTTP 错误统一使用：

```json
{
  "error": {
    "code": "draft_revision_conflict",
    "message": "服务器草稿已被其他页面更新",
    "stage": "save_draft",
    "fieldPath": "",
    "latestRevision": 7
  }
}
```

所有日期时间使用 UTC RFC 3339 字符串，摘要使用小写 SHA-256 十六进制，ID 使用 16 个随机字节的小写十六进制。HTTP DTO 使用 `camelCase`，持久化 JSON 与 HTTP DTO 共用相同字段名，但 Cookie、密码哈希和签名密钥永不出现在 API 响应中。

## Specification Coverage Matrix

| 规格章节 | 计划与任务 |
|---|---|
| 1–5 背景、目标、非目标、技术与状态边界 | 本总览；SPA 计划 Task 1、7 |
| 6 应用数据、固定目录、原子写入、损坏与锁 | Foundation Task 2、3 |
| 7 单密码认证、CLI、会话、CSRF、限速 | Foundation Task 4、5 |
| 8 档案、模板、唯一事实来源、未知字段往返 | Foundation Task 6、7；Rules/Config Task 4 |
| 9 规则资源、下载、调度、引用与需要应用 | Rules/Config Task 1、2、3 |
| 10 正式模块文件、唯一目录与临时事务 | Rules/Config Task 4；Core Operations Task 3 |
| 11 自动保存、ETag、冲突、离线与关闭提示 | Foundation Task 7；SPA Task 2 |
| 12 检查、应用顺序与失败恢复 | Rules/Config Task 5、6；Core Operations Task 3、4 |
| 13 核心生命周期、缺失、下载与更新 | Core Operations Task 1、2、5、6 |
| 14 原值显示 | SPA Task 3、5、6 |
| 15 前端组织、页面与配置交互 | SPA Task 1–7 |
| 16 HTTP API 与全局锁 | Foundation Task 5、7；Rules/Config Task 1、6；Core Operations Task 4、6 |
| 17 安全边界 | Foundation Task 2、5、8；Rules/Config Task 2；Core Operations Task 5 |
| 18 异常处理与安全日志 | 各计划故障测试；Core Operations Task 3、7 |
| 19 单元、组件、集成、契约与 E2E 测试 | 各计划测试步骤；SPA Task 8、9 |
| 20 一期验收标准 | SPA Task 9 |
| 21 二期稳定接口 | Foundation Task 8；Core Operations Task 2；SPA Task 7 |

## End-of-Suite Verification

完成四份计划后，在干净工作树中按顺序执行：

```bash
go test ./...
npm --prefix web run typecheck
npm --prefix web run test:unit
sh scripts/build.sh
npm --prefix web run test:e2e
sh scripts/contract-test.sh
git status --short
```

预期结果：所有命令退出码为 0；契约测试输出受测 sing-box 的真实版本和 SHA-256；`git status --short` 为空；测试工作目录中没有持久化 `config.staging-*`、`config.rollback-*` 或规则版本目录。
