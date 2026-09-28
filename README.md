# SingBox Control

当前仓库包含可构建的 Vue 管理界面、本地 Go 服务，以及保留用于视觉和交互参照的 `prototype/`。服务运行时固定在当前工作目录生成 `data/` 与 `config/`，不会接受数据目录或配置目录覆盖参数。

仪表盘提供运行概览、路由模式和节点选择器。核心版本、运行状态、活动档案与配置计数读取服务端数据；启动、停止和重启会通过受认证的生命周期 API 管理唯一 sing-box 进程。“设置 → 核心设置”可以决定控制台启动时是否自动运行最近已应用配置；没有已应用配置时始终保持停止。活动连接页面通过服务端读取当前已应用配置中的本机 Clash API，支持实时列表、详情与连接关闭，控制器密钥不会发送到浏览器。核心运行时仪表盘节点选择读取核心实际策略组，Selector 切换经核心确认后生效；核心停止时显示选定档案的待运行候选并标明来源，不将候选称为运行状态。URLTest 由核心自动选择。核心未运行或控制 API 不可用时无法手动切换，切换不会改写配置草稿。仪表盘的上传、下载、内存、活动连接数和节点延迟均读取核心真实数据。“快速控制”及其中的系统代理、TUN、局域网访问开关已移除。

节点测速由 sing-box 所在机器经指定出站向 `https://www.gstatic.com/generate_204` 发起 HTTPS HEAD 请求，显示核心返回的延迟（毫秒），不测下载带宽。单节点测试超时为 5 秒，最多同时执行 5 个请求，不自动重试。支持单节点、策略组和全部测速；批量按运行中的策略组递归收集、去重实际节点，跳过 Block，Direct 可测。嵌套策略组显示当前实际出口的延迟。结果逐个更新，区分排队、测试中、超时和失败；控制 API 故障单独提示，不当作节点超时。

页面同时读取核心的延迟历史，可在数值提示中查看最近测试时间。“最近测速成功”是历史测试统计，不代表持续在线。核心停止或配置变更时取消排队并丢弃旧响应；离开页面会取消浏览器请求，核心已开始的测试在本次超时内结束。测速不改写草稿、不主动切换 Selector；核心会更新延迟历史，URLTest 后续自动选择可能使用这些结果。URLTest 自动检测仍使用各组已应用的测试地址、间隔和容差。

仪表盘每秒通过只读接口 `GET /api/v1/runtime/metrics` 采样。服务端从当前核心的 Clash API `/connections` 读取累计上传、累计下载、内存和活动连接列表，仅返回汇总指标、采样时间与核心运行标识。上传／下载速率按连续两次累计字节数及实际采样间隔计算，包含经 sing-box 转发的 direct 流量，不代表整张网卡的流量。内存使用核心的 Go 运行时统计口径，与系统 RSS 不同；活动连接数使用当前列表长度，不含页面保留的已关闭历史。首次采样、核心停止、接口失效或超时时未知指标显示 `--`；重启、计数器清零及页面恢复可见后重新建立速率基准。无流量和无活动连接时才显示实测的零值。页面关闭或隐藏时取消采样，不显示固定的 24 小时流量或异常检测结论。

仪表盘的“规则／全局／直连”读取核心当前模式，并通过服务端 `GET/PUT /api/v1/runtime/mode` 调用 Clash API 的 `GET/PATCH /configs` 切换；写入后会再次读取核心，确认成功才更新选中状态。页面每 5 秒同步一次，重新进入页面、刷新状态或重启核心后会重新读取。切换作用于新连接，已有连接保留原出口；不会修改档案的默认模式或触发核心重启。核心启用了 `experimental.cache_file` 时由核心保存模式，未启用时重启后使用配置的默认模式。

三种模式的具体路由和 DNS 行为由已加载配置中的 `clash_mode` 规则决定：默认模板已提供全局、直连规则，规则模式继续按普通规则匹配。自定义配置需保留相应规则；界面会禁用核心未提供的模式并显示原因。模式切换不会向自定义配置注入规则，也不会覆盖其出站选择。

## 本地服务开发

