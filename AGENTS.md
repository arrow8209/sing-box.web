# Repository Guidelines

## Project Structure & Module Organization

The application is a Go local service plus a Vue SPA under `web/`. Go commands live in `cmd/singbox-web`; packages are in `internal/`; built SPA assets are embedded from `internal/webui/assets/`. Browser tests and their isolated harness live in `web/e2e/` and `e2e/harness/`. Product and architecture decisions are documented in `docs/`, including `docs/reference-schema-v2.md` and `docs/plans/2026-09-24-reference-resolution-acceptance.md`. Keep `README.md` aligned with user-facing setup changes. `data/`, `config/`, `dist/`, `Release/`, `node_modules/`, and `*.log` are generated or runtime-only and must not be committed.

## 配置引用职责与兼容边界

- v2 草稿的 `references` 是订阅、关联入站和策略组引用的权威业务输入；`buffers` 保留独立 JSON 与非派生配置，`editorState` 只保存展示状态，`bindings` 继续保存规则集绑定。旧 v1 档案读取时只在内存中适配，保存或复制时才迁移；旧客户端不得覆盖 v2 引用。格式与回退契约见 `docs/reference-schema-v2.md`。
- `internal/configgen` 使用档案和入站、订阅、规则快照生成同一候选，供预览、检测、手动应用和自动重载复用。前端不得重新实现订阅展开、节点归属重建或托管 sniff 生成，也不得为这些操作同步 PUT 派生草稿。应用复制本次检查通过的 JSON；源资源更新只标记相关档案过期，不直接重载核心。
- 候选原始 JSON 以字符串保留未知字段和大整数。关联入站解除关联或复制时使用同一修订的签名候选和 `If-Match`；来源失效时不得自动回退旧副本。历史数据回退只在隔离目录演练，使用 `scripts/restore-profile-v1.py` 前须停止服务，并保留迁移后的编辑归档。

## Build, Test, and Development Commands

- `sh scripts/build.sh` builds the SPA into embedded assets then compiles `dist/singbox-web-v<version>`, using `web/package.json` by default or the `VERSION` environment variable.
- `npm --prefix web run typecheck` type-checks the Vue application.
- `npm --prefix web run test:unit` runs component and store tests.
- `npm --prefix web run test:e2e` runs Playwright against the isolated harness.
- `go test -race ./...` and `go vet ./...` validate Go code.

The Playwright harness serves assets embedded by Go. Run `npm --prefix web run build` before browser tests after SPA changes, so E2E exercises the current frontend rather than previously embedded assets. This development build does not replace the versioned release command `sh scripts/build.sh`.

用户指定的端口约定如下，后续开发、配置和发布以此为准：

| 环境 | Web 访问端口 | Mixed 入站端口 |
| --- | --- | --- |
| 发布环境 | `50015` | `50016` |
| 开发环境 | `50018` | `50019` |

Web 服务监听地址保持 `0.0.0.0`。未经用户明确指示，不要更改以上端口约定。

The SPA has an npm install step and must be built through `scripts/build.sh` before running the compiled service.

## 发布与运行目录

- 发布产物固定输出并保留在 `dist/`，文件名包含版本号：`dist/singbox-web-v<version>`。
- `Release/` 是由用户自行维护的最终运行目录。本项目发布只负责更新版本并生成、验证 `dist/` 产物；发布时不操作 `Release/`，也不管理其中的运行服务。用户自行处理部署和运行，正式运行的 `data/` 和 `config/` 位于 `Release/` 下。
- 每次发布前必须更新并递增版本号，不得复用上一发布版本号。任何代码修改后重新发布（包括小修复）都必须增加版本号，不得用修改后的代码覆盖已发布的同版本产物。用户未指定目标版本时，默认递增补丁版本号。
- 本项目同步更新 `web/package.json` 和 `web/package-lock.json` 中的版本，并保持产物文件名一致。可用 `npm --prefix web version patch --no-git-tag-version` 更新版本后再构建；正式发布使用 `VERSION` 时也必须与版本文件保持一致。

## Coding Style & Naming Conventions

Use two-space indentation in HTML, CSS, TypeScript, JavaScript, and JSON. Follow the existing JavaScript/TypeScript style: semicolons, double-quoted strings, `const` by default, and `camelCase` for functions and variables. Go code follows standard `gofmt`. Use `kebab-case` CSS class names and define reusable colors, spacing, and dimensions as custom properties in `:root`. Preserve Chinese UI copy and responsive behavior.

## Testing Guidelines

Run `go test -race ./...`, `go vet ./...`, `npm --prefix web run typecheck`, and `npm --prefix web run test:unit` for relevant changes. UI work also requires `npm --prefix web run test:e2e` and manual desktop/tablet/mobile, light/dark, navigation, dialog, keyboard-focus checks. Contract tests are run with `sh scripts/contract-test.sh` only when its checked-in official binary selection is current and available.

## Commit & Pull Request Guidelines

History uses short Chinese, action-oriented commit subjects, such as `迁移交互原型到独立目录`. Keep each commit limited to one concern. Pull requests should include a concise summary, affected screens or flows, validation commands and results, and linked issues when applicable. Include before/after screenshots for visual changes, covering desktop and mobile when layout changes. Never commit real credentials, private endpoints, logs, or files from `temp/`.

## Security Notes

The local service can apply configurations and control a sing-box process. Use synthetic resources and isolated working directories for tests and migration rehearsals; do not use `Release/` runtime data. Never commit real credentials, private endpoints, or operational data. Do not commit browser traces or `test-results/`; checked-in visual baselines must use synthetic data.
