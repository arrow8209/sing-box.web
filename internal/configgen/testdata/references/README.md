# 引用解析兼容契约

`legacy.json` 是纯解析器的输入和完整模块期望值；`filters.json` 是过滤兼容矩阵。所有资源均为合成数据。T2 的 Go 测试逐项执行这些样例，并检查输入不变及重复解析幂等。

| 行为 | 约定 |
| --- | --- |
| 无 editorState / 无来源的旧入站 | 保持：独立输入，不按名称或内容猜来源 |
| 订阅字段删除、增删节点 | 保持：删除 generatedTags 全集后重建，独立 JSON 保留 |
| 多组复用、过滤、源顺序、重名 | 保持：组与引用遍历顺序、源顺序；同源同索引只生成一次；冲突依次使用 name-2、name-3 |
| 空 include / exclude | 保持：分别为不限制 / 不排除 |
| 普通文字及 `|` 分隔的普通文字 | 按子串匹配，节点名称含国旗或表情也可正常筛选 |
| JS 非法正则 | 保持：可确认非法时按原文进行字面匹配，包含 `[`、`(?i)hk` |
| JS 可用、Go 不支持或语义有差异 | 明确报错：lookaround、反向引用、Unicode/identity escapes 等不静默改义；改用普通文字或受支持的字符集合 |
| 单节点或来源消失 | 明确报错：不使用 keepFallback，不输出半份候选 |
| runtimeTags / 组类型矛盾 | 明确报错：定位 JSON 字段 |
| 已确认归属节点的字段被手改 | 有意改变：按源重新生成，自动重载也不保留该手改；需先转独立 |
| 缺 generatedTags、损坏元数据、未知版本 | 有意改变：明确报错，不降级成独立输入 |
| 组 default 不再是成员 | 有意改变：明确报错，不自动选择替代节点 |
| 关联入站 | 保持本地 tag；本地 tag 已删除则忽略旧绑定；structured 剥离五个遗留字段，custom 保留 |
| 托管 sniff | 新增插在开头，已有规则原位更新/删除；只认与保存的归属快照完全相等的规则；有冲突的手工 sniff 不修改 |
| 未知字段 / 大整数 | 保留原始 JSON token 的值，禁止经过 float64 或 JS number 重写候选 |

错误统一为 `stage: resolve_references`，包含 `code` 和 `fieldPath`；message 不包含源内容、地址或凭据。代码包括 `reference_source_missing`、`reference_source_unavailable`、`reference_node_missing`、`reference_filter_unsupported`、`reference_conflict`、`reference_metadata_invalid`、`reference_ownership_missing`、`reference_version_unsupported`。字段路径从旧 `editorState` / 模块路径开始，新 schema 迁移时使用 `references` 路径。

摘要使用 SHA-256：入站输入为 mode 与完整 config 原文；订阅为有序 outbounds 原文（名称和更新时间不影响配置）；使用 `inbound:<id>`、`subscription:<id>`、`rule:<id>` 区分类型。规则已有裸 ID 摘要入口仅作为旧记录兼容，不允许一次局部核对清空其他类型。

检测关联 draft revision、resourceDigests 与 configDigest，每次应用仍重新检查。规则文件锁只覆盖生成和核心检查，不承诺提交之后的文件内容冻结。