前置条件：Go 1.26、Node.js 22 与 npm。首次使用先安装前端依赖并构建：

```bash
npm --prefix web install
sh scripts/build.sh
./dist/singbox-web-v0.1.14 admin set-password
./dist/singbox-web-v0.1.14 serve
```

构建文件名固定包含版本号：`dist/singbox-web-v<版本号>`。默认读取 `web/package.json` 的 `version`（当前为 `0.1.14`）；也可以通过 `VERSION=0.1.14 sh scripts/build.sh` 指定本次构建版本，支持带 `v` 前缀。构建完成后会输出实际文件路径，启动时使用对应文件名。

每次正式发布前必须递增版本号，默认递增补丁版本。任何代码修改后重新发布（包括小修复）都必须增加版本号，不得覆盖已发布的同版本产物，例如 `0.1.0` 修改后再次发布应使用 `0.1.1`。同步更新 `web/package.json` 和 `web/package-lock.json`，产物文件名使用相同版本。发布时按以下顺序执行：

```bash
npm --prefix web version patch --no-git-tag-version
sh scripts/build.sh
```

发布流程只更新版本并生成、验证 `dist/` 中的产物。`Release/` 是由用户自行维护的最终运行目录，发布时不操作该目录或管理其中的运行服务。用户自行部署并以 `Release/` 为工作目录运行，使正式运行数据保存在 `Release/data/` 和 `Release/config/`。`Release/` 属于本机运行目录，不提交到 Git。正式发布若指定 `VERSION`，也必须与版本文件保持一致。

执行 `./dist/singbox-web-v0.1.14 -h` 或 `--help` 可查看命令帮助；显示帮助不需要设置密码，也不会启动服务或创建运行数据。

Web 服务启动时读取工作目录下的 `data/settings.json`，通过 `webPort` 配置访问端口，监听地址保持 `0.0.0.0`。缺省使用发布端口 `50022`。`defaultMixedPort` 控制新建档案和新建 Mixed 入站的默认端口，缺省为 `50023`；`defaultApiPort` 控制新建档案的 Clash API 默认监听地址 `127.0.0.1:<端口>`，缺省为 `50024`。服务设置接口返回端口及完整默认 API 监听地址，网页直接读取。已有档案、入站资源及已应用配置保留各自的监听地址和端口。

| 环境 | `webPort` | `defaultMixedPort` | `defaultApiPort` |
| --- | --- | --- | --- |
| 发布环境 | `50022` | `50023` | `50024` |
| 测试/开发环境 | `50015` | `50016` | `50017` |

执行 `serve` 后，终端会在 Web 端口成功监听后显示实际监听地址、本机访问 URL、进程 PID、工作目录、配置目录、默认 Mixed 入站端口，以及核心状态、已安装版本和运行 PID。默认 Mixed 端口用于新建入站，已有配置实际监听的端口以核心日志为准；端口占用导致启动失败时不会显示启动成功。

核心写入标准输出和标准错误的日志会以 `[核心]` 前缀实时显示在当前终端，同时继续保存在 `data/logs/sing-box.log`，并保留原有的文件大小和单行长度限制。Web 启停信息使用 `[Web]` 前缀。退出服务时会停止受管核心、写出最后一条未换行日志并关闭日志文件。

发布与开发配置示例分别见 [`examples/settings.production.json`](examples/settings.production.json) 和 [`examples/settings.development.json`](examples/settings.development.json)。已有 `data/settings.json` 时，在原文件中添加或修改这三个端口字段，保留下载代理和核心行为设置；修改前停止 Web 服务，重新启动后生效。端口必须是 `1` 到 `65535` 的整数，三个端口不能相同；配置错误会阻止启动并提示字段及文件路径。旧设置文件未包含端口字段时继续兼容，读取后使用发布默认值。仅添加 `defaultApiPort` 不会改写已有档案的 `experimental.clash_api.external_controller`；已有档案可以在“常规设置”中单独修改监听地址。

