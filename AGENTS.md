# Repository Guidelines

## Project Structure & Module Organization

The tracked application is a zero-dependency browser prototype under `prototype/`. Its entry point is `prototype/index.html`; application state, rendering, and event handling live in `prototype/src/app.js`, while global and responsive styles live in `prototype/src/styles.css`. Product and architecture decisions are documented in `docs/superpowers/specs/`. Keep `README.md` aligned with user-facing setup changes. `temp/`, `node_modules/`, and `*.log` are ignored scratch or generated content and must not become runtime dependencies.

## Build, Test, and Development Commands

- `npm --prefix prototype run dev` starts a Python HTTP server on port 4173.
- `npm --prefix prototype run preview` provides the same local preview command.
- `npm --prefix prototype run check` runs `node --check` against `src/app.js`.

Open <http://localhost:4173> after starting the server. The prototype has no install step, bundler, or production build. Use `-- --bind 0.0.0.0` only when LAN access is intentional.

## Coding Style & Naming Conventions

Use two-space indentation in HTML, CSS, JavaScript, and JSON. Follow the existing JavaScript style: strict mode, semicolons, double-quoted strings, `const` by default, and `camelCase` for functions and variables. Use `kebab-case` CSS class names and define reusable colors, spacing, and dimensions as custom properties in `:root`. Preserve Chinese UI copy and responsive behavior. No formatter or linter is configured, so keep diffs focused and match surrounding code.

## Testing Guidelines

There is currently no automated test framework or coverage threshold. Run `npm --prefix prototype run check` for every change. For UI work, manually verify desktop, tablet, and mobile layouts; light and dark themes; navigation; dialogs; and relevant `localStorage` persistence. Record those checks in the pull request. If adding a test framework, place tests under `prototype/tests/`, use `*.test.js`, and add an `npm test` script in the same change.

## Commit & Pull Request Guidelines

History uses short Chinese, action-oriented commit subjects, such as `迁移交互原型到独立目录`. Keep each commit limited to one concern. Pull requests should include a concise summary, affected screens or flows, validation commands and results, and linked issues when applicable. Include before/after screenshots for visual changes, covering desktop and mobile when layout changes. Never commit real credentials, private endpoints, logs, or files from `temp/`.

## Security Notes

This is an interaction prototype with simulated data, not a production control plane. Do not add real sing-box secrets or operational data to examples, browser storage, screenshots, or documentation.
