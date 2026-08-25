# sing-box Web 一期 Vue SPA 与验收 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有视觉原型迁移成 Vue 3/TypeScript 管理页面，接通全部一期 API，并用组件、端到端和真实 sing-box 契约测试完成产品验收。

**Architecture:** SPA 按 auth、profiles、config-editor、rule-resources、application、core、capabilities、settings/runtime 功能域拆分；组件只消费 typed API client 和功能 store。档案 store 保存服务器 ETag、最近有效配置、原始模块缓冲和未确认修改，运行状态只来自 `/core`；Vite 产物写入 Go 的嵌入目录，由同源服务提供。

**Tech Stack:** Vue 3.5.18、Vue Router 4.5.1、TypeScript 5.9.2、Vite 7.1.2、Vitest 3.2.4、Vue Test Utils 2.4.6、Playwright 1.54.2、Node.js 22、npm 10、Go 1.26。

## Global Constraints

- 登录页只显示密码输入框，不显示或接受用户名，不提供注册或找回密码。
- 配置工作区保留概览、DNS、入站、出站、路由、服务和高级标签。
- 代理节点不设独立页面，只能在出站模块 JSON 编辑器中手工粘贴或编辑节点 JSON。
- 产品现在和以后都不提供代理节点订阅、URL 拉取、解析、同步或定时刷新。
- 主操作名称固定为“应用”；持续显示保存中、已保存、冲突、需要应用、正在应用、运行中和应用失败。
- 应用前按最终模块文件预览配置，必须显示转换后的本地规则路径而不是资源 URL。
- 配置字段、模块预览、检查错误和核心输出按原值显示，不识别、遮罩或提供临时显示交互。
- 页面和 API 不提供合并配置下载、完整或脱敏配置复制及其他配置导出功能。
- 运行流量、连接、策略组、测速和完整日志明确显示二期尚未接入，不展示模拟数据或可执行按钮。
- 多档案只填写显示名称，不提供配置目录或路径输入；删除档案必须输入显示名称确认。
- 主题与界面偏好可以使用 `localStorage`；档案草稿、规则、核心和认证状态不得以浏览器存储作为事实来源。
- 保留桌面、平板、手机、浅色和深色响应式体验，界面文案使用中文。

---

## File Responsibility Map

- `web/src/api/`：唯一 fetch 边界、DTO、ETag、CSRF 和错误解码。
- `web/src/features/auth/`：会话初始化、仅密码登录/修改与路由守卫。
- `web/src/features/profiles/`：档案列表、侧栏、自动保存状态机、冲突与离线恢复。
- `web/src/features/config-editor/`：七个模块标签、路径编辑和 JSON 文本缓冲。
- `web/src/features/rule-resources/`：全局规则资源和档案绑定。
- `web/src/features/application/`：预览、检查、应用、漂移确认与错误跳转。
- `web/src/features/core/`：安装、版本、真实子进程状态和手动更新。
- `web/src/features/capabilities/`、`runtime/`：一期能力与二期稳定不可用边界。
- `web/src/features/settings/`：主题、布局偏好与密码修改。
- `web/src/styles/`：从原型提取的 token、壳层、组件和断点。
- `web/tests/`：纯函数、store 与组件测试；`e2e/`：完整浏览器流程和视觉断点。
- `internal/contracttest/`：显式使用真实官方稳定版，只执行 `version` 与 `check`。

### Task 1: 建立 Vue、typed API、仅密码认证与响应式壳层

**Files:**
- Create: `web/package.json`
- Create: `web/index.html`
- Create: `web/tsconfig.json`
- Create: `web/tsconfig.app.json`
- Create: `web/vite.config.ts`
- Create: `web/vitest.config.ts`
- Create: `web/src/env.d.ts`
- Create: `web/src/main.ts`
- Create: `web/src/App.vue`
- Create: `web/src/router.ts`
- Create: `web/src/api/types.ts`
- Create: `web/src/api/client.ts`
- Create: `web/src/features/auth/store.ts`
- Create: `web/src/features/auth/LoginView.vue`
- Create: `web/src/layout/AppShell.vue`
- Create: `web/src/layout/AppSidebar.vue`
- Create: `web/src/layout/AppTopbar.vue`
- Create: `web/src/styles/tokens.css`
- Create: `web/src/styles/base.css`
- Create: `web/tests/setup.ts`
- Create: `web/tests/auth.test.ts`
- Modify: `scripts/build.sh`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: 同源 `/api/v1/auth/*`、`/capabilities`；构建输出目录 `internal/webui/assets`。
- Produces: `api.request<T>(path, options): Promise<ApiResponse<T>>`、`useAuthStore()`、认证路由守卫和可登录 SPA。

- [ ] **Step 1: 写仅密码界面、未知字段禁用和路由守卫测试**

```ts
it("submits only the password and never renders a username field", async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ valid: true, expiresAt: "2026-08-22T00:00:00Z", csrfToken: "csrf-a" }));
  vi.stubGlobal("fetch", fetchMock);
  const wrapper = mount(LoginView, { global: { plugins: [router] } });
  await wrapper.get("input[type=password]").setValue("correct horse battery staple");
  await wrapper.get("form").trigger("submit");
  expect(wrapper.find('input[name="username"]').exists()).toBe(false);
  const [, request] = fetchMock.mock.calls[0];
  expect(JSON.parse(String(request.body))).toEqual({ password: "correct horse battery staple" });
});

it("redirects an invalid session to login", async () => {
  mockAPI({ "/api/v1/auth/session": { status: 200, body: { valid: false, expiresAt: null } } });
  await router.push("/");
  await router.isReady();
  expect(router.currentRoute.value.name).toBe("login");
});
```

- [ ] **Step 2: 运行前端测试并确认工程尚不存在**

Run: `npm --prefix web run test:unit -- auth.test.ts`

Expected: FAIL，`web/package.json` 或测试入口不存在。

- [ ] **Step 3: 创建精确依赖和构建脚本**

```json
{
  "name": "singbox-web-ui",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite --host 127.0.0.1 --port 5173",
    "build": "vue-tsc -b && vite build",
    "typecheck": "vue-tsc -b",
    "test:unit": "vitest run",
    "test:e2e": "playwright test"
  },
  "dependencies": {
    "vue": "3.5.18",
    "vue-router": "4.5.1",
    "jsonc-parser": "3.3.1"
  },
  "devDependencies": {
    "@playwright/test": "1.54.2",
    "@types/node": "22.17.1",
    "@vitejs/plugin-vue": "6.0.1",
    "@vue/test-utils": "2.4.6",
    "jsdom": "26.1.0",
    "typescript": "5.9.2",
    "vite": "7.1.2",
    "vitest": "3.2.4",
    "vue-tsc": "3.0.5"
  }
}
```

Vite `build.outDir` 固定为 `../internal/webui/assets`，`emptyOutDir:false`。`scripts/build.sh` 先用经过显式路径验证的 `find internal/webui/assets -mindepth 1 ! -name _placeholder -delete` 清理旧 hash 产物，再执行 `npm --prefix web run build`、确认 `index.html` 存在，最后构建 Go 二进制。生成产物忽略，`_placeholder` 保持跟踪。

- [ ] **Step 4: 定义 API 错误、响应和 CSRF 客户端**

