# SingBox Control

当前仓库包含本地 Web 服务的基础阶段，以及用于视觉参照的交互原型。服务运行时固定在当前工作目录生成 `data/` 与 `config/`，不会接受数据目录或配置目录覆盖参数。

## 本地服务开发

首次启动前设置管理密码：

```bash
go run ./cmd/singbox-web admin set-password
go run ./cmd/singbox-web serve
go test ./...
```

服务监听所有 IPv4 网卡的 `0.0.0.0:50010`。当前阶段不包含 systemd、Caddy、HTTPS 或公网部署；请仅允许可信网络访问。

## 视觉原型

`prototype/` 是零依赖、可直接预览的 sing-box Web 控制台交互原型，包含：

- 仪表盘与核心运行状态
- 配置概览按官方 JSON Schema 展示全部 14 个顶层模块
- 常用模块使用结构化表单，DNS、端点、服务及高级模块支持完整 JSON 片段编辑
- 新增入站会按类型显示专属字段，“自定义”类型支持输入、校验和再次编辑完整 JSON 对象
- 任意协议出站均可通过完整 JSON 对象导入、校验和再次编辑
- TUN 入站支持堆栈、自动/严格路由、端点独立 NAT、MTU 与多组 IPv4/IPv6 路由地址
- 策略组支持筛选条件、测速参数，以及引用其他出站与订阅节点
- 路由设置在单页中集中展示通用参数、规则集与有序规则，并支持按规则类型和动作编辑专属参数
- 订阅配置与同步操作
- 规则集管理
- 系统与界面设置
- 紧凑桌面布局、平板抽屉导航、手机横向配置标签、底部保存栏与底部弹窗

## 原型本地预览

```bash
npm --prefix prototype run dev
```

然后访问 <http://localhost:4173>。也可以直接在浏览器中打开 `prototype/index.html`。

如需从局域网中的其他设备访问：

```bash
npm --prefix prototype run dev -- --bind 0.0.0.0
```

## 项目结构

```text
.
├── docs/                    # 设计与实施文档
├── prototype/               # 零依赖交互原型
│   ├── index.html           # 页面入口
│   ├── package.json         # 开发与检查命令
│   └── src/
│       ├── app.js           # 页面数据与交互
│       └── styles.css       # 全局样式与响应式布局
└── README.md
```

当前页面使用演示数据，按钮、导航、主题切换、弹窗、配置保存、订阅同步和规则集更新均提供原型交互，不会修改真实的 sing-box 配置。它只是视觉参照，不是 `singbox-web serve` 的运行依赖。

低频模块的片段编辑器负责 JSON 语法和顶层容器类型校验。不同 sing-box 版本、平台及编译标签支持的字段可能不同，部署前仍需使用目标核心执行 `sing-box check`。
