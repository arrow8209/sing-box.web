# sing-box Web 管理界面（Phase 1）

本项目是单体 Go 可执行文件：Vue 前端嵌入二进制，后端通过 `/api/v1` 管理多个 sing-box 配置档案。Phase 1 只负责安全的配置编辑、校验和发布，不启动或控制 sing-box 核心。

## 环境与版本

开发/验收环境要求：Go 1.24+、Node.js 22.22.0、npm 10.9.4、Caddy 2.x，以及与内置 Schema 锁定的 sing-box 1.13.19。生产机还需要 `curl`；端到端测试额外需要 `jq`、`ss` 和 Chromium（Playwright）。确认版本：

```bash
go version
node --version                 # v22.22.0
npm --version                  # 10.9.4
caddy version
/usr/local/bin/sing-box version # 最后一项应为 1.13.19
```

## 安装、构建与测试

```bash
npm ci --prefix web
go test ./...
npm --prefix web run test
npm --prefix web run typecheck
./scripts/build.sh              # npm ci、前端构建、go test、生成 dist/singbox-web
```

发布验收还应运行 `go test -race ./...`、`go vet ./...`、`git diff --check`，并在已安装真实核心时运行：

```bash
SING_BOX_BIN=/usr/local/bin/sing-box go test -v ./tests/contract
npm --prefix web run test:e2e
```

端到端测试由 `scripts/e2e-server.sh` 启动并清理临时后端/Caddy；不要指向手工运行的生产服务。`tests/contract` 的模块往返测试可在没有核心时运行，真实二进制测试会明确跳过；正式验收必须提供 `SING_BOX_BIN`。

## 首次部署与目录权限

建议使用专用用户和组。数据根目录仅服务用户可读写（`0700`），正式配置根目录由服务用户和 `sing-box` 组共享（`0750`）：

```bash
sudo useradd --system --home /var/lib/singbox-web --shell /usr/sbin/nologin singbox-web
sudo groupadd --system sing-box 2>/dev/null || true
sudo install -d -o singbox-web -g singbox-web -m 0700 /var/lib/singbox-web
sudo install -d -o singbox-web -g sing-box -m 0750 /etc/sing-box/web-profiles
sudo install -d -o root -g sing-box -m 0750 /etc/sing-box
```

完成下节的二进制安装后，首次设置管理员密码必须在终端交互输入（至少 6 个字符，不要把密码写进 shell 历史或脚本）：

```bash
sudo -u singbox-web /usr/local/bin/singbox-web admin set-password \
  --data-dir /var/lib/singbox-web
```

`--data-dir` 保存 `auth.json`、草稿和应用锁；`--config-root` 保存每个档案的稳定目录与发布文件。两者必须是不同目录，且服务用户拥有相应写权限。

## 安装二进制、systemd 与 Caddy

构建完成后只安装可执行文件：

```bash
sudo install -o root -g root -m 0755 dist/singbox-web /usr/local/bin/singbox-web
sudo install -o root -g root -m 0644 deploy/systemd/singbox-web.service \
  /etc/systemd/system/singbox-web.service
sudo systemctl daemon-reload
sudo systemctl enable --now singbox-web
sudo systemctl status singbox-web
```

`deploy/systemd/singbox-web.service` 使用 `singbox-web` 用户、回环监听 `127.0.0.1:8080`、`NoNewPrivileges` 和 `ProtectSystem`；按实际路径核对 `SupplementaryGroups=sing-box`、`ReadWritePaths` 和 `ExecStart` 后再启用。

Caddy 只负责公网 TLS 和反向代理。复制 `deploy/caddy/Caddyfile.example`，将域名换成真实域名：