完整开发服务使用开发配置示例中的端口，同样通过 `scripts/build.sh` 构建后执行 `./dist/singbox-web-v0.1.14 serve`（版本号以构建输出为准），从 `http://localhost:50015` 访问。开发和发布使用各自的工作目录保存 `data/`、`config/`。单独运行 `npm --prefix web run dev` 时，Vite 的前端开发端口也为 `50015`；它仅提供前端开发服务，请勿与完整 Web 服务同时占用该端口。端口被占用时 Vite 会报错，不会自动改用其他端口。

Web 服务可以管理 sing-box 核心程序、对各档案候选配置执行真实 `check`，并将通过检查的档案应用到唯一运行目录；核心更新只能经管理员确认后手动触发。当前阶段不提供 systemd、自启动、反向代理、TLS 或高级运行控制；请仅允许可信网络访问。

配置检查针对草稿生成的完整多文件目录，不是单个 `config.json`。每个自定义配置固定使用自身 ID 对应的目录，服务以工作目录为当前目录执行：

```bash
data/bin/sing-box check -C config.staging-<profile-id>
```

`config.staging-<profile-id>/` 在检查成功或失败后都会保留，后续检查同一配置时更新为最新完整候选，因此管理员可以随时在后台审核。目录及文件权限分别为 `0700` 和 `0600`，属于不会提交到 Git 的运行时数据；其中可能含认证信息等敏感配置，请勿复制到日志、工单或公开位置。

“应用”、复制、重命名和删除当前档案的操作集中在“配置工作台”；“配置档案”页用于新建和选择档案。应用会先执行与“检查”按钮相同的完整检查；失败时直接显示检查结果且不修改正式 `config/`。检查成功后，服务只将已验证配置原子复制到唯一的 `config/`，不会停止、启动或重新加载 sing-box，也不会改写核心运行状态。若复制期间正式目录同时发生变化，本次复制会被拒绝并保留现场内容。

配置工作台的“常规设置”会真实编辑当前档案的 `experimental.clash_api.default_mode`、`external_controller` 和 `secret`，并通过草稿自动保存接口持久化；API 密钥在表单中以密码形式显示。默认情况下，草稿保存后仍需点击“应用”才会同步到正式 `config/`。启用“配置变更自动重载”后，当前已应用档案每次保存都会自动检查并应用；核心正在运行时会随后重启加载新配置，核心已停止时只更新正式配置而不会擅自启动。其他档案仍保持草稿状态。侧栏分别显示“当前应用/已启动”和“有修改未同步”两个独立图标，其中当前应用图标在全部档案中最多出现一次。

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

代理地址必须是完整的 `http://` 或 `https://` URL，例如 `http://127.0.0.1:7890`，当前不接受 URL 内账号密码。代理和核心行为设置都保存在权限为 `0600` 的 `data/settings.json`；下载代理只影响管理服务自身的下载，不修改浏览器请求或 sing-box 核心流量。

核心版本检查和开始下载都会请求 GitHub API。若出口 IP 的 API 额度耗尽，两项操作都会失败；页面会提示 GitHub 限流，并在响应提供额度重置时间时显示可重试时间。此时应等待额度恢复后重试，限流本身不能用于判断代理是否生效。

## 代理出口测试

启动并应用 sing-box 配置后，可以通过测试脚本设置客户端 HTTP/HTTPS 代理并查询出口 IP。脚本默认连接发布环境的 Mixed 入站 `http://127.0.0.1:50023`；开发环境使用 `scripts/test-proxy-ip.sh http://127.0.0.1:50016`。已有入站使用其他端口时，传入该入站的实际地址：

```bash
scripts/test-proxy-ip.sh
scripts/test-proxy-ip.sh http://127.0.0.1:7890
PROXY_URL=http://127.0.0.1:7890 scripts/test-proxy-ip.sh
```

第二个参数或 `IP_LOOKUP_URL` 可以覆盖默认的 `https://api.ipify.org`。脚本会同时设置大小写形式的 `HTTP_PROXY`、`HTTPS_PROXY`、`http_proxy` 和 `https_proxy`，并为 `curl` 设置连接及总超时；获得出口 IP 后，还会通过 `https://ipwho.is` 输出中文国家、省州和城市信息。

## JSON 订阅

