# Task 21：前端 API `/api/v1` 前缀修复报告

## TDD

### RED

命令：

```bash
npm --prefix web test -- client.test.ts
```

结果：退出码 `1`。新增回归测试失败，断言显示：

```text
Expected: "/api/v1/auth/session?refresh=true"
Received: "/auth/session?refresh=true"
```

其余 `118` 个测试通过；总计 `1` 个失败、`118` 个通过。

### GREEN

命令：

```bash
npm --prefix web exec -- vitest run src/api/client.test.ts
```

输出：

```text
✓ web/src/api/client.test.ts (7 tests)
Test Files  1 passed (1)
Tests       7 passed (7)
```

退出码 `0`。

## 修改文件

- `web/src/api/client.ts`：在唯一的 `fetch` 入口规范化路径；为未带版本前缀的相对 API 路径添加 `/api/v1/`，保留已带 `/api/v1` 或 `/api/v1/` 的路径。
- `web/src/api/client.test.ts`：覆盖普通路径携带查询参数和已带版本前缀的路径。

## 自审

- 查询字符串未重组或编码，直接随路径保留。
- 已有 `/api/v1/...` 路径不会重复加前缀。
- 原有错误解析、401 清理/事件、ETag、CSRF、凭据设置逻辑未改动，且聚焦测试全数通过。
- 已运行 `git diff --check`，无空白错误。
