# Repository Guidelines

## Project Structure & Module Organization

The application is a Go local service plus a Vue SPA under `web/`. Go commands live in `cmd/singbox-web`; packages are in `internal/`; built SPA assets are embedded from `internal/webui/assets/`. Browser tests and their isolated harness live in `web/e2e/` and `e2e/harness/`. Product and architecture decisions are documented in `docs/superpowers/specs/`. Keep `README.md` aligned with user-facing setup changes. `data/`, `config/`, `dist/`, `node_modules/`, and `*.log` are generated or runtime-only and must not be committed.

## Build, Test, and Development Commands

- `sh scripts/build.sh` builds the SPA into embedded assets then compiles `dist/singbox-web`.
- `npm --prefix web run typecheck` type-checks the Vue application.
- `npm --prefix web run test:unit` runs component and store tests.
- `npm --prefix web run test:e2e` runs Playwright against the isolated harness.
- `go test -race ./...` and `go vet ./...` validate Go code.

The production service defaults to `0.0.0.0:50010`. Do not change this listener convention without explicit user direction. The SPA has an npm install step and must be built through `scripts/build.sh` before running the compiled service.

## Coding Style & Naming Conventions

Use two-space indentation in HTML, CSS, TypeScript, JavaScript, and JSON. Follow the existing JavaScript/TypeScript style: semicolons, double-quoted strings, `const` by default, and `camelCase` for functions and variables. Go code follows standard `gofmt`. Use `kebab-case` CSS class names and define reusable colors, spacing, and dimensions as custom properties in `:root`. Preserve Chinese UI copy and responsive behavior.

## Testing Guidelines

Run `go test -race ./...`, `go vet ./...`, `npm --prefix web run typecheck`, and `npm --prefix web run test:unit` for relevant changes. UI work also requires `npm --prefix web run test:e2e` and manual desktop/tablet/mobile, light/dark, navigation, dialog, keyboard-focus checks. Contract tests are run with `sh scripts/contract-test.sh` only when its checked-in official binary selection is current and available.

## Commit & Pull Request Guidelines

History uses short Chinese, action-oriented commit subjects, such as `迁移交互原型到独立目录`. Keep each commit limited to one concern. Pull requests should include a concise summary, affected screens or flows, validation commands and results, and linked issues when applicable. Include before/after screenshots for visual changes, covering desktop and mobile when layout changes. Never commit real credentials, private endpoints, logs, or files from `temp/`.

## Security Notes

This is an interaction prototype with simulated data, not a production control plane. Do not add real sing-box secrets or operational data to examples, browser storage, screenshots, or documentation.