“订阅配置”用于将一组完整的 sing-box `outbounds` 作为本地 JSON 节点集合导入、查看和整体编辑。编辑器接受包含 `outbounds`（或 `nodes`）数组的对象，也接受直接的节点数组；每个节点必须包含唯一的非空 `type` 和 `tag`。节点中的其他字段会由服务端完整保存到权限为 `0600` 的 `data/subscriptions.json`。

当前订阅功能接收本地 JSON，不接受远程订阅 URL，也不会定时从网络获取订阅节点。订阅列表接口只返回数量和协议摘要；管理员查看或编辑时可读取完整节点。预览、检测、应用和自动重载由服务端直接读取所需来源快照。

自定义配置的“出站”页按 GUI.for.SingBox 的策略组模型支持 Direct、Block、Selector、URLTest 四种结构化类型；其他协议节点仍可通过“编辑全部 JSON”维护。Selector 和 URLTest 可以引用当前档案中的其他策略出站、整个 JSON 订阅或单个订阅节点；整个订阅会按 `include` / `exclude` 展开，编辑时仍保留为一个订阅选择项。服务端生成的节点仅出现在候选和最终 sing-box `outbounds` 中，不会重复显示成策略卡片。来源 ID、过滤、顺序和组引用保存在有版本的业务引用字段；`hidden` 和图标留在界面状态中。重命名策略出站时会同步更新已知配置引用，重复节点 `tag` 会生成不冲突的名称。

预览、检测、应用和自动重载使用同一服务端引用解析流程，在本次操作中读取已保存的档案与最新来源快照。单纯点击预览或检测不会保存草稿、递增 revision、更新正式配置或重启运行核心；手动应用在本次请求中生成、检查并提交同一候选。订阅字段的修改、删除及节点增删会反映在下一次候选中。来源缺失、指定节点消失、过滤表达式不兼容或引用结构矛盾时会显示可定位的“解析引用”错误，不沿用旧展开副本。用户正常保存档案仍遵循“配置变更自动重载”设置；更新入站或订阅来源本身只标记相关档案过期，不会直接重载核心。

## 入站配置

一级“入站配置”用于集中维护可复用的 sing-box 入站对象，支持 Mixed、HTTP、SOCKS、TUN、Direct 的结构化编辑，也可使用完整 JSON 保存其他类型和未知字段。资源保存在权限为 `0600` 的 `data/inbound-resources.json`；列表接口只返回 `tag`、类型和监听摘要，包含认证用户等完整内容仅在查看、编辑或导入时读取。

自定义配置的“入站”页可以一次选择多个资源导入，并默认保留来源关联。服务端生成候选时读取源资源的最新内容，同时保留档案内的 `tag`；同名导入会自动生成本地别名。来源 ID 保存在档案的业务引用字段中，不进入 sing-box 配置文件。

旧档案中没有来源记录的入站继续作为独立配置维护，可点击入站行的链接按钮选择源资源。关联入站的生成 JSON 只读；用户选择解除关联或复制时，会按当前所见的服务端候选固定原始 JSON，保留未知字段和大整数。独立入站若含超出 JavaScript 安全范围的整数，结构化改写会提示改用完整 JSON 编辑器，以免损失原始值。源删除后，只有确有迁移前已保存副本时才能明确恢复旧内容；没有副本则需重新关联或补充独立配置。删除档案内入站不删除源资源。新版引用格式及隔离回退方法见[配置引用草稿 v2 契约](docs/reference-schema-v2.md)。

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

浏览器 E2E 在每个用例开始时启动独立临时服务，使用各自的档案、设置、本地 fake core 和本地规则/release 服务；用例失败后也会关闭服务并清理运行目录，避免影响后续用例。测试不会读取仓库的 `data/`、访问公网或启动真实代理。默认占用开发/测试 Web 端口 `50015`，运行前请确保该端口空闲。

真实 sing-box 契约测试使用已记录的官方二进制选择：`sh scripts/contract-test.sh`。脚本下载并校验官方核心；测试再用该核心编译本地合成规则，通过回环 HTTP 服务下载规则，并对生成配置执行真实 `check`，不依赖公网规则源。只有维护者在明确允许网络访问后才可执行 `sh scripts/contract-test.sh --refresh` 更新选择。

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
