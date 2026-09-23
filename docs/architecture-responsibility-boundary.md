# 控制台职责边界：现状分析与重构方案

日期：2026-09-23

对照设想：

1. 前后端分离：前端展示/修改数据，后端处理数据。
2. 大部分数据处理逻辑在后端。
3. 页面彼此独立；耦合只应是轻量数据耦合，不是业务逻辑耦合。
4. 一次修改需求应只改一个局部。

## 1. 结论

基础设施已经前后端分离，领域处理没有。

Go 服务负责认证、草稿落盘、规则下载、`sing-box check`、应用复制、核心进程。这些符合一期规格（`docs/superpowers/specs/2026-08-18-singbox-web-phase1-design.md`）。

但配置的“变成可应用的 sing-box JSON”发生在浏览器里：

- 订阅引用展开成 `outbounds` 数组
- 入站资源库关联写回档案草稿
- 旧嗅探字段迁移成 `route.rules` 的 `sniff` 动作
- 策略组 `editorState` 与 JSON 文本双写
- 预览/检测/应用前的引用同步（`cb6b6d0`，20 个文件）

页面在路由上是分开的，数据上共用一份可变 `Draft`（`config` + `buffers` + `editorState`）。配置工作台再把档案 CRUD、应用栏、入站/出站/路由/DNS 编辑器编在同一个 Vue 树里。所以“改一块”会穿过多个 feature、store、单测和 e2e。

规则集已经按你的设想工作：档案只存 `resourceId + tag`，生成时由后端注入本地 `route.rule_set`。入站和订阅没有走这条路。

这不只是结构难看，还有正确性缺口：`preview` / `validate` / `apply` / 自动重载都从**已经保存的草稿修订**起步。引用同步只发生在浏览器点按钮之前。API 客户端、过期标签页、或自动重载一份尚未同步的草稿时，后端会把过期的 inbounds/outbounds 送进 `sing-box check`。`hidden` / `include` / `exclude` / `icon` 若漏进 JSON，除 `__id_in_gui` 外后端不会剥离。

## 2. 四点对照

| 设想 | 现状 | 偏差 |
| --- | --- | --- |
| 前端展示/改数据，后端处理 | 前端改 JSON 文本和 `editorState`，并执行展开/迁移/同步；后端把已经展开的草稿写成多文件配置 | 处理层在 Vue |
| 数据处理在后端 | 后端：落盘、规则注入、语义检查、check、复制、核心。前端：`outbound.ts` 722 行、`dns.ts` 432 行、`route.ts` 386 行、`inbound.ts` 251 行、`references.ts` 130 行 | 领域算法在 TS |
| 页面独立，只做数据耦合 | 路由独立。`config-editor` 依赖 application / inbounds / profiles / rule-resources / subscriptions；`application` 再依赖上述模块做同步 | 业务逻辑耦合 |
| 一次需求改一处 | 刚合入的 `cb6b6d0`「同步配置引用」改了 20 个文件、+748 行。历史功能提交常 8–45 个文件 | 改动放大器叠加 |

一期规格本来就把结构化表单和 JSON 路径编辑放在前端，后端负责生成/检查/应用。后来加了入站资源库、订阅、策略组、嗅探迁移、引用同步，这些本应像规则集一样变成“ID 绑定 + 后端展开”，却继续堆进 Vue，因为 `editorState` 被定义成后端不解释的 UI 附件。

## 3. 当前数据流

```
浏览器
  ├─ 表单改 buffers / editorState
  ├─ TS：展开订阅、同步入站、迁移 sniff、改 route
  ├─ PUT /profiles/{id}/draft     （已经是“成品 JSON”）
  └─ POST preview | validate | apply
Go
  ├─ 用 buffers 重建 config
  ├─ 注入规则集本地路径（唯一的后端展开）
  ├─ 语义检查 + sing-box check
  └─ 复制到 config/
```

目标数据流：

```
浏览器
  ├─ 表单改可见字段和引用 ID
  ├─ PUT /profiles/{id}/draft     （引用，不是展开结果）
  └─ POST preview | validate | apply
Go DraftCompiler
  ├─ 读入站资源、订阅、规则快照
  ├─ 展开引用、注入 sniff、剥离 GUI 字段
  ├─ 生成多文件配置
  └─ check / 应用
```

关键不对称：`internal/configgen/generator.go` 的 `injectLocalRuleSets` 已经按绑定生成规则集；入站/订阅没有对等编译器。后端 `internal/inbounds/model.go` 仍写着“导入后切断与资源 ID 的关系”，而 `cb6b6d0` 又在 `editorState.inbounds.resourceBindings` 里恢复了这条关系。两套模型叠在一起。

## 4. 页面耦合

前端 feature 跨模块 import：

- `application` → `config-editor`, `inbounds`, `profiles`, `subscriptions`
- `config-editor` → `application`, `inbounds`, `profiles`, `rule-resources`, `subscriptions`
- `dashboard` → `config-editor`（出站辅助函数）, `core`, `profiles`
- `inbounds` → `settings`（默认端口，可接受）
- `settings` → `auth`, `core`（可接受）

