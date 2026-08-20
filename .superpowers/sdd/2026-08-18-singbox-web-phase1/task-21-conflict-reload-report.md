# Task 21：草稿冲突重载修复报告

## 结论

“重新载入服务器版本”现在直接丢弃本地冲突内容并重新获取服务器草稿；该明确选择不再触发原生 `window.confirm`。保留“用本地内容覆盖”路径的原有安全行为。

## RED / GREEN 证据

- RED：`npm --prefix web exec -- vitest run src/stores/draft.test.ts` 中，新测试检测到 `window.confirm` 被调用 1 次。
- GREEN：同一命令通过，7 个测试全部通过。
- 类型检查：`npm --prefix web run typecheck` 通过。

## Fix round 1：ETag 回归覆盖

- 在 `reloadAfterConflict` 成功路径断言 `draft.etag === '"draft-5"'`，覆盖载入服务器草稿时同步更新 ETag。
- `npm --prefix web exec -- vitest run src/stores/draft.test.ts` 通过（7/7）。
- `npm --prefix web run typecheck` 通过。