```ts
export interface ApiErrorBody {
  code: string;
  message: string;
  stage?: string;
  fieldPath?: string;
  latestRevision?: number;
  details?: Record<string, unknown>;
}

export class ApiError extends Error {
  constructor(public status: number, public body: ApiErrorBody) {
    super(body.message);
  }
}

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  headers?: HeadersInit;
  body?: unknown;
}

export interface ApiResponse<T> {
  data: T;
  etag?: string;
}

let csrfToken = "";

export async function request<T>(path: string, options: RequestOptions = {}): Promise<ApiResponse<T>> {
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (options.body !== undefined) headers.set("Content-Type", "application/json");
  if (!/^(GET|HEAD)$/.test(options.method ?? "GET") && csrfToken) headers.set("X-CSRF-Token", csrfToken);
  const response = await fetch(`/api/v1${path}`, { method: options.method ?? "GET", credentials: "same-origin", headers, body: options.body === undefined ? undefined : JSON.stringify(options.body) });
  const payload = response.status === 204 ? undefined : await response.json();
  if (!response.ok) throw new ApiError(response.status, payload.error);
  if (payload?.csrfToken) csrfToken = payload.csrfToken;
  return { data: payload as T, etag: response.headers.get("ETag") ?? undefined };
}
```

客户端遇到 401 清空内存 CSRF 并导航登录；不得把 Cookie、密码或 CSRF 写入 `localStorage`。

- [ ] **Step 5: 实现认证 store、登录页和 shell**

`useAuthStore` 暴露 `initialize/login/logout/changePassword` 和 `status: "loading"|"anonymous"|"authenticated"`。App 启动先请求 session；登录只传 password；退出成功清状态；修改密码传 current/new password，成功后所有页面回到登录。Shell 的顶部和侧栏使用真实路由 slot，不引入原型演示状态。

- [ ] **Step 6: 提取基础视觉 token 并保持主题入口**

从 `prototype/src/styles.css` 复制实际颜色、间距、圆角、阴影与字体 token 到 `tokens.css`，保留 `data-theme="dark"` 覆盖；`base.css` 只含 reset、body、focus、按钮、表单与屏幕阅读辅助类。不要整文件复制 8072 行样式。

- [ ] **Step 7: 安装依赖并验证登录构建**

Run:

```bash
npm --prefix web install
npm --prefix web exec -- playwright install chromium
npm --prefix web run test:unit -- auth.test.ts
npm --prefix web run typecheck
sh scripts/build.sh
go test ./internal/webui ./internal/httpapi
```

Expected: 所有命令退出码为 0；嵌入 handler 返回新 Vue `index.html`；登录页面没有用户名、注册或找回密码；lockfile 为 `web/package-lock.json`。

- [ ] **Step 8: 提交 Vue 与认证壳层**

```bash
git add web scripts/build.sh .gitignore internal/webui/assets/_placeholder
git commit -m "建立 Vue 登录与应用壳层"
```

### Task 2: 实现档案侧栏、自动保存、离线与冲突恢复

**Files:**
- Create: `web/src/features/profiles/types.ts`
- Create: `web/src/features/profiles/api.ts`
- Create: `web/src/features/profiles/store.ts`
- Create: `web/src/features/profiles/ProfileSidebar.vue`
- Create: `web/src/features/profiles/ProfileDialog.vue`
- Create: `web/src/features/profiles/ProfileDeleteDialog.vue`
- Create: `web/src/features/profiles/SaveStatus.vue`
- Create: `web/src/features/profiles/ConflictDialog.vue`
- Create: `web/tests/profile-store.test.ts`
- Create: `web/tests/profile-components.test.ts`
- Modify: `web/src/layout/AppSidebar.vue`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: 档案与草稿 API、ETag、浏览器 `online/offline/beforeunload` 事件。
- Produces: `useProfileStore()`、活动档案与草稿、600 ms 防抖保存状态机、显式冲突“加载服务器版本/用本地内容覆盖”。

- [ ] **Step 1: 写防抖、冲突、断线恢复和关闭提示测试**

```ts
it("debounces edits and saves with the current ETag", async () => {
  vi.useFakeTimers();
  const api = fakeProfileApi({ draft: draftFixture(3), etag: '"rev-3"' });
  const store = createProfileStore(api);
  await store.open("profile-a");
  store.mutate((draft) => { draft.buffers.log = `{"level":"debug"}`; });
  store.mutate((draft) => { draft.buffers.log = `{"level":"warn"}`; });
  await vi.advanceTimersByTimeAsync(600);
  expect(api.saveDraft).toHaveBeenCalledTimes(1);
  expect(api.saveDraft).toHaveBeenCalledWith("profile-a", '"rev-3"', { buffers: expect.objectContaining({ log: `{"level":"warn"}` }), bindings: expect.any(Array) });
  expect(store.saveState.value).toBe("saved");
  expect(store.etag.value).toBe('"rev-4"');
});

it("keeps local content on 409 until the user chooses", async () => {
  const api = fakeProfileApi({ draft: draftFixture(3), etag: '"rev-3"', saveError: apiError(409, "draft_revision_conflict", { latestRevision: 4 }) });
  const store = createProfileStore(api);
  await store.open("profile-a");
  store.mutate((draft) => { draft.buffers.outbounds = `[{"type":"future","tag":"local"}]`; });
  await store.flush();
  expect(store.saveState.value).toBe("conflict");
  expect(store.draft.value?.buffers.outbounds).toContain('"local"');
  expect(store.conflict.value?.latestRevision).toBe(4);
});

it("retains pending edits while offline and retries after online", async () => {
  const api = fakeProfileApi({ saveError: new TypeError("Failed to fetch") });
  const store = createProfileStore(api);
  await store.open("profile-a");
  store.mutate((draft) => { draft.buffers.log = `{"level":"error"}`; });
  await store.flush();
  expect(store.saveState.value).toBe("offline");
  api.clearSaveError();
  window.dispatchEvent(new Event("online"));
  await flushPromises();
  expect(store.saveState.value).toBe("saved");
});
```

- [ ] **Step 2: 运行档案测试并确认失败**

Run: `npm --prefix web run test:unit -- profile-store.test.ts profile-components.test.ts`

Expected: FAIL，档案 store 与组件尚不存在。

- [ ] **Step 3: 定义与后端完全一致的 DTO**

```ts
export interface ProfileSummary {
  id: string;
  name: string;
  draftRevision: number;
  appliedRevision: number | null;
  appliedDigest: string;
  appliedAt: string | null;
  needsApply: boolean;
  lastValidation: ValidationSummary | null;
  quarantinedError?: string;
}

export interface Draft {
  profileId: string;
  revision: number;
  config: Record<string, unknown>;
  buffers: Record<string, string>;
  bindings: RuleBinding[];
  savedAt: string;
}

export type DraftInput = Pick<Draft, "buffers" | "bindings">;

export type SaveState = "idle" | "dirty" | "saving" | "saved" | "offline" | "conflict" | "error";
```

`profiles/api.ts` 的 `saveDraft` 明确构造 `{buffers: draft.buffers, bindings: draft.bindings}`；Config 由服务端从有效原始缓冲重建，避免浏览器把未知大整数转换为 IEEE-754 后回写。不得序列化 `config/profileId/revision/savedAt`，修订只通过 `If-Match` 发送。

