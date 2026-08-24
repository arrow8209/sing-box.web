# SingBox Control

当前仓库包含可构建的 Vue 管理界面、本地 Go 服务，以及保留用于视觉和交互参照的 `prototype/`。服务运行时固定在当前工作目录生成 `data/` 与 `config/`，不会接受数据目录或配置目录覆盖参数。

## 本地服务开发

前置条件：Go 1.26、Node.js 22 与 npm。首次使用先安装前端依赖并构建：

```bash
npm --prefix web install
sh scripts/build.sh
./dist/singbox-web admin set-password
./dist/singbox-web serve
```

服务监听所有 IPv4 网卡的 `0.0.0.0:50010`。Web 服务作为唯一父进程管理 sing-box 子进程；应用档案会自动执行停止、`check` 和重启。核心更新只能经管理员确认后手动触发。当前阶段不提供 systemd、自启动、反向代理、TLS 或高级运行控制；请仅允许可信网络访问。

`internal/integration` 的测试会把当前 Go 测试二进制复制为受控 fake sing-box 核心，并只使用本地 `httptest` 规则服务；不会访问公网、启动真实代理或写入仓库工作目录。

## 规则集来源

“规则集中心”使用 GUI-for-Cores Ruleset-Hub 的完整 sing-box 目录，不再使用旧的 SagerNet GeoSite/GeoIP 规则地址：

- 目录：`https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full.json`
- 目录校验：`https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full-version.txt`
- GeoSite：`https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/`
- GeoIP：`https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/`

服务端会校验目录 SHA-256 并缓存最近一次成功结果；刷新失败不会覆盖可用缓存。网络规则统一以 Binary（`.srs`）格式保存，规则集目录导入也固定使用 Binary。页面中直接维护的手工规则统一使用 Source JSON（`.json`）；手工内容只会写入服务端派生的固定文件，不接受任意本地路径。目录的 Source JSON 仅用于结构预览，不会保存为网络规则资源。

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

一期不包含 systemd、Caddy、HTTPS/公网部署、订阅、配置导出、自动核心更新或高级运行控制。低频模块的片段编辑器负责 JSON 语法和顶层容器类型校验；部署前仍需使用目标核心执行 `sing-box check`。
