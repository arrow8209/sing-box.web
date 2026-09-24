# 配置引用职责迁移阶段验收

日期：2026-09-24。执行依据：[开发计划](2026-09-24-reference-resolution-development-plan.md)与[职责方案](../frontend-backend-responsibilities-and-migration.md)。测试使用合成数据、隔离工作目录和假核心；未使用 `Release/` 运行数据。

| 编号 | 结果 | 对应验证 |
| --- | --- | --- |
| A1 | 通过：直接 API 使用新订阅，删除字段不残留 | `TestReferenceCompatibilityFixtures/replace-generated-set`、`TestReferencesDirectOperationsAndAutoReload`、`reference-sync.spec.ts` |
| A2 | 通过：整订阅、单节点、多组复用、过滤和同名分配结果稳定；不兼容过滤报字段错误 | `TestReferenceCompatibilityFixtures`、`TestReferenceSharedAliasesAndSourceOrder`、`TestReferenceFilterCompatibility` |
| A3 | 通过：入站改名、端口、structured 字段剥离与多条 sniff 归属保持；手工规则不被重排 | `TestReferenceSniffOwnershipAndPositions`、`TestMigrationKeepsIndependentSniffAndRoutePositions`、`inbounds.spec.ts` |
| A4 | 通过：源变更与重启后同 revision 检测过期，其他档案不受影响；资源保存不直接重载 | `TestResourceDigestsMergeAndValidationExpiresWithoutDraftRevision`、入站/订阅资源服务 change hook 测试 |
| A5 | 通过：自动重载开启时，预览和检测不发送内部草稿 PUT，revision 与已应用摘要不变，核心状态不变 | `reference-operations.spec.ts`、`application.test.ts` |
| A6 | 通过：手动应用只提交一次，本请求的已验证 JSON 被复制，不额外重启 | `reference-operations.spec.ts`、`TestReferencesDirectOperationsAndAutoReload` |
| A7 | 通过：用户正常保存与自动重载按当前/非当前档案及核心运行状态执行 | `TestAutoReloadAppliesCurrentProfileAndReplacesRunningCore`、`TestAutoReloadSkipsNonCurrentProfile`、`TestAutoReloadAppliesCurrentProfileWithoutStartingStoppedCore` |
| A8 | 通过：候选后解绑/复制固定签名原文，大整数与新值保持；无旧副本时明确报错 | `TestCandidateFreezeUsesDisplayedRawTextAndETag`、`TestVersionTwoFreezeAndExplicitBackupRecovery`、`TestExplicitSavedCopyRecoveryAfterSourceDeletion`、`inbounds.spec.ts` |
| A9 | 通过：未保存编辑、迟到候选、档案切换及 revision 冲突不覆盖新输入 | `application.test.ts`、`TestSaveDraftRejectsStaleRevision`、`TestCandidateFreezeUsesDisplayedRawTextAndETag` |
| A10 | 通过：规则文件锁维持原边界；检查后摘要变化使较旧应用保持过期 | `TestRuleDigestChangedAfterCheckKeepsOlderApplyStale`、`TestSnapshotRejectsResourceFileDrift`、`TestConcurrentSnapshotsWithReverseBindingsDoNotDeadlock` |
| A11 | 通过：v1/v2、复制、重复迁移、损坏元数据、旧客户端拒写与隔离回退演练 | `TestLegacyMigrationPreservesIndependentRawJSONAndBacksUpDraft`、`TestDamagedLegacyMetadataCannotBeSilentlyMigrated`、`TestVersionTwoReferencesGenerateWithoutSavedExpansion`；`scripts/restore-profile-v1.py` 在 `/tmp` 隔离数据上恢复并归档 v2 修改 |
| A12 | 通过：运行成员与选择由核心 API 决定；停止时显示明确的待运行候选并保留 `hidden` | `outbounds.spec.ts`、`proxy-selection.spec.ts`、`dashboard-proxy-selector.test.ts`、桌面/平板/手机浅色与深色视觉用例 |

验证结果：Go 全量竞态测试、`go vet ./...`、前端类型检查全部通过；前端单测 140/140、浏览器用例 34/34 通过。验证命令：`go test -race ./...`、`go vet ./...`、`npm --prefix web run typecheck`、`npm --prefix web run test:unit`、`npm --prefix web run test:e2e`，以及版本号递增后的 `sh scripts/build.sh`（`dist/singbox-web-v0.1.7`）。视觉检查覆盖桌面、平板、手机、浅色、深色、导航、对话框和键盘焦点。构建产物只保留在 `dist/`，不操作 `Release/`。

官方核心契约测试依项目规则本次不执行：仓库选择记录为 `1.13.19`，2026-09-24 官方最新稳定版为 `1.14.1`，不满足“签入选择当前且可用”的条件。此次 Go 集成测试与浏览器测试使用项目假核心；目标环境仍应使用对应正式核心执行最终配置检查。

回退边界：M2 的旧格式可由服务端适配器读取；M3 写入 v2 后，旧客户端不能覆盖新引用。迁移前备份保存在 `data/drafts/migration-backups/`，离线回退脚本先归档 v2 草稿、索引、核心状态和正式配置，再恢复 v1 草稿并清除已应用标记。迁移后的编辑不会自动并入旧格式，需从归档核对后手工处理。规则文件在检查后仍可由调度器更新；此时 `needsApply` 保持为真，不宣称规则文件随已提交 JSON 一起冻结。
