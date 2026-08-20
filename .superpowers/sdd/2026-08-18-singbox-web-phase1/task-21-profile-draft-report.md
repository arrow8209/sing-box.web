# Task 21：创建档案后的草稿载入修复

## RED

- 新增回归测试覆盖创建完成后必须先设置 `activeId`、GET 新档案草稿并保留 ETag，最后才导航。
- 修复前测试失败：导航时 `activeId` 为 `null`。

## GREEN

- `profiles.create` 在追加服务端返回的档案后复用 `select(data.id)`；该既有路径设置活动档案并载入草稿。
- 草稿载入成功后才导航至 `/config/:id/general`。

## 验证

- `npm --prefix web exec -- vitest run src/stores/profiles.test.ts`：6/6 通过。
- `npm --prefix web test`：29 个测试文件、121 个测试通过。
- `npm --prefix web run typecheck`：通过。

## 自审

- 改动限定于创建档案后的选择、草稿载入与导航时序；未修改路由守卫或创建对话框错误处理，因此 POST 创建失败仍会由调用方捕获且对话框保持打开。
- 完整测试的既有 Vue Router/injection 警告未导致测试失败，未在本任务范围内处理。