- [ ] **Step 4: 实现可注入 API 的 store 状态机**

```ts
function mutate(change: (draft: Draft) => void): void {
  if (!draft.value || saveState.value === "conflict") return;
  change(draft.value);
  draft.value.revision = serverRevision.value;
  saveState.value = "dirty";
  pendingGeneration += 1;
  scheduleSave(pendingGeneration, 600);
}

async function flush(): Promise<void> {
  if (!draft.value || !etag.value || !["dirty", "offline", "error"].includes(saveState.value)) return;
  const generation = pendingGeneration;
  const payload = structuredClone(draft.value);
  saveState.value = "saving";
  try {
    const response = await api.saveDraft(activeProfileId.value, etag.value, payload);
    etag.value = response.etag;
    serverRevision.value = response.data.revision;
    if (generation === pendingGeneration) { draft.value = response.data; saveState.value = "saved"; }
    else { saveState.value = "dirty"; scheduleSave(pendingGeneration, 600); }
  } catch (error) {
    classifySaveFailure(error);
  }
}
```

`flush` 通过单一 `saveInFlight` promise 串行化，同一档案永远不会有两个并发 PUT；保存期间的新编辑只增加 generation，并在前一请求确认后用新 ETag 再保存。切换档案前必须 flush；仍离线、冲突或失败时弹出阻止切换提示，不能丢弃本地内容。`beforeunload` 只在 dirty/saving/offline/conflict/error 时 `preventDefault`。断线内容只保存在当前内存，不写 localStorage；浏览器真的关闭时由提示保护。

- [ ] **Step 5: 实现显式冲突选择**

“加载服务器版本”重新 GET draft 并替换本地内容；“用本地内容覆盖”先 GET 最新 ETag/服务器内容供对照，用户再次确认后以最新 ETag PUT 当前本地 payload。任何新的 409 重新进入冲突，不循环静默覆盖。

- [ ] **Step 6: 实现档案创建、改名、删除与侧栏**

新建/改名弹窗只有 `name`；删除弹窗必须输入精确显示名称。删除当前运行档案时，根据 API 409 展示“先应用其他档案”或勾选“停止核心并删除当前配置”，后者提交 `stopCurrent:true`。隔离草稿在侧栏显示错误并禁止打开编辑，但其他档案保持可用。

- [ ] **Step 7: 运行组件、类型和存储守卫测试**

Run: `npm --prefix web run test:unit -- profile && npm --prefix web run typecheck`

Expected: PASS；测试断言档案请求无目录字段；草稿不写 localStorage；旧标签 409 后本地文本仍在；页面显示“草稿已保存”与“需要应用”是不同状态。

- [ ] **Step 8: 提交档案与自动保存界面**

```bash
git add web/src/features/profiles web/src/layout web/src/router.ts web/tests
git commit -m "实现档案自动保存与冲突恢复"
```

### Task 3: 实现八标签配置编辑、模块 JSON 与手工出站节点

**Files:**
- Create: `web/src/features/config-editor/module-map.ts`
- Create: `web/src/features/config-editor/json.ts`
- Create: `web/src/features/config-editor/paths.ts`
- Create: `web/src/features/config-editor/ConfigWorkspace.vue`
- Create: `web/src/features/config-editor/ConfigTabs.vue`
- Create: `web/src/features/config-editor/OverviewPanel.vue`
- Create: `web/src/features/config-editor/StructuredModulePanel.vue`
- Create: `web/src/features/config-editor/ModuleJsonDialog.vue`
- Create: `web/src/features/config-editor/OutboundJsonEditor.vue`
- Create: `web/src/features/config-editor/ReferenceErrors.vue`
- Create: `web/tests/config-json.test.ts`
- Create: `web/tests/config-editor.test.ts`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: `useProfileStore().draft/mutate`；不调用配置导入、订阅或导出 API。
- Produces: 七个固定标签、`setJsonPath(text, path, value)`、`saveModuleBuffer(module, text)` 和出站数组 JSON 编辑器。

- [ ] **Step 1: 写结构化编辑保留未知字段、无效缓冲和出站节点往返测试**

