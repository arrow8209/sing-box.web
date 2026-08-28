# SingBox Control

当前仓库包含可构建的 Vue 管理界面、本地 Go 服务，以及保留用于视觉和交互参照的 `prototype/`。服务运行时固定在当前工作目录生成 `data/` 与 `config/`，不会接受数据目录或配置目录覆盖参数。

仪表盘已完整迁移交互原型中的快速控制、运行指标、流量图、路由模式和节点选择器。核心版本、运行状态、活动档案与配置计数读取服务端数据；尚未接入运行 API 的开关、流量、连接、路由模式、节点切换和测速只保留页面内演示状态，不会修改 sing-box 或宿主机配置。

## 本地服务开发

前置条件：Go 1.26、Node.js 22 与 npm。首次使用先安装前端依赖并构建：

```bash
npm --prefix web install
sh scripts/build.sh
./dist/singbox-web admin set-password
./dist/singbox-web serve
```

服务监听所有 IPv4 网卡的 `0.0.0.0:50016`。Web 服务可以管理 sing-box 核心程序、对各档案候选配置执行真实 `check`，并将通过检查的档案应用到唯一运行目录；核心更新只能经管理员确认后手动触发。当前阶段不提供 systemd、自启动、反向代理、TLS 或高级运行控制；请仅允许可信网络访问。

配置检查针对草稿生成的完整多文件目录，不是单个 `config.json`。每个自定义配置固定使用自身 ID 对应的目录，服务以工作目录为当前目录执行：

```bash
data/bin/sing-box check -C config.staging-<profile-id>
```

`config.staging-<profile-id>/` 在检查成功或失败后都会保留，后续检查同一配置时更新为最新完整候选，因此管理员可以随时在后台审核。目录及文件权限分别为 `0700` 和 `0600`，属于不会提交到 Git 的运行时数据；其中可能含认证信息等敏感配置，请勿复制到日志、工单或公开位置。

“应用”、复制、重命名和删除当前档案的操作集中在“配置工作台”；“配置档案”页用于新建和选择档案。应用会先执行与“检查”按钮相同的完整检查；失败时直接显示检查结果，不停止核心、不修改正式 `config/`。检查成功后，服务复制已验证配置、停止当前核心、原子替换唯一的 `config/`，再启动所选档案；切换或启动失败会尝试恢复此前目录和核心。若正式目录存在外部修改，页面会要求再次确认，避免静默覆盖。

配置档案可在服务端复制为独立副本；草稿配置、编辑器状态和规则绑定会一并复制，但不继承已应用修订、摘要或检查结果。副本从 revision 1 开始，重复复制时名称会自动编号。

`internal/integration` 的测试会把当前 Go 测试二进制复制为受控 fake sing-box 核心，并只使用本地 `httptest` 规则服务；不会访问公网、启动真实代理或写入仓库工作目录。

## 规则集来源

“规则集中心”使用 GUI-for-Cores Ruleset-Hub 的完整 sing-box 目录，不再使用旧的 SagerNet GeoSite/GeoIP 规则地址：

- 目录：`https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full.json`
- 目录校验：`https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full-version.txt`
- GeoSite：`https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/`
- GeoIP：`https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/`

服务端会校验目录 SHA-256 并缓存最近一次成功结果；刷新失败不会覆盖可用缓存。网络规则统一以 Binary（`.srs`）格式保存，规则集目录导入也固定使用 Binary。升级时，历史网络 Source 元数据会保留原资源 ID 并迁移为 Binary，旧的 Source 下载文件会被删除，内容按需重新下载。页面中直接维护的手工规则统一使用 Source JSON（`.json`）；手工内容只会写入服务端派生的固定文件，不接受任意本地路径。目录的 Source JSON 仅用于结构预览，不会保存为网络规则资源。

## 下载代理

“设置 → 代理设置”可以为服务端执行的 HTTP/HTTPS 下载配置统一代理。代理会用于核心版本查询、核心资产和校验文件、规则集目录、规则预览以及远程规则下载；保存后立即生效，无需重启服务。留空保存时明确使用直连，不读取进程中的 `HTTP_PROXY` 或 `HTTPS_PROXY` 环境变量。

代理地址必须是完整的 `http://` 或 `https://` URL，例如 `http://127.0.0.1:7890`，当前不接受 URL 内账号密码。该设置保存在权限为 `0600` 的 `data/settings.json`，只影响管理服务自身的下载，不修改浏览器请求或 sing-box 核心流量。