真正的枢纽是 `Draft`，不是路由。

`config-editor` 约 5372 行，是前端体量最大的一块（整个 `web/src` 约 1.2 万行）。`ConfigWorkspace.vue` 同时做：档案复制/重命名/删除、草稿保存编排、应用/预览/检查、各模块保存、DNS FakeIP 默认规则、入站解除关联、出站重命名连带改写其它模块。入站页的对话框被配置工作台直接 import（`InboundEditorDialog` / `InboundImportDialog`），等于两个页面共享同一套业务组件，而不是共享一份 DTO。

产品文案已经互相打架：`InboundResourcesView.vue` 仍写「导入后任一侧修改或删除均不联动」，工作台和 `references.ts` 却实现了关联并在应用前同步。这是「导入即切断」和「活绑定」两套模型并存的直接症状。

浏览器里还有一批生成期逻辑同样不该留在面板里：Fake-IP 默认 DNS 规则、出站/DNS 标签重命名跨模块改写、路由/DNS 禁用规则只存在 `editorState`（丢掉这份附件规则就会消失）。

`editorState` 名义上是 UI 附件（`internal/profile/model.go`），实际上是第二份配置：出站策略组引用、入站资源绑定、托管 sniff 规则、DNS 编辑状态。后端原样存取，前端负责让它和 `buffers` 保持一致。任一侧漏改，预览和应用就会漂。

## 5. 为什么一改就要动很多地方

### 5.1 刚合入的引用同步就是样本

`cb6b6d0`「同步配置引用并关联入站源资源」按产品语义只是一句话。实际改动 20 个文件、+748 / −60：

- `application/store.ts`、`application/types.ts`、新建 `application/references.ts`
- `config-editor/ConfigWorkspace.vue`、`InboundConfigPanel.vue`、`inbound.ts`、`outbound.ts`、`use-outbound-editor.ts`
- `inbounds/InboundEditorDialog.vue`、`InboundImportDialog.vue`
- `web/tests/application.test.ts`、`inbounds.test.ts`、`outbound-options.test.ts`、新建 `reference-sync.test.ts`
- `web/e2e/inbounds.spec.ts`、`outbounds.spec.ts`、新建 `reference-sync.spec.ts`
- `README.md`、`web/package.json`、版本锁文件

原因：同步算法必须理解出站 `editorState`、入站绑定、路由 sniff 意图，还要写回全局草稿。测试又同时锁前端算法和浏览器路径。

### 5.2 改动放大器

1. **三份模型**：`buffers`（编辑文本）、`config`（解析对象）、`editorState`（UI/引用）。浏览器不能安全往返大整数，所以保存只交 buffers，结构化编辑必须用 jsonc 改文本并同时对齐另外两份。改行为通常三处一起改，嗅探这类需求还要改第四处（route）。
2. **领域逻辑贴在面板旁边**：`outbound.ts` / `inbound.ts` / `dns.ts` / `route.ts` 被面板、对话框、application、dashboard、单测同时 import。
3. **跨 feature 复用的是实现，不是契约**：入站库和配置入站页共用对话框和 `inbounds/model.ts`。
4. **测试镜像实现**：前端算法单测 + 面板测试 + Playwright。Go 侧没有对应的引用展开测试，因为展开不在 Go。
5. **发布约定**：几乎每次改代码都要动版本和 README，再叠 2–3 个文件。

最近约 50 次提交平均 16.6 个文件（去掉视觉快照仍约 10.2）。分布是双峰：文档/版本 1–4 个文件，功能 8–25，全站样式 45–93。历史量级：入站资源库 49；订阅 44；配置应用与侧栏 45；仪表盘节点切换 25。纯 UI 可以很小（连接列表删列 1 文件）。一碰到“草稿含义”，半径就跳升。

同一概念还有多份拷贝，进一步放大半径：入站类型/endpoint/tag 长度限制在 `inbounds/model.ts` 与 `internal/inbounds/service.go` 各写一遍；capabilities 在 Go 结构体和 TS 接口各一份；配置模块表在 `module-map.ts` 与 `configgen/model.go`；深度比较函数至少四份。入站对话框里的 `EditorForm` 实际上就是 schema，加一个字段要改 interface、initialize、校验、serialize、模板、单测和 e2e。

## 6. 后端其实已经按你的设想分层

`internal/` 按领域分包：`profile`、`inbounds`、`subscriptions`、`rules`、`configgen`、`application`、`core`。HTTP 层是薄的 `handler.go`，路由在 `internal/httpapi/router.go`。

缺的是 **DraftCompiler**：在生成之前，根据 Web 模型（绑定、订阅引用、托管规则）得到最终 JSON。现在这块在 `web/src/features/application/references.ts` 和若干 `config-editor/*.ts`。

## 7. 目标所有权