```ts
it("changes one JSON path without deleting unknown fields or changing large integers", () => {
  const input = `{"level":"info","future":{"large":9007199254740993,"nested":[1,"x",true]}}`;
  const output = setJsonPath(input, ["level"], "debug");
  expect(output).toContain(`"level": "debug"`);
  expect(output).toContain(`"large":9007199254740993`);
  expect(output).toContain(`"nested":[1,"x",true]`);
});

it("keeps invalid text and the last valid object", () => {
  const draft = draftFixture(1, { outbounds: [{ type: "direct", tag: "direct" }] });
  const result = applyModuleBuffer(draft, "outbounds", `[{"type":`);
  expect(result.draft.buffers.outbounds).toBe(`[{"type":`);
  expect(result.draft.config.outbounds).toEqual([{ type: "direct", tag: "direct" }]);
  expect(result.error?.fieldPath).toBe("outbounds");
});

it("round-trips manually pasted outbound JSON without masking or field loss", async () => {
  const node = [{ type: "hysteria2", tag: "新加坡 01", server: "sg.example.test", server_port: 443, password: "visible-test-value", future: { x: 1 } }];
  const wrapper = mountEditorWithDraft({ outbounds: [] });
  await wrapper.get("[data-outbound-json]").setValue(JSON.stringify(node, null, 2));
  await wrapper.get("[data-save-module]").trigger("click");
  expect(currentDraft().config.outbounds).toEqual(node);
  expect(wrapper.text()).toContain("visible-test-value");
});
```

- [ ] **Step 2: 运行配置编辑测试并确认失败**

Run: `npm --prefix web run test:unit -- config-json.test.ts config-editor.test.ts`

Expected: FAIL，配置编辑模块尚不存在。

- [ ] **Step 3: 定义模块和标签映射**

```ts
export const configTabs = [
  { id: "overview", label: "概览", modules: ["log"] },
  { id: "dns", label: "DNS", modules: ["dns"] },
  { id: "inbounds", label: "入站", modules: ["inbounds"] },
  { id: "outbounds", label: "出站", modules: ["outbounds"] },
  { id: "route", label: "路由", modules: ["route"] },
  { id: "services", label: "服务", modules: ["services"] },
  { id: "advanced", label: "高级", modules: ["ntp", "certificate", "certificate_providers", "http_clients", "network_namespaces", "experimental"] },
] as const;
```

`$schema` 可在草稿中无损保留和高级 JSON 查看，但不呈现“官方 Schema 校验”或作为正式模块生成。每个低频模块显示存在/缺失状态和“编辑 JSON”入口。

- [ ] **Step 4: 实现不可变路径修改与缓冲解析**

```ts
import { applyEdits, modify, parse, type ParseError } from "jsonc-parser";

export function setJsonPath(text: string, path: readonly (string | number)[], value: unknown): string {
  const edits = modify(text, path, value, { formattingOptions: { insertSpaces: true, tabSize: 2, eol: "\n" } });
  return applyEdits(text, edits);
}

export function applyModuleBuffer(draft: Draft, module: string, text: string): BufferResult {
  const next = structuredClone(draft);
  next.buffers[module] = text;
  if (text.trim() === "") {
    delete next.config[module];
    return { draft: next };
  }
  const errors: ParseError[] = [];
  const value = parse(text, errors, { allowTrailingComma: false, disallowComments: true });
  if (errors.length !== 0) return { draft: next, error: { code: "invalid_json_buffer", message: `JSON 语法错误（offset ${errors[0].offset}）`, fieldPath: module } };
  next.config[module] = value;
  return { draft: next };
}
```

结构化字段直接对 `buffers[module]` 调用 `setJsonPath`，只改目标 token，未识别字段和值（包括超出 JavaScript 安全整数的数字文本）保持原样；随后在内存重新解析该模块供表单刷新。API 保存只提交 buffers/bindings，Config 由服务端重建。无效 buffer 仍调用档案自动保存，但让应用按钮禁用。

- [ ] **Step 5: 实现一期结构化字段和模块 JSON dialog**

结构化编辑只覆盖原型已有且可明确映射的路径：`log.level`；DNS servers/rules/final；inbounds/outbounds/services 数组项目的 `type/tag` 与完整对象 JSON；`route.final/rules/rule_set`；`experimental.clash_api` 与 `cache_file` 的常用字段。任何项目都提供完整 JSON 编辑，保存时验证顶层容器类型并保留未知键。

- [ ] **Step 6: 实现出站手工节点体验并移除代理订阅概念**

出站页顶部明确写“手工粘贴或编辑 sing-box `outbounds` JSON”；编辑器用 `jsonc-parser.parseTree` 确认根节点为 array，格式化用 `jsonc-parser.format` 的文本 edits，保持数字 token 原文；不解析 URL、不发网络请求、不分组为订阅。密码等字段原值显示，代码中没有 mask/reveal/copy whole config 控件。

- [ ] **Step 7: 运行编辑器、类型和无损回归测试**

Run: `npm --prefix web run test:unit -- config && npm --prefix web run typecheck`

Expected: PASS；未来协议和嵌套未知字段保留；无效文本已自动保存但禁止应用；七个标签均存在；出站页没有订阅入口。

- [ ] **Step 8: 提交配置编辑工作区**

```bash
git add web/src/features/config-editor web/src/router.ts web/tests
git commit -m "实现无损配置编辑工作区"
```

### Task 4: 实现规则资源管理和档案绑定

**Files:**
- Create: `web/src/features/rule-resources/types.ts`
- Create: `web/src/features/rule-resources/api.ts`
- Create: `web/src/features/rule-resources/store.ts`
- Create: `web/src/features/rule-resources/RuleResourcesView.vue`
- Create: `web/src/features/rule-resources/RuleResourceDialog.vue`
- Create: `web/src/features/rule-resources/RuleResourceTable.vue`
- Create: `web/src/features/rule-resources/RuleBindingEditor.vue`
- Create: `web/tests/rule-resources.test.ts`
- Modify: `web/src/features/config-editor/StructuredModulePanel.vue`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: 规则资源 CRUD/update API、活动档案 bindings 和自动保存 store。
- Produces: 规则页面、立即更新/禁用/删除、下一次更新时间、引用档案和 route 本地资源绑定编辑。

- [ ] **Step 1: 写只读路径、更新提示、引用删除和绑定保存测试**

```ts
it("never submits localPath and shows a changed update notice", async () => {
  const api = fakeRuleApi({ update: { changed: true, digest: "a".repeat(64), message: "规则资源已更新，请点击“应用”使 sing-box 使用新规则。" } });
  const wrapper = mountRuleView(api);
  await wrapper.get("[data-create-rule]").trigger("click");
  await fillRuleDialog(wrapper, { name: "GeoSite CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "6h" });
  await submitDialog(wrapper);
  expect(api.create).toHaveBeenCalledWith({ name: "GeoSite CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "6h", disabled: false });
  expect(JSON.stringify(api.create.mock.calls[0][0])).not.toContain("localPath");
  await wrapper.get("[data-update-rule]").trigger("click");
  expect(wrapper.text()).toContain("规则资源已更新，请点击“应用”");
});

it("lists referencing profiles when deletion returns 409", async () => {
  const api = fakeRuleApi({ deleteError: apiError(409, "rule_resource_referenced", { profiles: [{ id: "profile-a", name: "家庭出口" }] }) });
  const wrapper = mountRuleView(api);
  await wrapper.get("[data-delete-rule]").trigger("click");
  expect(wrapper.text()).toContain("家庭出口");
  expect(wrapper.text()).toContain("仍在引用");
});
```

- [ ] **Step 2: 运行规则组件测试并确认失败**

Run: `npm --prefix web run test:unit -- rule-resources.test.ts`

Expected: FAIL，规则功能组件尚不存在。

- [ ] **Step 3: 实现 typed store、CRUD 和调度状态**

Resource DTO 完整包含后端字段；create/patch DTO 明确省略 `id/localPath/last* /nextUpdateAt`。页面显示 source/binary、手动或服务端周期、disabled、固定相对路径、最近尝试/成功、摘要缩写、最近错误、下一次更新时间，不伪造规则数或文件大小。

- [ ] **Step 4: 实现立即更新和引用保护反馈**

同一资源更新中禁用其操作按钮；成功按 `changed` 区分提示；失败保留并显示后端原始错误。删除 409 显示所有引用档案并提供跳转，不提供强制删除。

- [ ] **Step 5: 实现档案绑定编辑**

route 配置区可以选择全局资源、填写 sing-box tag、切换 enabled；只写活动草稿的 `{resourceId,tag,enabled}`。UI 预览 `localPath` 但不允许编辑。手工 local/inline `route.rule_set` 继续由模块 JSON 管理；原生 remote 项显示阻止应用错误和“请建立 Web 规则资源并绑定”的修复指引。

- [ ] **Step 6: 运行规则、自动保存和类型测试**

Run: `npm --prefix web run test:unit -- rule-resources profile-store && npm --prefix web run typecheck`

Expected: PASS；规则更新后相关档案出现 needsApply；绑定修改走草稿 ETag；页面没有 `download_detour` 输入；浏览器关闭不影响服务端调度展示。

- [ ] **Step 7: 提交规则资源界面**

```bash
git add web/src/features/rule-resources web/src/features/config-editor web/src/router.ts web/tests
git commit -m "实现规则资源与档案绑定界面"
```

### Task 5: 实现最终模块预览、检查、应用与失败反馈

**Files:**
- Create: `web/src/features/application/types.ts`
- Create: `web/src/features/application/api.ts`
- Create: `web/src/features/application/store.ts`
- Create: `web/src/features/application/ApplicationBar.vue`
- Create: `web/src/features/application/PreviewDialog.vue`
- Create: `web/src/features/application/ValidationErrors.vue`
- Create: `web/src/features/application/ApplyProgressDialog.vue`
- Create: `web/src/features/application/DriftConfirmDialog.vue`
- Create: `web/tests/application.test.ts`
- Modify: `web/src/features/config-editor/ConfigWorkspace.vue`

**Interfaces:**
- Consumes: 活动草稿/保存状态、preview/validate/apply API 和 `/core` operation polling。
- Produces: “应用”主操作、模块级预览、错误跳转、真实阶段、漂移二次确认和中断后状态重载。

- [ ] **Step 1: 写保存门槛、本地规则预览、阶段轮询和漂移确认测试**

```ts
it("disables apply until the active draft is saved and valid", () => {
  const wrapper = mountApplicationBar({ saveState: "dirty", bufferErrors: [], coreState: "running" });
  expect(wrapper.get("[data-apply]").attributes("disabled")).toBeDefined();
  expect(wrapper.text()).toContain("请先等待草稿保存");
});

it("previews generated module files and local rule paths", async () => {
  const api = fakeApplicationApi({ preview: { files: [{ name: "80-route.json", content: { route: { rule_set: [{ type: "local", tag: "GeoSite-CN", format: "binary", path: "data/rulesets/resource-a.srs" }] } }], digest: "a".repeat(64) } });
  const wrapper = mountApplication(api);
  await wrapper.get("[data-preview]").trigger("click");
  expect(wrapper.text()).toContain("80-route.json");
  expect(wrapper.text()).toContain("data/rulesets/resource-a.srs");
  expect(wrapper.text()).not.toContain("https://");
});

it("requires a second user action for external drift", async () => {
  const api = fakeApplicationApi({ applyError: apiError(409, "config_external_drift", { confirmationDigest: "b".repeat(64) }) });
  const wrapper = mountApplication(api);
  await wrapper.get("[data-apply]").trigger("click");
  expect(wrapper.text()).toContain("检测到 config 被外部修改");
  await wrapper.get("[data-confirm-drift]").trigger("click");
  expect(api.apply).toHaveBeenLastCalledWith("profile-a", { revision: 4, confirmedConfigDigest: "b".repeat(64) });
});
```

- [ ] **Step 2: 运行应用组件测试并确认失败**

Run: `npm --prefix web run test:unit -- application.test.ts`

Expected: FAIL，应用 store 和组件尚不存在。

- [ ] **Step 3: 实现预览和检查状态**

预览先 `profileStore.flush()`，只在 saved 后按当前 revision 请求；按后端文件顺序显示 filename 和格式化的原始 JSON，不提供合并、复制或下载。检查显示 Web 语义错误列表或真实 `sing-box check` stdout/stderr，字段值不脱敏；每个带 fieldPath 的错误映射到八标签并可跳转。

- [ ] **Step 4: 实现应用门槛与真实阶段轮询**

门槛依次检查：活动档案、无隔离、saveState=saved、无 buffer 错误、核心已安装、非其他操作。POST apply 在后台 promise 中等待；同时每 300 ms GET `/core`，把 operation.stage 映射为“停止核心/生成配置/检查配置/切换配置/启动核心/提交状态”。请求成功或失败后停止 polling 并刷新 core、profiles、draft。

```ts
const stageLabels: Record<string, string> = {
  snapshot: "准备资源快照",
  stop_core: "停止核心",
  generate_config: "生成配置",
  check_config: "检查配置",
  swap_config: "切换配置",
  start_core: "启动核心",
  commit_state: "提交状态",
  recover: "恢复此前配置",
};
```

- [ ] **Step 5: 处理中断、失败恢复和漂移确认**

网络中断后显示“正在重新读取服务器真实状态”，而非假定失败；立即刷新 core/profile 判断运行档案和 appliedRevision。后端错误显示 stage、message、fieldPath 和原始 output。漂移 dialog 显示外部内容不会导入草稿，确认按钮明确写“覆盖外部 config 并应用”，只重发服务端给出的 digest。

- [ ] **Step 6: 运行应用、档案与错误定位测试**

Run: `npm --prefix web run test:unit -- application profile && npm --prefix web run typecheck`

Expected: PASS；未保存/冲突/缺核心时禁用；成功后 needsApply 清除且显示运行中；失败时显示准确恢复结果；预览无 URL、detour、周期或导出按钮。

- [ ] **Step 7: 提交预览与应用体验**

```bash
git add web/src/features/application web/src/features/config-editor web/tests
git commit -m "实现配置预览与应用流程"
```

### Task 6: 实现真实仪表盘、核心安装更新和设置

**Files:**
- Create: `web/src/features/core/types.ts`
- Create: `web/src/features/core/api.ts`
- Create: `web/src/features/core/store.ts`
- Create: `web/src/features/core/CoreView.vue`
- Create: `web/src/features/core/CoreStatusCard.vue`
- Create: `web/src/features/core/CoreUpdateDialog.vue`
- Create: `web/src/features/dashboard/DashboardView.vue`
- Create: `web/src/features/settings/SettingsView.vue`
- Create: `web/src/features/settings/AppearanceSettings.vue`
- Create: `web/src/features/settings/PasswordSettings.vue`
- Create: `web/tests/core.test.ts`
- Create: `web/tests/dashboard-settings.test.ts`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: `/core`、latest release、update/status、profiles/rules summaries、auth password API。
- Produces: 真实仪表盘、缺失/运行/失败核心页、二次确认式手动下载更新、主题偏好和修改密码。

- [ ] **Step 1: 写无模拟指标、缺核心、确认更新与会话失效测试**

```ts
it("renders only server-backed dashboard facts", () => {
  const wrapper = mountDashboard({ profiles: [profileFixture()], rules: [ruleFixture()], core: coreFixture({ state: "running", pid: 4321, currentProfileId: "profile-a" }) });
  expect(wrapper.text()).toContain("PID 4321");
  expect(wrapper.text()).toContain("家庭出口");
  for (const forbidden of ["实时流量", "内存占用", "连接数", "延迟", "206 ms"]) expect(wrapper.text()).not.toContain(forbidden);
});

it("shows manual download when core is absent", () => {
  const wrapper = mountCoreView(coreFixture({ state: "not_installed", installedVersion: "" }));
  expect(wrapper.text()).toContain("尚未安装 sing-box");
  expect(wrapper.get("[data-download-core]").text()).toContain("手动下载");
});

it("does not start an update before version confirmation", async () => {
  const api = fakeCoreApi({ latest: { version: "1.12.3", updateAvailable: true } });
  const wrapper = mountCoreView(undefined, api);
  await wrapper.get("[data-open-update]").trigger("click");
  expect(api.startUpdate).not.toHaveBeenCalled();
  await wrapper.get("[data-confirm-update]").trigger("click");
  expect(api.startUpdate).toHaveBeenCalledWith({ confirmedVersion: "1.12.3" });
});
```

- [ ] **Step 2: 运行核心/仪表盘测试并确认失败**

Run: `npm --prefix web run test:unit -- core.test.ts dashboard-settings.test.ts`

Expected: FAIL，相关视图尚不存在。

- [ ] **Step 3: 实现真实核心 store 与轮询生命周期**

进入 dashboard/core/application 页面时每 2 秒刷新 `/core`；标签页隐藏时暂停，恢复可见立即刷新。更新中每 500 ms 轮询 `/core/update`，完成后刷新核心和 latest。只显示 API 返回的 version、digest 缩写、state、PID、current profile、startedAt、lastError、operation；不从档案推断 PID 或运行。

- [ ] **Step 4: 实现下载/更新二次确认和阶段**

先主动请求 latest；dialog 显示当前版本、目标稳定版和“不会自动更新”。管理员点击确认才 POST。阶段按后端原值映射为检查、下载、摘要校验、程序验证、配置预检、停止、安装、重启、完成/失败；失败保持并显示后端恢复结果。不得提供 alpha、历史版本或自动更新 toggle。

- [ ] **Step 5: 实现只含真实摘要的仪表盘**

卡片固定为：档案总数和 needsApply 数、规则资源最近成功/下次更新时间、核心安装版本和真实状态、PID、当前档案、最近错误。移除原型中的流量、内存、连接、策略组、延迟、系统代理、TUN 和伪造运行时图表。

- [ ] **Step 6: 实现主题/布局偏好和密码修改**

`localStorage` 键只允许 `singbox-theme`、`singbox-accent`、`singbox-sidebar-collapsed`。密码表单包含当前密码、新密码、确认新密码；成功后 auth store 变 anonymous 并跳登录。设置页不包含自启动、systemd、Caddy、HTTPS、TUN、系统代理、DNS 或防火墙操作。

- [ ] **Step 7: 运行核心、仪表盘、设置与类型测试**

Run: `npm --prefix web run test:unit -- core dashboard settings && npm --prefix web run typecheck`

Expected: PASS；更新必须二次确认；缺核心仍能导航编辑；密码修改后路由回登录；DOM 和 store 不含模拟运行数字。

- [ ] **Step 8: 提交核心与仪表盘页面**

```bash
git add web/src/features/core web/src/features/dashboard web/src/features/settings web/src/router.ts web/tests
git commit -m "接通真实核心状态与手动更新"
```

### Task 7: 固定 capabilities、二期 RuntimeApi 和禁止功能守卫

**Files:**
- Create: `web/src/features/capabilities/types.ts`
- Create: `web/src/features/capabilities/store.ts`
- Create: `web/src/features/runtime/api.ts`
- Create: `web/src/features/runtime/RuntimeUnavailableView.vue`
- Create: `web/tests/capabilities-runtime.test.ts`
- Create: `web/tests/forbidden-features.test.ts`
- Modify: `web/src/layout/AppSidebar.vue`
- Modify: `web/src/router.ts`
- Modify: `internal/capabilities/service.go`
- Modify: `internal/capabilities/handler_test.go`

**Interfaces:**
- Consumes: `GET /capabilities`。
- Produces: 稳定 `RuntimeApi` 接口、一期不可用页和无订阅/导出/模拟控制入口的导航。

- [ ] **Step 1: 写 capabilities 渲染和源码禁止项测试**

```ts
it("shows phase-two runtime capabilities as unavailable", async () => {
  mockCapabilities({ configuration: true, ruleResources: true, application: true, coreUpdate: true, runtime: false, traffic: false, connections: false, proxyGroups: false });
  const wrapper = mount(RuntimeUnavailableView);
  await flushPromises();
  expect(wrapper.text()).toContain("二期尚未接入");
  expect(wrapper.text()).toContain("流量");
  expect(wrapper.find("button[data-runtime-action]").exists()).toBe(false);
});

it("contains no forbidden product routes or calls", () => {
  const sources = readAllSourceFiles(resolve(projectRoot, "src"));
  const joined = sources.map((source) => source.content).join("\n");
  for (const value of ["/subscriptions", "/subscription", "/config/export", "downloadConfig", "copyFullConfig", "mockTraffic", "mockLatency"]) {
    expect(joined).not.toContain(value);
  }
});
```

- [ ] **Step 2: 运行能力/守卫测试并确认旧导航或缺文件导致失败**

Run: `npm --prefix web run test:unit -- capabilities-runtime.test.ts forbidden-features.test.ts`

Expected: FAIL，能力模块不存在或导航尚未满足守卫。

- [ ] **Step 3: 定义不依赖草稿的 RuntimeApi 稳定边界**

```ts
export interface RuntimeStatus { available: boolean; observedAt: string | null; }
export interface TrafficSnapshot { uploadBytes: number; downloadBytes: number; observedAt: string; }
export interface ConnectionSummary { id: string; openedAt: string; }
export interface ProxyGroupSummary { tag: string; selected: string; }
export interface RuntimeEvent { type: "status" | "traffic" | "connections" | "proxyGroups"; observedAt: string; }

export interface RuntimeApi {
  getStatus(): Promise<RuntimeStatus>;
  getTraffic(): Promise<TrafficSnapshot>;
  listConnections(): Promise<ConnectionSummary[]>;
  listProxyGroups(): Promise<ProxyGroupSummary[]>;
  subscribe(listener: (event: RuntimeEvent) => void): () => void;
}

export class UnavailableRuntimeApi implements RuntimeApi {
  private unavailable(): ApiError { return new ApiError(501, { code: "runtime_unavailable", message: "运行数据将在二期接入" }); }
  getStatus(): Promise<RuntimeStatus> { return Promise.reject(this.unavailable()); }
  getTraffic(): Promise<TrafficSnapshot> { return Promise.reject(this.unavailable()); }
  listConnections(): Promise<ConnectionSummary[]> { return Promise.reject(this.unavailable()); }
  listProxyGroups(): Promise<ProxyGroupSummary[]> { return Promise.reject(this.unavailable()); }
  subscribe(_listener: (event: RuntimeEvent) => void): () => void { return () => undefined; }
}
```

RuntimeApi 不导入 profile store；一期 view 不调用这些方法，只根据 capabilities 显示稳定说明。

- [ ] **Step 4: 固定产品导航和后端能力值**

侧栏只含仪表盘、档案配置树、规则资源、核心、运行数据（二期标记）和设置。删除原型订阅、规则中心模板、代理组、连接、测速、模式切换、系统代理等入口。后端一期 capabilities 的 runtime/traffic/connections/proxyGroups 均为 false，配置/规则/应用/coreUpdate 为 true。

- [ ] **Step 5: 扩展禁止项守卫到用户文案与 API client**

守卫扫描 `web/src` 的路由、API path 和导航 label，禁止“订阅配置”“导出配置”“复制完整配置”“测速”“切换模式”与模拟 runtime fixture；允许 `RuntimeUnavailableView` 的“测速将在后续阶段提供”说明，因此文案白名单只限定在该文件并断言没有 button。

- [ ] **Step 6: 运行前后端能力测试与类型检查**

Run: `npm --prefix web run test:unit -- capabilities forbidden && npm --prefix web run typecheck && go test ./internal/capabilities`

Expected: PASS；导航无禁止入口；RuntimeApi 与档案解耦；一期页面无模拟结果。

- [ ] **Step 7: 提交能力边界与导航清理**

```bash
git add web/src/features/capabilities web/src/features/runtime web/src/layout web/src/router.ts web/tests internal/capabilities
git commit -m "固定一期能力与二期运行边界"
```

### Task 8: 完成原型视觉迁移、响应式布局与可访问性验证

**Files:**
- Create: `web/src/styles/layout.css`
- Create: `web/src/styles/components.css`
- Create: `web/src/styles/responsive.css`
- Create: `web/src/components/AppDialog.vue`
- Create: `web/src/components/AppToast.vue`
- Create: `web/src/components/StatusBadge.vue`
- Create: `web/src/components/JsonEditor.vue`
- Create: `web/playwright.config.ts`
- Create: `e2e/harness/main.go`
- Create: `e2e/harness/fakecore.go`
- Create: `e2e/harness/ruleserver.go`
- Create: `e2e/visual.spec.ts`
- Modify: `web/src/main.ts`
- Modify: relevant `web/src/**/*.vue`

**Interfaces:**
- Consumes: `prototype/src/styles.css` 视觉值和现有 Vue 语义 DOM。
- Produces: 1440px 桌面、900px 平板、390px 手机以及 light/dark 两主题可用界面；统一 dialog/toast/status/json editor。

- [ ] **Step 1: 写三断点布局和主题视觉测试**

```ts
for (const scenario of [
  { name: "desktop-light", width: 1440, height: 900, theme: "light" },
  { name: "desktop-dark", width: 1440, height: 900, theme: "dark" },
  { name: "tablet-light", width: 900, height: 1024, theme: "light" },
  { name: "tablet-dark", width: 900, height: 1024, theme: "dark" },
  { name: "mobile-light", width: 390, height: 844, theme: "light" },
  { name: "mobile-dark", width: 390, height: 844, theme: "dark" },
]) {
  test(`${scenario.name} configuration workspace`, async ({ page }) => {
    await page.setViewportSize({ width: scenario.width, height: scenario.height });
    await loginAndOpenProfile(page);
    await setTheme(page, scenario.theme);
    await expect(page.locator("main")).toBeVisible();
    await expect(page.locator("body")).not.toHaveCSS("overflow-x", "scroll");
    await expect(page).toHaveScreenshot(`${scenario.name}-config.png`, { fullPage: true });
  });
}
```

- [ ] **Step 2: 运行视觉测试并保存当前失败基线证据**

Run: `npm --prefix web run test:e2e -- visual.spec.ts`

Expected: FAIL，因为 Playwright 配置、隔离 harness、基线或完成样式尚不存在；保留失败截图在测试输出，不提交临时输出。

- [ ] **Step 3: 建立只供浏览器测试的隔离视觉 harness**

`web/playwright.config.ts` 固定 `testDir:"../e2e"`、`workers:1`、baseURL `http://127.0.0.1:4173`，并给 `webServer` 设置 `cwd:".."`、`command:"go run ./e2e/harness"`。harness 在 `/tmp` 创建随机工作目录、用生产 auth/profile/rules 服务写入固定无敏感值 fixture、安装支持 `version/check/run` 的 fake core，并在 SIGTERM 后停止核心和删除该确切临时目录；它通过测试依赖注入监听 4173，不增加生产 CLI 的监听或目录参数。Task 9 在相同 harness 上扩展故障控制，不另建服务器。

- [ ] **Step 4: 按职责迁移原型布局而非整文件复制**

`layout.css` 只放 app shell/sidebar/topbar/page/dialog；`components.css` 放卡片、表格、表单、状态、JSON editor；`responsive.css` 固定断点 `1100px`（折叠侧栏/平板 drawer）和 `680px`（手机标签横向滚动、底部主操作、全宽底部 dialog）。复用 tokens，不写行内 style。

```css
@media (max-width: 1100px) {
  .app-sidebar { position: fixed; inset: 0 auto 0 0; transform: translateX(-100%); }
  .app-sidebar[data-open="true"] { transform: translateX(0); }
  .app-main { margin-inline-start: 0; }
}

@media (max-width: 680px) {
  .config-tabs { overflow-x: auto; scroll-snap-type: x proximity; }
  .application-bar { position: sticky; inset-block-end: 0; padding-bottom: calc(12px + env(safe-area-inset-bottom)); }
  .app-dialog { align-self: end; width: 100%; max-height: 92dvh; border-radius: 20px 20px 0 0; }
}
```

- [ ] **Step 5: 统一键盘、焦点、dialog 和状态播报**

AppDialog 使用原生 `<dialog>`，打开后聚焦首个字段，Escape 关闭非破坏性操作，删除/应用过程中不可误关；关闭恢复触发按钮焦点。所有图标按钮有 `aria-label`；SaveStatus 与 apply/update 阶段用 `aria-live="polite"`；错误摘要聚焦并链接到字段。

- [ ] **Step 6: 建立视觉基线并手工检查关键页面**

Run: `npm --prefix web run test:e2e -- visual.spec.ts --update-snapshots`

Expected: 生成六张受版本控制截图基线（3 断点 × light/dark 中各计划场景），无横向页面溢出。

手工打开 dashboard、七个配置标签、规则、核心、设置和所有 dialog；在 Chrome 响应式模式检查 1440×900、900×1024、390×844，键盘 Tab/Escape、浅/深主题、侧栏 drawer、手机底部应用栏。把检查清单记录到本计划执行提交说明或 PR。

- [ ] **Step 7: 运行组件、视觉和类型全量验证**

Run: `npm --prefix web run test:unit && npm --prefix web run typecheck && npm --prefix web run test:e2e -- visual.spec.ts`

Expected: PASS；截图与基线一致；无 Vue warning、无水平页面滚动、焦点可见、主题重载保持。

- [ ] **Step 8: 提交响应式视觉迁移**

```bash
git add web/src/styles web/src/components web/src web/playwright.config.ts e2e/harness e2e/visual.spec.ts e2e/visual.spec.ts-snapshots
git commit -m "完成控制台响应式视觉迁移"
```

### Task 9: 完成 E2E、真实 sing-box 契约、文档与一期验收

**Files:**
- Modify: `web/playwright.config.ts`
- Modify: `e2e/harness/main.go`
- Modify: `e2e/harness/fakecore.go`
- Modify: `e2e/harness/ruleserver.go`
- Create: `e2e/auth.spec.ts`
- Create: `e2e/profiles.spec.ts`
- Create: `e2e/rules-apply.spec.ts`
- Create: `e2e/core-update.spec.ts`
- Create: `e2e/security.spec.ts`
- Create: `internal/contracttest/contract_test.go`
- Create: `internal/contracttest/testdata/selection.json`
- Create: `scripts/contract-test.sh`
- Modify: `scripts/build.sh`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `.gitignore`
- Delete: `prototype/index.html`
- Delete: `prototype/package.json`
- Delete: `prototype/src/app.js`
- Delete: `prototype/src/styles.css`

**Interfaces:**
- Consumes: 完整 Go/Vue 应用、隔离 `/tmp` 工作目录、fake core/Release/rule server；契约模式消费显式 `SINGBOX_CONTRACT_BINARY` 或由脚本下载到临时目录的最新官方稳定版。
- Produces: 浏览器验收套件、真实版本/check 契约记录、单命令构建与准确 README。

- [ ] **Step 1: 写完整浏览器场景（先失败）**

```ts
test("password-only login and global session invalidation", async ({ page, browser }) => {
  await page.goto("/");
  await expect(page.locator('input[type="password"]')).toHaveCount(1);
  await expect(page.locator('input[name="username"]')).toHaveCount(0);
  await login(page);
  const second = await browser.newPage();
  await login(second);
  await changePassword(page, "correct horse battery staple", "a second sufficiently long password");
  await second.reload();
  await expect(second).toHaveURL(/\/login$/);
});

test("draft conflict, rule update, apply, profile switch and recovery", async ({ page, browser }) => {
  await login(page);
  const profileA = await createProfile(page, "家庭出口");
  const second = await browser.newPage();
  await login(second);
  await second.goto(`/profiles/${profileA}/config/outbounds`);
  await pasteOutbounds(page, manualNodeFixture());
  await expect(page.getByText("草稿已保存")).toBeVisible();
  await pasteOutbounds(second, secondNodeFixture());
  await expect(second.getByText("服务器草稿已被其他页面更新")).toBeVisible();
  const resource = await createAndUpdateRule(page, "GeoSite Test");
  await bindRule(page, resource, "GeoSite-Test");
  await previewLocalRule(page, `data/rulesets/${resource}.json`);
  await applyProfile(page);
  await expect(page.getByText("运行中")).toBeVisible();
  await expect(page.getByText("规则已更新，请应用")).toHaveCount(0);
  const profileB = await createProfile(page, "备用出口");
  await applyProfile(page, profileB);
  await expect(page.getByText("备用出口")).toBeVisible();
});
```

- [ ] **Step 2: 运行 E2E 并确认 harness 尚未完成**

Run: `npm --prefix web run test:e2e`

Expected: FAIL，Playwright webServer 或场景 helper 尚不存在。

- [ ] **Step 3: 实现隔离 E2E harness**

harness 用 `os.MkdirTemp("", "singbox-web-e2e-*")` 创建工作目录，直接调用 auth Store 设置固定测试密码，通过依赖注入让 app 监听 `127.0.0.1:4173`；fake core 支持 version/check/run、调用顺序、观察期失败和优雅退出；本地 HTTP server 同时提供规则 JSON、稳定 Release 元数据、checksums 和安全 tar.gz。退出时关闭 server/process 并删除明确的临时目录。测试不得访问公网或仓库 `temp/`。

- [ ] **Step 4: 覆盖规格中的浏览器与安全流程**

补齐以下独立 tests：登录/退出/修改密码和无用户名；创建/改名/删除/刷新恢复；多标签冲突/明确覆盖；离线恢复；规则 CRUD/立即更新/调度状态/引用删除；未知出站字段往返；预览/语义错误跳转/check 失败；应用成功/恢复/切换；核心缺失下载和更新成功/失败；Cookie/CSRF/安全头/SPA 路由；运行页无模拟数据；无订阅与配置导出入口。

- [ ] **Step 5: 写真实二进制契约测试和选择记录**

```go
//go:build contract

func TestEmbeddedTemplatePassesManagedSingBoxCheck(t *testing.T) {
    binary := os.Getenv("SINGBOX_CONTRACT_BINARY")
    if binary == "" { t.Fatal("SINGBOX_CONTRACT_BINARY is required") }
    version := runBounded(t, binary, "version")
    digest := sha256File(t, binary)
    selection := readSelection(t, "testdata/selection.json")
    if version != selection.Version || digest != selection.SHA256 { t.Fatalf("contract binary mismatch: version=%s sha256=%s", version, digest) }
    workDir := t.TempDir()
    draft, snapshot := createContractDraftAndLocalRule(t, workDir)
    output := generateContractConfig(t, draft, snapshot)
    configDir := filepath.Join(workDir, "config")
    writeOutput(t, output, configDir)
    result := runBounded(t, binary, "check", "-C", configDir)
    t.Logf("sing-box=%s sha256=%s output=%s", version, digest, result)
}
```

首次选版显式运行 `scripts/contract-test.sh --refresh`：它使用生产 `updater.ReleaseClient/Fetcher` 查询当时的官方最新稳定版、下载到 `mktemp -d`，把 version、Release tag、资产名/官方 URL、checksums URL、archive SHA-256 和 binary SHA-256 写入 `internal/contracttest/testdata/selection.json`。普通 `scripts/contract-test.sh` 只读取这份已提交选择，下载精确记录的官方资产并复验两层摘要，再设置 `SINGBOX_CONTRACT_BINARY` 执行带 `contract` tag 的测试，最后删除临时目录；因此未来 latest 变化不会悄然改变契约版本。脚本需要网络时由执行者明确批准，不提交二进制。

`createContractDraftAndLocalRule` 从嵌入 ProfileSeed 创建草稿，调用基础 rules Store 为全部默认 binding 分配临时安装 ID，并用生产 Downloader 从每个嵌入默认 URL 下载到该测试工作目录的固定 `data/rulesets/<id>.<json|srs>`；随后取得持锁 snapshot 交给生产 Generator。它不读取 `temp/config.default.json`，不使用真实代理节点凭据，并且所有网络响应都经过与产品相同的协议、大小、摘要和格式检查。

- [ ] **Step 6: 运行 E2E 和真实契约并修复实际兼容差异**

Run:

```bash
sh scripts/build.sh
npm --prefix web run test:e2e
sh scripts/contract-test.sh --refresh
sh scripts/contract-test.sh
```

Expected: PASS；E2E 只启动 fake core；契约测试日志包含确切官方稳定版与 SHA-256，只调用真实 binary 的 `version` 和 `check`，不调用 `run` 或改变系统网络。若真实 check 发现默认模板字段不兼容，修改受版本控制模板或基础 Web 语义实现，并增加对应回归测试后重跑。

- [ ] **Step 7: 重写仓库说明并退役旧原型运行入口**

README 包含：前置 Go 1.26/Node 22；`npm --prefix web install`；`sh scripts/build.sh`；交互式 `./dist/singbox-web admin set-password`；`./dist/singbox-web serve`；默认 `127.0.0.1:8080`；固定工作目录树与测试命令。Vue 视觉和交互测试全部通过后删除 `prototype/`，避免仓库继续提供含已否决入口的第二套可运行页面；视觉历史仍由 Git 和 E2E 截图保留。同步更新 `AGENTS.md` 的目录、构建和验证命令为 Go/Vue 真实应用。明确不包含 systemd/Caddy/HTTPS/公网部署、订阅、配置导出、自动核心更新和高级运行控制。

- [ ] **Step 8: 执行规格覆盖与禁止项最终审计**

Run:

```bash
rg -n "订阅配置|/subscriptions|downloadConfig|copyFullConfig|mockTraffic|mockLatency" web/src internal cmd
rg -n "configDir|dataDir|rulesDir|binaryPath" web/src internal/httpapi internal/profile internal/rules
rg -n "systemd|Caddy|HTTPS|自动更新" web/src
```

Expected: 第一条无结果；第二条只在服务端内部固定 layout 类型或测试拒绝断言中出现，不在 request DTO/表单出现；第三条只在“非范围/不提供”说明中出现，无可执行控件。

- [ ] **Step 9: 运行一期完整验证**

Run:

```bash
go test -race ./...
go vet ./...
npm --prefix web run typecheck
npm --prefix web run test:unit
sh scripts/build.sh
npm --prefix web run test:e2e
sh scripts/contract-test.sh
git status --short
```

Expected: 所有命令直接可运行且退出码为 0；契约脚本下载并使用与本次 `selection.json` 一致的真实测试二进制；没有残留子进程、staging、rollback、下载临时目录或未跟踪运行数据；`git status --short` 只显示本任务预期文档/测试改动。

- [ ] **Step 10: 提交一期产品与验收套件**

```bash
git add web e2e internal/contracttest scripts README.md AGENTS.md .gitignore prototype
git commit -m "完成一期管理页面与验收测试"
```