```bash
sudo install -d -m 0755 /etc/caddy
sudo install -m 0644 deploy/caddy/Caddyfile.example /etc/caddy/Caddyfile
sudoedit /etc/caddy/Caddyfile
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

示例限制请求体为 2MB，注入 HSTS、`X-Content-Type-Options` 和 `Referrer-Policy`，并仅代理到本机服务。部署后检查：

```bash
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS https://web.example.com/healthz
```

## 配置档案、发布与外部校验

每个档案的正式目录是 `--config-root/<slug>/`，内部包含稳定发布入口和顶层模块 JSON 文件；不要手工合并或写入 `config.json`。发布前执行 Schema、语义检查和真实核心检查，发布后可由运维在应用外复核：

```bash
sing-box check -C /etc/sing-box/web-profiles/primary
```

外部检查必须针对稳定档案目录（不是草稿目录），且目录中应只有模块文件。若外部工具或人工修改造成漂移，发布流程会先阻止静默覆盖并报告漂移；管理员必须核对现场后，明确确认使用当前有效的 Web 草稿重新校验并发布（或先恢复目录 owner/mode 和已知发布产物）。该路径绝不导入、合并或接管外部配置。

Phase 1 不支持导入服务器已有配置、不支持配置历史浏览、不支持回滚，也不兼容旧版 Schema。发布失败或进程中断后的原子恢复只保证稳定目录最终指向一个完整的旧版本或完整的新版本；不应假定入口一定仍是旧版本，客户端必须重新读取发布状态。临时目录和未完成入口会被清理，重启服务会执行恢复检查。不要删除原稳定目录来“修复”发布。

## Schema 更新记录

当前内置版本记录在 `internal/assets/schema.version.json`：

| 字段 | 当前值 |
|---|---|
| targetVersion | `1.13.19` |
| schemaCommit | `583a690e888cdf6e8b024bf017ebf6297e08338d` |
| SHA-256（schema JSON） | `ee03dcaffe9282100418de280c61d3f0c8022712af50bc79c072994247133c41` |

只使用运维脚本更新并保留版本、来源 commit、抓取时间和 SHA-256 记录：

```bash
SING_BOX_BIN=/usr/local/bin/sing-box ./scripts/update-singbox-schema.sh
sha256sum internal/assets/sing-box.schema.json
cat internal/assets/schema.version.json
```

提交 Schema 更新时必须同时更新上述元数据，并在变更记录中注明旧/新 `targetVersion`、`schemaCommit`、SHA-256、执行命令和真实 `sing-box check -C` 结果。

## 导出与敏感值

“下载完整配置”会导出可运行的合并 JSON，包含 Clash API Secret 等敏感值；仅可在受控终端下载、传输和删除，不要贴到工单、日志、聊天或 Git。应用在确认后才允许完整导出。推荐使用“复制脱敏配置”进行分享，脱敏值为 `***REDACTED***`，不得将其误认为可运行配置；脱敏导出不应包含原 Secret，且浏览器日志/审计日志不会记录 Secret。

## Phase 1 能力边界与 Phase 2 交接

一期“运行状态”“订阅”“规则集”页面保留导航但显示“尚未接入”，不提供核心启动/停止/重启、实时流量/延迟/连接/日志、订阅拉取或规则资源调度；`/api/v1/core` 不存在。配置中的原生远程规则 URL 仍可编辑，但由 sing-box 自身处理，不由 Web 下载。

二期沿设计文档独立拆分：A 只读发现、版本、状态、日志和指标；B 绑定已发布档案并控制生命周期；C 接入模式、策略组、测速、连接和实时日志。二期仍不得让 Web 主进程以 root 运行，systemd/受限辅助进程权限须另行设计。交接依据：[设计文档](docs/superpowers/specs/2026-08-18-singbox-web-phase1-design.md) 与[实施计划](docs/superpowers/plans/2026-08-18-singbox-web-phase1.md)；不恢复已删除的历史测试文档。

从新上下文重启二期 A 时，使用设计第 20 节的精确提示词：

```text
$superpowers:brainstorming 请先阅读
docs/superpowers/specs/2026-08-18-singbox-web-phase1-design.md，
基于其中“二期接口边界”和“演进路线与交接”，
为二期核心控制模块制定独立设计。
```

## 故障与恢复

- `profiles.json` 或 `auth.json` 损坏：服务拒绝启动并报告路径；从受控备份恢复后再启动，不自动重置密码或覆盖文件。
- 单个草稿 JSON 损坏：服务可启动，但档案进入只读错误状态；不要用默认模板覆盖，保留原文件并人工修复/恢复。
- 外部漂移：发布前后检查稳定目录；发现额外、缺失或被改写模块时先阻止静默覆盖并保留现场。管理员可明确确认以当前有效 Web 草稿重新校验发布，或恢复 owner/mode 和已知发布产物；绝不导入外部配置。
- 中断发布、超时或崩溃：原子恢复保证稳定入口最终是完整旧版本或完整新版本，客户端必须重新读取发布状态；临时目录由恢复流程清理，重启后检查未完成标记。确认健康检查与 `sing-box check -C` 均通过后再发布。

## 开发启动

```bash
SINGBOX_WEB_LISTEN=127.0.0.1:50010 \
SINGBOX_WEB_DATA_DIR=$PWD/.singbox-web/data \
SINGBOX_WEB_CONFIG_ROOT=$PWD/.singbox-web/config \
SINGBOX_WEB_PUBLIC_ORIGIN=http://127.0.0.1:50010 \
./scripts/start-dev.sh
```

开发服务首次运行会提示设置管理员密码。生产环境请始终使用 Caddy HTTPS、专用用户和上面的权限约束。