## JSON 订阅

“订阅配置”用于将一组完整的 sing-box `outbounds` 作为本地 JSON 节点集合导入、查看和整体编辑。编辑器接受包含 `outbounds`（或 `nodes`）数组的对象，也接受直接的节点数组；每个节点必须包含唯一的非空 `type` 和 `tag`。节点中的其他字段会由服务端完整保存到权限为 `0600` 的 `data/subscriptions.json`。

当前订阅功能不接受 URL，不执行网络拉取、解析、同步或定时刷新。订阅列表接口只返回数量和协议摘要，管理员打开查看、编辑或配置出站引用时才读取完整节点内容。

自定义配置的“出站”页按 GUI.for.SingBox 的策略组模型支持 Direct、Block、Selector、URLTest 四种结构化类型；其他协议节点仍可通过“编辑全部 JSON”维护。Selector 和 URLTest 可以引用当前档案中的其他策略出站、整个 JSON 订阅或单个订阅节点；整个订阅会按 `include` / `exclude` 展开，编辑时仍保留为一个订阅选择项。展开节点和 `direct` / `block` 仅写入最终 sing-box `outbounds`，不会重复显示成策略卡片；`hidden`、筛选条件和图标保存在独立编辑器状态中，不会作为无效字段传给 sing-box。重命名策略出站时会同步更新已知配置引用，重复节点 `tag` 会生成不冲突的名称。

## 入站配置

一级“入站配置”用于集中维护可复用的 sing-box 入站对象，支持 Mixed、HTTP、SOCKS、TUN、Direct 的结构化编辑，也可使用完整 JSON 保存其他类型和未知字段。资源保存在权限为 `0600` 的 `data/inbound-resources.json`；列表接口只返回 `tag`、类型和监听摘要，包含认证用户等完整内容仅在查看、编辑或导入时读取。

自定义配置的“入站”页可以一次选择多个资源导入。导入只把当时的完整对象复制进档案的 `inbounds`，不保存资源 ID 或来源关系；之后修改或删除任一侧都不会联动另一侧。若导入对象的 `tag` 已被当前档案使用，页面会自动生成不冲突的新名称。

结构化入站不会写入 sing-box 1.13 已移除的旧嗅探字段。导入包含历史 `sniff` 字段的结构化资源时，会按[官方迁移方式](https://sing-box.sagernet.org/migration/#migrate-legacy-inbound-fields-to-rule-actions)生成按 inbound tag 限定的 `route.rules` `sniff` 动作；新配置应在“路由”页维护协议嗅探。入站编辑器不提供旧版“协议嗅探”和“使用嗅探结果覆盖目标地址”参数，用户通过“完整 JSON”维护的入站对象也不会被自动迁移这些字段。

## 开发与验证

```bash
go test -race ./...
go vet ./...
npm --prefix web run typecheck
npm --prefix web run test:unit
sh scripts/build.sh
npm --prefix web run test:e2e
```

浏览器 E2E 使用隔离的临时目录、本地 fake core 和本地规则/release 服务，不会读取仓库的 `data/`、访问公网或启动真实代理。真实 sing-box 契约测试使用已记录的官方二进制选择：`sh scripts/contract-test.sh`。只有维护者在明确允许网络访问后才可执行 `sh scripts/contract-test.sh --refresh` 更新选择。

如本机正用 `prototype/` 占用默认的 4173 端口，可为 E2E 指定其他本机端口：

```bash
SINGBOX_WEB_E2E_PORT=4174 npm --prefix web run test:e2e
```

## 视觉参照原型

`prototype/` 不会被正式服务加载或构建，保留它是为了让 Vue 页面持续对齐既有的信息架构、视觉层次和响应式布局。可单独预览：

```bash
npm --prefix prototype run dev
```

## 项目结构

```text
.
├── docs/                    # 设计与实施文档
├── prototype/               # 保留的视觉与交互参照原型
├── web/                     # Vue SPA、单元和浏览器测试
├── e2e/harness/             # 隔离的浏览器测试服务
├── internal/                # Go 服务和嵌入的前端资源
├── scripts/build.sh         # 构建嵌入 SPA 与服务二进制
└── README.md
```

当前版本不包含 systemd、Caddy、HTTPS/公网部署、远程订阅拉取、配置导出、自动核心更新或高级运行控制。低频模块的片段编辑器负责 JSON 语法和顶层容器类型校验；部署前仍需使用目标核心执行 `sing-box check`。
