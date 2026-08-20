# Task 21 原始 JSON 模块完整标签修复报告

## RED

将 `ConfigWorkspace.test.ts` 的契约从短标签更新为完整 descriptor 标签后，运行：

```text
npm --prefix web test -- --run src/features/config/ConfigWorkspace.test.ts
```

测试按预期失败：页面仍暴露 `编辑概览 JSON`，无法找到 `编辑配置概览 JSON`。

## GREEN

- `ConfigWorkspace.vue` 的按钮文本、按钮 name 和 raw 区域无障碍名称改用 `activeDescriptor.label`。
- `RawModuleEditor.vue` 接收可选完整模块 label，并将 `<textarea>` 的 `aria-label` 暴露为 `<完整标签> JSON`。
- 组件测试验证 `编辑配置概览 JSON`、`配置概览 JSON` 及 textarea 无障碍名称。

验证结果：

```text
npm --prefix web test -- --run src/features/config/ConfigWorkspace.test.ts
29 test files passed, 121 tests passed
npm --prefix web run typecheck
通过
```

## 自审

仅修改配置工作区、原始编辑器及其最小组件测试；其他模块继续由各自 descriptor 的完整 label 生成名称。未运行服务器、E2E、watch 或交互程序。

## 提交

提交：`修复原始 JSON 编辑器模块完整标签`（中文提交）