| 职责 | 归属 |
| --- | --- |
| 表单、列表、对话框、导航 | 前端 |
| 草稿修订、ETag、冲突 | 后端 `profile` |
| 入站库 / 订阅 / 规则 CRUD | 后端对应包 |
| 展开引用、注入 sniff、展开订阅、剥离 GUI 字段 | 后端 DraftCompiler |
| 规则集路径注入、语义检查、check、应用 | 后端（已有） |
| 核心进程、下载、Clash 运行时 | 后端（已有） |
| 跨页耦合 | 只传 `profileId`、`resourceId`、`subscriptionId`、摘要 |

页面边界：

- 入站配置页：只读写 `inbound-resources`。
- 配置工作台入站页：只展示档案内入站投影；关联用资源 ID，不 import 入站库的 store/model 实现。
- 订阅页：只读写订阅。
- 出站页：编辑策略组和引用 ID；不要在浏览器里展开节点 JSON。
- 应用栏：`flush` 后调用 preview/validate/apply；不要在调用前改草稿。

## 8. 重构方案（可增量）

不重写前端，不把表单搬到服务端。按规则集已经验证的模式，把另外两类引用收编到生成期。

### 阶段 0：把约定写死

在 `AGENTS.md` 增加所有权矩阵和禁令：新的展开/迁移/同步逻辑不得进 Vue。跨页面禁止 import 对方 feature 的算法模块，只允许 `types` 和 API DTO。

### 阶段 1：把当前“同步引用”改道到后端（先止血）

这是 `cb6b6d0` 的正确产品行为、错误实现位置。

1. 新增 `internal/compile`（或扩 `configgen`），输入草稿 + 入站库 + 订阅，输出物化后的 config/buffers。
2. `preview` / `validate` / `apply` 在生成前调用它。失败返回现有错误结构（`stage=sync_references`）。
3. 前端 `application/store.ts` 只 `flush` 再调 API；删除浏览器侧 `synchronizeReferences`。
4. 把 `web/tests/reference-sync.test.ts` 的用例下沉为 Go 测试。
5. 保留一条 e2e：改订阅或入站源后，检测/应用看到新内容。

完成后，这类需求的改动半径变成：Go 编译器 + 其测试；前端最多改提示文案。自动重载也会走同一编译器，不再依赖浏览器是否先同步过。

### 阶段 2：引用改成规则集同款模型

入站：档案存 `{ tag, resourceId }`（可加本地 tag 覆盖），不再把源 JSON 复制进草稿后再靠前端回刷。后端注释里的“导入即切断关系”与“保持关联并同步”只能留一个；建议与规则集一致，选活绑定。

出站：`editorState.outbounds.groups[].references` 视为 Web 模型；最终 `outbounds` 数组只在编译期生成。`hidden` / `include` / `exclude` / `icon` 继续不进 sing-box。

嗅探：托管 sniff 规则由编译器写入 `route.rules`，不要在入站面板里改路由 JSON。

`editorState` 在 Go 里变成有类型的结构，未知键继续保留。不要上完整 sing-box JSON Schema。

### 阶段 3：拆配置工作台，让页面只依赖 DTO

- `ConfigWorkspace.vue` 降为壳：标签 + 插槽。档案操作回 `profiles`，应用栏留在 `application`。
- 入站编辑对话框改成只收 `InboundInput`、只吐 `InboundInput` 的展示组件。配置页和资源库页各自写适配，不再共享 store。
- 仪表盘节点选择只走 `/runtime/proxy-groups`，删除对 `config-editor/outbound` 的依赖。

### 阶段 4：测试跟着所有权走

- 领域：Go 单测（编译、绑定、冲突、缺失源）。
- 前端：表单渲染和操作，不测展开算法。
- e2e：每条主路径一条，不复制领域用例。
- 版本号继续按发布约定递增；纯内部重构不必改 README 行为说明。

## 9. 改完之后的半径

| 需求 | 现在 | 目标 |
| --- | --- | --- |
| 入站表单加一个字段 | 对话框 + model + 配置面板 + inbound.ts + 测试 + 可能 references | 对话框（若只是展示）；若要校验，再加后端入站 DTO |
| 订阅变更反映到 Selector | references + outbound.ts + store + 单测 + e2e | DraftCompiler |
| 嗅探迁移 | inbound.ts + route.ts + ConfigWorkspace + 测试 | DraftCompiler |
| 路由行展示 | 可以只改 `RouteConfigPanel.vue`（已相对局部） | 不变 |
| 同步引用失败提示 | 前端 store + 类型 + 工作台 | API 错误码 + 应用栏展示 |

## 10. 明确不做

- 不把 Vue 改成服务端渲染，配置表单仍适合 SPA。
- 不维护官方 sing-box JSON Schema。
- 不一次搬走 `dns.ts` / `route.ts` 里所有表单辅助函数；先搬“生成期处理”。
- 不另起 BFF；编译器放在现有 Go 单体即可。
- 不引入跨页面的超级 store。`Draft` 已经是超级 store。

## 11. 建议顺序

先做阶段 1，且用刚合入的引用同步验收。它已经证明：产品要后端处理，实现却不得不改 20 个前端文件。把这条路径改对，后面的入站关联和出站展开才有地方放。
