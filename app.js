(() => {
  "use strict";

  const app = document.querySelector("#app");
  const modalRoot = document.querySelector("#modal-root");
  const toastStack = document.querySelector("#toast-stack");

  const svgPaths = {
    dashboard:
      '<rect x="3" y="3" width="7" height="7" rx="2"/><rect x="14" y="3" width="7" height="7" rx="2"/><rect x="3" y="14" width="7" height="7" rx="2"/><rect x="14" y="14" width="7" height="7" rx="2"/>',
    config:
      '<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>',
    subscription:
      '<path d="M5 12.55a7 7 0 0 1 6.45 6.45M5 5a14 14 0 0 1 14 14M6 19h.01"/>',
    rules:
      '<path d="M4 6h16M4 12h16M4 18h16"/><circle cx="8" cy="6" r="1.5" fill="currentColor" stroke="none"/><circle cx="14" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="10" cy="18" r="1.5" fill="currentColor" stroke="none"/>',
    settings:
      '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06-2.83 2.83-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21h-4v-.09a1.65 1.65 0 0 0-1.08-1.5 1.65 1.65 0 0 0-1.82.33l-.06.06-2.83-2.83.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3v-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06 2.83-2.83.06.06A1.65 1.65 0 0 0 9 4.6a1.65 1.65 0 0 0 1-1.51V3h4v.09A1.65 1.65 0 0 0 15 4.6a1.65 1.65 0 0 0 1.82-.33l.06-.06 2.83 2.83-.06.06A1.65 1.65 0 0 0 19.4 9c.12.6.65 1.02 1.26 1.02H21v4h-.09A1.65 1.65 0 0 0 19.4 15Z"/>',
    chevron: '<path d="m9 18 6-6-6-6"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/>',
    moon: '<path d="M20.5 13.4A8 8 0 1 1 10.6 3.5a6.5 6.5 0 0 0 9.9 9.9Z"/>',
    sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.42-1.41M17.66 6.34l1.41-1.41"/>',
    bell: '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/>',
    menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
    cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M18 9h4M2 15h4M18 15h4M10 10h4v4h-4z"/>',
    globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a15 15 0 0 1 0 18M12 3a15 15 0 0 0 0 18"/>',
    shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/><path d="m9 12 2 2 4-4"/>',
    route: '<circle cx="6" cy="19" r="2"/><circle cx="18" cy="5" r="2"/><path d="M6 17V9a4 4 0 0 1 4-4h6M14 9l4 4 4-4"/>',
    dns: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.4 2.5 3.7 5.5 3.7 9S14.4 18.5 12 21M12 3C9.6 5.5 8.3 8.5 8.3 12S9.6 18.5 12 21"/>',
    inbound: '<path d="M5 4h14v16H5zM9 12h9M15 8l4 4-4 4"/>',
    outbound: '<path d="M5 4h14v16H5zM6 12h9M11 8l4 4-4 4"/>',
    general: '<path d="M4 5h16v14H4zM4 9h16M8 13h4M8 16h7"/>',
    rotate: '<path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 9A7 7 0 0 1 18.8 6.5L20 12M4 12l1.2 5.5A7 7 0 0 0 17.9 15"/>',
    power: '<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.8 0"/>',
    check: '<path d="m5 12 4 4L19 6"/>',
    upload: '<path d="m12 3-4 4M12 3l4 4M12 3v12M5 14v6h14v-6"/>',
    download: '<path d="m12 15-4-4M12 15l4-4M12 15V3M5 14v6h14v-6"/>',
    link: '<path d="M10 13a5 5 0 0 0 7.07.07l2-2A5 5 0 0 0 12 4l-1.15 1.15M14 11a5 5 0 0 0-7.07-.07l-2 2A5 5 0 0 0 12 20l1.15-1.15"/>',
    memory: '<rect x="4" y="6" width="16" height="12" rx="2"/><path d="M8 10h2v4H8zM14 10h2v4h-2zM7 2v4M12 2v4M17 2v4M7 18v4M12 18v4M17 18v4"/>',
    activity: '<path d="M3 12h4l2-7 4 14 2-7h6"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
    gauge: '<path d="M4.9 19a9 9 0 1 1 14.2 0M12 12l4-4"/><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none"/>',
    edit: '<path d="M12 20h9M16.5 3.5a2.12 2.12 0 0 1 3 3L8 18l-4 1 1-4Z"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    trash: '<path d="M3 6h18M8 6V4h8v2M19 6l-1 15H6L5 6M10 11v5M14 11v5"/>',
    grip: '<circle cx="9" cy="5" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="5" r="1" fill="currentColor" stroke="none"/><circle cx="9" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="9" cy="19" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="19" r="1" fill="currentColor" stroke="none"/>',
    more: '<circle cx="5" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1" fill="currentColor" stroke="none"/>',
    copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
    info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
    close: '<path d="m6 6 12 12M18 6 6 18"/>',
    save: '<path d="M5 3h12l3 3v15H4V3h1zM8 3v6h8V3M8 21v-7h8v7"/>',
    code: '<path d="m8 9-4 3 4 3M16 9l4 3-4 3M14 5l-4 14"/>',
    server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01M11 7h6M11 17h6"/>',
    sparkles: '<path d="m12 3 1.2 3.8L17 8l-3.8 1.2L12 13l-1.2-3.8L7 8l3.8-1.2ZM18.5 14l.8 2.2 2.2.8-2.2.8-.8 2.2-.8-2.2-2.2-.8 2.2-.8ZM5 13l.8 2.2L8 16l-2.2.8L5 19l-.8-2.2L2 16l2.2-.8Z"/>',
    lightning: '<path d="m13 2-9 12h8l-1 8 9-12h-8z"/>',
    user: '<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>',
    external: '<path d="M15 3h6v6M10 14 21 3M18 13v7H4V6h7"/>',
    lock: '<rect x="4" y="10" width="16" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
    palette: '<path d="M12 3a9 9 0 0 0 0 18h1.5a2 2 0 0 0 0-4H12a2 2 0 0 1 0-4h5a4 4 0 0 0 4-4c0-3.3-4-6-9-6Z"/><circle cx="7.5" cy="9" r="1" fill="currentColor" stroke="none"/><circle cx="10" cy="6.5" r="1" fill="currentColor" stroke="none"/><circle cx="14" cy="6.5" r="1" fill="currentColor" stroke="none"/>',
    terminal: '<path d="m5 7 4 4-4 4M11 17h8"/><rect x="2" y="3" width="20" height="18" rx="2"/>',
    arrowUp: '<path d="m6 15 6-6 6 6"/>',
    arrowDown: '<path d="m6 9 6 6 6-6"/>',
    folder: '<path d="M3 6h7l2 2h9v11H3z"/>',
    database: '<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v7c0 1.7 3.6 3 8 3s8-1.3 8-3V5M4 12v7c0 1.7 3.6 3 8 3s8-1.3 8-3v-7"/>',
  };

  function icon(name, className = "") {
    const path = svgPaths[name] || svgPaths.info;
    return `<span class="icon ${className}" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${path}</svg></span>`;
  }

  function escapeHtml(value) {
    return String(value).replace(/[&<>"']/g, (character) => ({
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#039;",
    })[character]);
  }

  function readStorage(key, fallback) {
    try {
      return window.localStorage.getItem(key) || fallback;
    } catch {
      return fallback;
    }
  }

  function writeStorage(key, value) {
    try {
      window.localStorage.setItem(key, value);
    } catch {
      // 文件协议或隐私模式下可能无法访问 localStorage，原型仍可正常工作。
    }
  }

  const configTabs = [
    { id: "general", label: "通用配置", icon: "general" },
    { id: "inbounds", label: "入站配置", icon: "inbound" },
    { id: "outbounds", label: "出站配置", icon: "outbound" },
    { id: "routing", label: "路由设置", icon: "route" },
    { id: "dns", label: "DNS 设置", icon: "dns" },
  ];

  const customInboundExample = `{
  "type": "mixed",
  "tag": "custom-in",
  "listen": "127.0.0.1",
  "listen_port": 7891,
  "sniff": true,
  "sniff_override_destination": false
}`;

  const initialRoute = window.location.hash.replace(/^#\/?/, "").split("/").filter(Boolean);
  const initialPage = ["dashboard", "config", "subscriptions", "rulesets", "settings"].includes(initialRoute[0])
    ? initialRoute[0]
    : "dashboard";

  const state = {
    page: initialPage,
    activeConfig: configTabs.some((tab) => tab.id === initialRoute[1]) ? initialRoute[1] : "general",
    configExpanded: true,
    sidebarOpen: false,
    coreRunning: true,
    systemProxy: true,
    tunMode: true,
    autoStart: true,
    allowLan: false,
    ipv6: true,
    fakeIp: true,
    clashApi: true,
    routeMode: "rule",
    theme: readStorage("singbox-theme", "light"),
    accent: readStorage("singbox-accent", "blue"),
    dirty: false,
    lastSaved: "10:24",
    customInbounds: [],
    formInbounds: [],
    editingCustomInboundId: null,
    editingOutboundId: null,
    outbounds: [
      {
        id: "outbound-select",
        tag: "节点选择",
        type: "selector",
        hidden: false,
        include: "",
        exclude: "",
        icon: "",
        interrupt_exist_connections: true,
        refs: [
          { groupId: "built-in", id: "outbound-urltest", tag: "自动选择", type: "Built-In" },
          { groupId: "subscriptions", id: "sub-1", tag: "Cloud Route Premium", type: "Subscription" },
          { groupId: "subscriptions", id: "sub-2", tag: "Edge Network", type: "Subscription" },
        ],
      },
      {
        id: "outbound-urltest",
        tag: "自动选择",
        type: "urltest",
        hidden: false,
        include: "",
        exclude: "",
        icon: "",
        interrupt_exist_connections: true,
        url: "https://www.gstatic.com/generate_204",
        interval: "5m",
        tolerance: 50,
        refs: [
          { groupId: "subscriptions", id: "sub-1", tag: "Cloud Route Premium", type: "Subscription" },
          { groupId: "subscriptions", id: "sub-2", tag: "Edge Network", type: "Subscription" },
        ],
      },
      {
        id: "outbound-failover",
        tag: "故障转移",
        type: "selector",
        hidden: false,
        include: "",
        exclude: "",
        icon: "",
        interrupt_exist_connections: true,
        refs: [
          { groupId: "built-in", id: "outbound-urltest", tag: "自动选择", type: "Built-In" },
          { groupId: "built-in", id: "direct", tag: "direct", type: "Built-In" },
        ],
      },
      {
        id: "outbound-direct",
        tag: "直连",
        type: "direct",
        refs: [],
      },
    ],
    rulesetSearch: "",
    rulesetType: "all",
    settingsSection: "core-settings",
    subscriptions: [
      {
        id: "sub-1",
        name: "Cloud Route Premium",
        logo: "CR",
        tone: "blue",
        nodes: 42,
        used: 68.4,
        total: 200,
        expires: "2026-09-18",
        interval: "每 6 小时",
        updated: "8 分钟前",
      },
      {
        id: "sub-2",
        name: "Edge Network",
        logo: "EN",
        tone: "cyan",
        nodes: 18,
        used: 24.7,
        total: 100,
        expires: "2026-10-02",
        interval: "每天",
        updated: "2 小时前",
      },
    ],
    rulesets: [
      { id: 1, name: "geosite-cn", source: "SagerNet/sing-geosite", type: "remote", format: "binary", count: "124,632", size: "3.8 MB", updated: "今天 09:42" },
      { id: 2, name: "geoip-cn", source: "SagerNet/sing-geoip", type: "remote", format: "binary", count: "8,926", size: "1.2 MB", updated: "今天 09:42" },
      { id: 3, name: "geosite-geolocation-!cn", source: "SagerNet/sing-geosite", type: "remote", format: "binary", count: "86,319", size: "4.6 MB", updated: "昨天 22:16" },
      { id: 4, name: "private", source: "本地规则", type: "local", format: "source", count: "32", size: "4.1 KB", updated: "8 月 16 日" },
      { id: 5, name: "ads-all", source: "privacy-protection", type: "remote", format: "binary", count: "216,804", size: "6.7 MB", updated: "8 月 17 日" },
    ],
  };

  const pageMeta = {
    dashboard: { title: "仪表盘", parent: "控制台" },
    config: { title: "自定义配置 A", parent: "配置" },
    subscriptions: { title: "订阅配置", parent: "资源管理" },
    rulesets: { title: "规则集", parent: "资源管理" },
    settings: { title: "设置", parent: "系统" },
  };

  function inboundCount() {
    return 3 + state.customInbounds.length + state.formInbounds.length;
  }

  function applyTheme() {
    document.documentElement.dataset.theme = state.theme;
    document.documentElement.dataset.accent = state.accent;
    document.querySelector('meta[name="theme-color"]')?.setAttribute(
      "content",
      state.theme === "dark" ? "#0e1320" : "#f4f7fb",
    );
  }

  function navItem(page, label, iconName) {
    return `<button class="nav-item ${state.page === page ? "active" : ""}" data-page="${page}">
      ${icon(iconName)}<span>${label}</span>
    </button>`;
  }

  function renderSidebar() {
    const configActive = state.page === "config";
    return `<aside class="sidebar" aria-label="主导航">
      <div class="brand">
        <div class="brand-mark"><span>S</span></div>
        <div class="brand-copy">
          <div class="brand-title">SingBox Control</div>
          <div class="brand-subtitle">Web panel</div>
        </div>
      </div>
      <div class="sidebar-scroll">
        <div class="nav-label">概览</div>
        ${navItem("dashboard", "仪表盘", "dashboard")}
        <button class="nav-parent ${configActive ? "active" : ""} ${state.configExpanded ? "expanded" : ""}" data-action="config-parent">
          ${icon("config")}<span>配置</span>${icon("chevron", "nav-chevron")}
        </button>
        ${
          state.configExpanded
            ? `<div class="nav-config-tree">
                <button class="config-name" data-config="general">
                  <span class="config-status-dot"></span><span>自定义配置 A</span>
                </button>
                ${configTabs
                  .map(
                    (tab) => `<button class="nav-subitem ${configActive && state.activeConfig === tab.id ? "active" : ""}" data-config="${tab.id}">${tab.label}</button>`,
                  )
                  .join("")}
              </div>`
            : ""
        }
        <div class="nav-label">资源</div>
        ${navItem("subscriptions", "订阅配置", "subscription")}
        ${navItem("rulesets", "规则集", "rules")}
        <div class="nav-label">系统</div>
        ${navItem("settings", "设置", "settings")}
      </div>
      <div class="sidebar-footer">
        <div class="core-mini">
          <div class="core-mini-icon">${icon("cpu")}</div>
          <div class="core-mini-copy">
            <div class="core-mini-title">sing-box 1.11.11</div>
            <div class="core-mini-state">${state.coreRunning ? "● 核心运行中" : "○ 核心已停止"}</div>
          </div>
          <button class="entry-action" data-action="restart-core" title="重启核心">${icon("rotate")}</button>
        </div>
      </div>
    </aside>`;
  }

  function renderTopbar() {
    const meta = pageMeta[state.page];
    const current = state.page === "config" ? configTabs.find((tab) => tab.id === state.activeConfig)?.label : meta.title;
    return `<header class="topbar">
      <button class="mobile-menu" data-action="toggle-sidebar" aria-label="打开菜单">${icon("menu")}</button>
      <div class="page-heading">
        <h1>${meta.title}</h1>
        <div class="breadcrumbs"><span>${meta.parent}</span><span>/</span><span class="current">${current}</span></div>
      </div>
      <div class="topbar-actions">
        <button class="top-search" data-modal="command">
          ${icon("search")}<span>搜索或跳转...</span><span class="keycap">⌘ K</span>
        </button>
        <button class="icon-button theme-button" data-action="toggle-theme" aria-label="切换主题" title="切换主题">
          ${icon(state.theme === "dark" ? "sun" : "moon")}
        </button>
        <button class="icon-button" data-action="notifications" aria-label="通知" title="通知">
          ${icon("bell")}<span class="notification-dot"></span>
        </button>
        <div class="avatar" title="管理员">AD</div>
      </div>
    </header>`;
  }

  function render() {
    applyTheme();
    document.body.classList.toggle("sidebar-open", state.sidebarOpen);
    app.innerHTML = `<div class="app-shell">
      ${renderSidebar()}
      <button class="sidebar-overlay" data-action="close-sidebar" aria-label="关闭菜单"></button>
      <main class="main-shell">
        ${renderTopbar()}
        <div class="page-content">${renderPage()}</div>
      </main>
    </div>`;
  }

  function renderPage() {
    if (state.page === "config") return renderConfigPage();
    if (state.page === "subscriptions") return renderSubscriptions();
    if (state.page === "rulesets") return renderRulesets();
    if (state.page === "settings") return renderSettings();
    return renderDashboard();
  }

  function metric(iconName, tone, label, value, unit, trend, trendType = "") {
    return `<article class="card metric-card">
      <div class="metric-icon ${tone}">${icon(iconName)}</div>
      <div class="metric-copy">
        <div class="metric-label">${label}</div>
        <div class="metric-value">${value}<small>${unit}</small></div>
        <div class="metric-trend ${trendType}">${trendType === "neutral" ? icon("clock") : icon("arrowUp")}<span>${trend}</span></div>
      </div>
    </article>`;
  }

  function renderDashboard() {
    return `<section>
      <div class="page-intro">
        <div class="page-intro-copy">
          <h2>早上好，管理员</h2>
          <p>所有服务运行正常。过去 24 小时共处理 18.6 GB 流量，未发现异常连接。</p>
        </div>
        <div class="page-actions">
          <button class="btn" data-action="refresh-dashboard">${icon("rotate")}刷新状态</button>
          <button class="btn btn-primary" data-config="general">${icon("config")}管理配置</button>
        </div>
      </div>

      <div class="dashboard-grid">
        <article class="card hero-status">
          <div class="hero-content">
            <div class="hero-eyebrow"><span class="pulse-dot"></span>系统实时状态</div>
            <div class="hero-main">
              <h2>${state.coreRunning ? "sing-box 正在运行" : "sing-box 已停止"}</h2>
              <p>${state.coreRunning ? "核心服务稳定，控制 API 与入站监听均可用。" : "当前不会转发任何流量，点击启动恢复服务。"}</p>
            </div>
            <div class="hero-footer">
              <div class="hero-meta">
                <div class="hero-meta-item"><span>运行时间</span><strong>${state.coreRunning ? "12 天 08:42:16" : "--"}</strong></div>
                <div class="hero-meta-item"><span>核心版本</span><strong>1.11.11</strong></div>
                <div class="hero-meta-item"><span>活动配置</span><strong>自定义配置 A</strong></div>
              </div>
              <div class="hero-actions">
                <button class="hero-btn" data-action="restart-core">${icon("rotate")}重启</button>
                <button class="hero-btn ${state.coreRunning ? "hero-btn-danger" : ""}" data-action="toggle-core">
                  ${icon("power")}${state.coreRunning ? "停止核心" : "启动核心"}
                </button>
              </div>
            </div>
          </div>
        </article>

        <article class="card control-card">
          <div class="card-head">
            <div><h3 class="card-title">快速控制</h3><p class="card-subtitle">常用网络功能开关</p></div>
            <span class="badge badge-success">已连接</span>
          </div>
          <div class="control-list">
            ${controlRow("globe", "", "系统代理", "127.0.0.1:7890", "systemProxy")}
            ${controlRow("shield", "cyan", "TUN 模式", "自动路由 · Strict Route", "tunMode")}
            ${controlRow("users", "orange", "局域网访问", "监听 0.0.0.0", "allowLan")}
          </div>
        </article>

        <div class="metric-grid">
          ${metric("upload", "", "实时上传", "2.48", "MB/s", "较 1 小时前 +12.4%")}
          ${metric("download", "cyan", "实时下载", "8.72", "MB/s", "较 1 小时前 +6.8%")}
          ${metric("link", "green", "活动连接", "128", "条", "峰值 246", "neutral")}
          ${metric("memory", "orange", "内存占用", "86.4", "MB", "稳定运行", "neutral")}
        </div>

        <div class="dashboard-lower">
          <article class="card traffic-card">
            <div class="card-head">
              <div><h3 class="card-title">实时流量</h3><p class="card-subtitle">最近 30 分钟网络吞吐</p></div>
              <div class="chart-legend"><span class="legend-item"><i class="legend-dot"></i>下载</span><span class="legend-item"><i class="legend-dot cyan"></i>上传</span></div>
            </div>
            <div class="traffic-chart-wrap">
              <svg class="traffic-chart" viewBox="0 0 760 220" preserveAspectRatio="none" role="img" aria-label="实时流量折线图">
                <defs>
                  <linearGradient id="trafficGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#4f6ff8" stop-opacity=".24" />
                    <stop offset="100%" stop-color="#4f6ff8" stop-opacity="0" />
                  </linearGradient>
                </defs>
                <line class="chart-grid-line" x1="52" y1="32" x2="744" y2="32"/><line class="chart-grid-line" x1="52" y1="76" x2="744" y2="76"/><line class="chart-grid-line" x1="52" y1="120" x2="744" y2="120"/><line class="chart-grid-line" x1="52" y1="164" x2="744" y2="164"/>
                <text class="chart-label" x="8" y="35">12 MB/s</text><text class="chart-label" x="14" y="79">8 MB/s</text><text class="chart-label" x="14" y="123">4 MB/s</text><text class="chart-label" x="30" y="167">0</text>
                <path class="chart-area" d="M52 164 L52 152 C72 148 84 130 102 138 S134 112 151 122 S181 148 201 131 S229 94 249 108 S276 142 297 128 S326 83 348 98 S378 129 399 117 S428 48 450 69 S479 131 501 115 S531 89 553 101 S581 146 604 129 S634 58 657 82 S686 118 708 97 S729 73 744 78 L744 164 Z"/>
                <path class="chart-line" d="M52 152 C72 148 84 130 102 138 S134 112 151 122 S181 148 201 131 S229 94 249 108 S276 142 297 128 S326 83 348 98 S378 129 399 117 S428 48 450 69 S479 131 501 115 S531 89 553 101 S581 146 604 129 S634 58 657 82 S686 118 708 97 S729 73 744 78"/>
                <path class="chart-line-secondary" d="M52 159 C82 155 96 146 120 151 S162 139 189 148 S229 153 254 144 S302 130 328 142 S371 151 398 146 S444 123 470 137 S516 151 545 143 S586 125 613 137 S656 143 682 132 S721 121 744 126"/>
                <circle class="chart-now" cx="744" cy="78" r="4"/>
                <text class="chart-label" x="52" y="196">30 分钟前</text><text class="chart-label" x="374" y="196">15 分钟前</text><text class="chart-label" x="723" y="196">现在</text>
              </svg>
            </div>
          </article>

          <div class="overview-stack">
            <article class="card active-profile">
              <div class="profile-top"><div class="profile-icon">${icon("config")}</div><span class="badge badge-success">使用中</span></div>
              <div class="profile-name">自定义配置 A</div>
              <div class="profile-description">规则模式 · 最后应用于今天 10:24</div>
              <div class="profile-stats">
                <div class="profile-stat"><strong>${inboundCount()}</strong><span>入站</span></div>
                <div class="profile-stat"><strong>12</strong><span>出站</span></div>
                <div class="profile-stat"><strong>16</strong><span>路由规则</span></div>
              </div>
            </article>
            <article class="card route-mode-card">
              <div class="mode-title-row"><h3 class="card-title">路由模式</h3><span class="faint" style="font-size:9px">即时生效</span></div>
              <div class="mode-options">
                ${["rule", "global", "direct"].map((mode) => `<button class="mode-option ${state.routeMode === mode ? "active" : ""}" data-mode="${mode}">${{ rule: "规则", global: "全局", direct: "直连" }[mode]}</button>`).join("")}
              </div>
            </article>
          </div>

          <article class="card connection-card">
            <div class="card-head">
              <div><h3 class="card-title">最近连接</h3><p class="card-subtitle">当前 128 条活动连接</p></div>
              <button class="btn btn-sm btn-quiet" data-action="view-connections">查看全部 ${icon("chevron")}</button>
            </div>
            <div class="table-wrap">
              <table class="data-table">
                <thead><tr><th>目标地址</th><th>协议</th><th>路由规则</th><th>出站</th><th>速度</th></tr></thead>
                <tbody>
                  ${connectionRow("G", "github.com:443", "TCP · TLS", "geosite-github", "节点选择 / HK-02", "2.8 MB/s")}
                  ${connectionRow("Y", "youtube.com:443", "UDP · QUIC", "geosite-youtube", "自动选择 / JP-01", "5.1 MB/s")}
                  ${connectionRow("C", "api.openai.com:443", "TCP · TLS", "domain-suffix", "节点选择 / US-03", "846 KB/s")}
                  ${connectionRow("B", "bilibili.com:443", "TCP · TLS", "geosite-cn", "direct", "1.3 MB/s")}
                </tbody>
              </table>
            </div>
          </article>
        </div>
      </div>
    </section>`;
  }

  function controlRow(iconName, tone, name, description, key) {
    return `<div class="control-row">
      <div class="control-icon ${tone}">${icon(iconName)}</div>
      <div class="control-copy"><div class="control-name">${name}</div><div class="control-desc">${description}</div></div>
      <button class="switch ${state[key] ? "on" : ""}" data-toggle="${key}" aria-label="切换${name}" aria-pressed="${state[key]}"></button>
    </div>`;
  }

  function connectionRow(letter, domain, protocol, rule, outbound, speed) {
    return `<tr>
      <td><div class="domain-cell"><span class="domain-favicon">${letter}</span><span class="table-primary">${domain}</span></div></td>
      <td>${protocol}</td><td><span class="type-badge">${rule}</span></td><td>${outbound}</td>
      <td><span class="speed-cell text-green">${icon("download")}${speed}</span></td>
    </tr>`;
  }

  function configIntro() {
    return `<div class="page-intro">
      <div class="page-intro-copy"><h2>配置工作台</h2><p>通过表单管理 sing-box 配置，保存后可直接应用到运行中的核心。</p></div>
      <div class="page-actions">
        <button class="btn" data-modal="json">${icon("code")}JSON 预览</button>
        <button class="btn btn-primary" id="save-config-button" data-action="save-config">${icon("save")}${state.dirty ? "保存更改" : "保存并应用"}</button>
      </div>
    </div>
    <article class="card profile-banner">
      <div class="profile-banner-icon">${icon("config")}</div>
      <div class="profile-banner-copy"><div class="profile-banner-title">自定义配置 A <span class="badge badge-success">当前配置</span></div><div class="profile-banner-desc">本地手动维护 · config/custom-a.json</div></div>
      <div class="profile-banner-meta"><div><span class="faint">最后保存</span><strong>${state.lastSaved}</strong></div><div><span class="faint">配置版本</span><strong>v12</strong></div></div>
    </article>
    <nav class="config-tabs" aria-label="配置分类">
      ${configTabs.map((tab) => `<button class="config-tab ${state.activeConfig === tab.id ? "active" : ""}" data-config="${tab.id}">${icon(tab.icon)}${tab.label}</button>`).join("")}
    </nav>`;
  }

  function renderConfigPage() {
    let content = renderGeneralConfig();
    if (state.activeConfig === "inbounds") content = renderInboundConfig();
    if (state.activeConfig === "outbounds") content = renderOutboundConfig();
    if (state.activeConfig === "routing") content = renderRoutingConfig();
    if (state.activeConfig === "dns") content = renderDnsConfig();
    return `<section>${configIntro()}${content}</section>`;
  }

  function formField(label, name, value, options = {}) {
    const { type = "text", hint = "", suffix = "", full = false } = options;
    if (type === "select") {
      return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}${hint ? `<small>${hint}</small>` : ""}</span><select class="field-select" data-config-field="${name}">${options.items.map((item) => `<option ${item === value ? "selected" : ""}>${item}</option>`).join("")}</select></label>`;
    }
    return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}${hint ? `<small>${hint}</small>` : ""}</span><span class="${suffix ? "input-suffix-wrap" : ""}"><input class="field-input" type="${type}" value="${value}" data-config-field="${name}" />${suffix ? `<span class="input-suffix">${suffix}</span>` : ""}</span></label>`;
  }

  function settingToggle(name, description, key) {
    return `<div class="setting-row"><div class="setting-copy"><div class="setting-name">${name}</div><div class="setting-desc">${description}</div></div><button class="switch ${state[key] ? "on" : ""}" data-toggle="${key}" aria-label="切换${name}" aria-pressed="${state[key]}"></button></div>`;
  }

  function renderGeneralConfig() {
    return `<div class="config-layout">
      <div class="config-main">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">基础参数</h3><p class="card-subtitle">配置名称、日志与服务端口</p></div><span class="type-badge">general</span></div>
          <div class="card-body"><div class="form-grid">
            ${formField("配置名称", "profileName", "自定义配置 A", { hint: "仅用于控制台展示" })}
            ${formField("日志等级", "logLevel", "info", { type: "select", items: ["debug", "info", "warn", "error", "panic"] })}
            ${formField("Mixed 入站端口", "mixedPort", "7890", { type: "number", suffix: "端口" })}
            ${formField("Clash API 端口", "apiPort", "9090", { type: "number", suffix: "端口" })}
            ${formField("缓存文件", "cacheFile", "cache.db", { hint: "工作目录下" })}
            ${formField("网络接口", "interface", "自动选择", { type: "select", items: ["自动选择", "eth0", "wlan0", "en0"] })}
          </div></div>
        </article>
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">核心功能</h3><p class="card-subtitle">常用实验特性与兼容选项</p></div></div>
          <div class="card-body setting-list">
            ${settingToggle("启用 IPv6", "允许 DNS 与出站连接使用 IPv6 网络", "ipv6")}
            ${settingToggle("Clash API 兼容", "开放外部控制接口与流量统计", "clashApi")}
            ${settingToggle("FakeIP 持久化", "在核心重启后保留 FakeIP 映射", "fakeIp")}
          </div>
        </article>
      </div>
      <aside class="config-aside">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">配置摘要</h3><p class="card-subtitle">当前配置结构</p></div></div>
          <div class="card-body setting-list">
            ${summarySetting("入站", `${inboundCount()} 个监听器`, "inbound")}
            ${summarySetting("出站", "12 个节点 / 4 个组", "outbound")}
            ${summarySetting("路由", "16 条规则", "route")}
            ${summarySetting("DNS", "4 个服务器", "dns")}
          </div>
        </article>
        <div class="info-callout">${icon("info")}<span>配置保存前会执行语法检查。应用失败时将自动回滚到上一个可用版本。</span></div>
        ${renderCodePreview(false)}
      </aside>
    </div>`;
  }

  function summarySetting(name, value, iconName) {
    return `<div class="setting-row"><div class="entry-icon" style="width:32px;height:32px">${icon(iconName)}</div><div class="setting-copy"><div class="setting-name">${name}</div><div class="setting-desc">${value}</div></div>${icon("chevron", "faint")}</div>`;
  }

  function renderCodePreview(full = false) {
    return `<div class="code-preview ${full ? "code-preview-full" : ""}">
      <div class="code-preview-head"><span class="window-dots"><i></i><i></i><i></i></span><span>config.json</span><button class="entry-action" data-action="copy-json" title="复制 JSON">${icon("copy")}</button></div>
      <pre>{
  <span class="code-key">"log"</span>: { <span class="code-key">"level"</span>: <span class="code-string">"info"</span> },
  <span class="code-key">"inbounds"</span>: [
    { <span class="code-key">"type"</span>: <span class="code-string">"mixed"</span>, <span class="code-key">"listen_port"</span>: <span class="code-number">7890</span> },
    { <span class="code-key">"type"</span>: <span class="code-string">"tun"</span>, <span class="code-key">"auto_route"</span>: <span class="code-number">true</span> }
  ],
  <span class="code-key">"route"</span>: { <span class="code-key">"final"</span>: <span class="code-string">"节点选择"</span> }
}</pre>
    </div>`;
  }

  function renderInboundConfig() {
    return `<div class="config-layout">
      <div class="config-main">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">入站监听器</h3><p class="card-subtitle">流量进入 sing-box 的入口</p></div><button class="btn btn-sm btn-primary" data-modal="inbound">${icon("plus")}新增入站</button></div>
          <div class="entry-list">
            ${entryCard("mixed-in", "Mixed Proxy", "mixed", "127.0.0.1:7890", "HTTP 与 SOCKS 复合代理", "inbound", "")}
            ${entryCard("tun-in", "TUN Interface", "tun", "singtun0 · 172.19.0.1/30", "自动路由 · 严格路由", "shield", "cyan")}
            ${entryCard("dns-in", "DNS Listener", "direct", "127.0.0.1:5353", "DNS 劫持入口", "dns", "green")}
            ${state.formInbounds.map(formInboundCard).join("")}
            ${state.customInbounds.map(customInboundCard).join("")}
          </div>
        </article>
      </div>
      <aside class="config-aside">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">入站选项</h3><p class="card-subtitle">全局监听行为</p></div></div>
          <div class="card-body setting-list">
            ${settingToggle("嗅探协议", "识别 HTTP、TLS 与 QUIC 流量", "clashApi")}
            ${settingToggle("覆盖目标地址", "使用嗅探结果重写目标", "fakeIp")}
            ${settingToggle("允许局域网", "接受来自其他设备的连接", "allowLan")}
          </div>
        </article>
        <div class="info-callout">${icon("shield")}<span>TUN 模式需要系统网络管理权限。首次启用时，浏览器端应引导用户完成授权。</span></div>
      </aside>
    </div>`;
  }

  function entryCard(id, title, type, meta, description, iconName, tone) {
    return `<div class="entry-card" data-entry="${id}">
      <span class="entry-drag">${icon("grip")}</span><div class="entry-icon ${tone}">${icon(iconName)}</div>
      <div class="entry-copy"><div class="entry-title-row"><span class="entry-title">${title}</span><span class="type-badge">${type}</span><span class="status-badge status-online">启用</span></div><div class="entry-meta"><span>${meta}</span><span class="dot-separator"></span><span>${description}</span></div></div>
      <div class="entry-actions"><button class="entry-action" data-action="edit-entry" title="编辑">${icon("edit")}</button><button class="entry-action" data-action="duplicate-entry" title="复制">${icon("copy")}</button><button class="entry-action danger" data-action="remove-entry" title="删除">${icon("trash")}</button></div>
    </div>`;
  }

  function customInboundCard(item) {
    const config = item.config;
    const title = escapeHtml(config.tag || "自定义入站");
    const type = escapeHtml(config.type);
    const listener = config.listen || config.interface_name || "由完整 JSON 定义";
    const endpoint = config.listen_port ? `${listener}:${config.listen_port}` : listener;
    return `<div class="entry-card" data-entry="${item.id}">
      <span class="entry-drag">${icon("grip")}</span><div class="entry-icon violet">${icon("code")}</div>
      <div class="entry-copy"><div class="entry-title-row"><span class="entry-title">${title}</span><span class="type-badge">${type}</span><span class="badge badge-primary">自定义 JSON</span></div><div class="entry-meta"><span>${escapeHtml(endpoint)}</span><span class="dot-separator"></span><span>完整配置 · ${Object.keys(config).length} 个字段</span></div></div>
      <div class="entry-actions"><button class="entry-action" data-modal="inbound" data-id="${item.id}" title="编辑 JSON">${icon("edit")}</button><button class="entry-action" data-action="copy-custom-inbound" data-id="${item.id}" title="复制 JSON">${icon("copy")}</button><button class="entry-action danger" data-action="remove-custom-inbound" data-id="${item.id}" title="删除">${icon("trash")}</button></div>
    </div>`;
  }

  function formInboundCard(item) {
    const config = item.config;
    const enabled = item.enabled !== false;
    const listener = config.interface_name || config.listen || "由类型配置定义";
    const endpoint = config.listen_port ? `${listener}:${config.listen_port}` : listener;
    const descriptions = {
      mixed: "HTTP 与 SOCKS 复合代理",
      tun: "虚拟网卡与自动路由",
      socks: "SOCKS4 / SOCKS5 代理",
      http: "HTTP 代理服务",
      direct: "透明流量直连入口",
    };
    return `<div class="entry-card" data-entry="${item.id}">
      <span class="entry-drag">${icon("grip")}</span><div class="entry-icon">${icon(config.type === "tun" ? "shield" : "inbound")}</div>
      <div class="entry-copy"><div class="entry-title-row"><span class="entry-title">${escapeHtml(config.tag || `${config.type} 入站`)}</span><span class="type-badge">${escapeHtml(config.type)}</span><span class="status-badge ${enabled ? "status-online" : "badge-orange"}">${enabled ? "启用" : "停用"}</span></div><div class="entry-meta"><span>${escapeHtml(endpoint)}</span><span class="dot-separator"></span><span>${descriptions[config.type] || "表单配置"}</span></div></div>
      <div class="entry-actions"><button class="entry-action" data-action="edit-entry" title="编辑">${icon("edit")}</button><button class="entry-action" data-action="duplicate-entry" title="复制">${icon("copy")}</button><button class="entry-action danger" data-action="remove-form-inbound" data-id="${item.id}" title="删除">${icon("trash")}</button></div>
    </div>`;
  }

  function renderOutboundConfig() {
    return `<div class="config-layout">
      <div class="config-main">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">出站与策略组</h3><p class="card-subtitle">决定流量最终使用的出口</p></div><button class="btn btn-sm btn-primary" data-modal="outbound">${icon("plus")}新增出站</button></div>
          <div class="outbound-grid">
            ${state.outbounds.map((outbound) => outboundCard(outbound)).join("")}
          </div>
        </article>
      </div>
      <aside class="config-aside">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">默认链路</h3><p class="card-subtitle">当前出站拓扑</p></div></div>
          <div class="card-body topology">
            <div class="topology-node active"><span class="topology-dot"></span>路由规则</div>
            <div class="topology-node"><span class="topology-dot"></span>节点选择 · selector</div>
            <div class="topology-node"><span class="topology-dot"></span>HK-02 · hysteria2</div>
            <div class="topology-node"><span class="topology-dot"></span>Internet</div>
          </div>
        </article>
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">健康检查</h3><p class="card-subtitle">策略组自动测速参数</p></div></div>
          <div class="card-body"><div class="form-grid" style="grid-template-columns:1fr">
            ${formField("测试地址", "testUrl", "https://www.gstatic.com/generate_204")}
            ${formField("测试间隔", "testInterval", "5", { suffix: "分钟" })}
            ${formField("容差", "tolerance", "50", { suffix: "ms" })}
          </div></div>
        </article>
      </aside>
    </div>`;
  }

  function outboundCard(outbound) {
    const meta = {
      selector: { description: "手动选择引用的出站或订阅节点", iconName: "globe", tone: "" },
      urltest: { description: `每 ${outbound.interval || "5m"} 自动测速`, iconName: "gauge", tone: "cyan" },
      direct: { description: "不经过代理服务器", iconName: "lightning", tone: "green" },
      block: { description: "直接阻断匹配流量", iconName: "shield", tone: "orange" },
    }[outbound.type] || { description: "自定义出站", iconName: "outbound", tone: "violet" };
    const refs = outbound.refs || [];
    const fallbackMembers = outbound.type === "direct" ? ["本地"] : outbound.type === "block" ? ["阻断"] : [];
    const members = refs.length ? refs.slice(0, 3).map((reference) => reference.tag) : fallbackMembers;
    const extraCount = Math.max(0, refs.length - members.length);
    const countLabel = ["selector", "urltest"].includes(outbound.type) ? `${refs.length} 个引用` : "内置出站";
    return `<div class="outbound-card" data-outbound-id="${escapeHtml(outbound.id)}">
      <div class="outbound-head"><div class="entry-icon ${meta.tone}">${icon(meta.iconName)}</div><div class="outbound-name"><strong>${escapeHtml(outbound.tag)}</strong><span>${escapeHtml(meta.description)}</span></div><button class="entry-action" data-modal="outbound" data-id="${escapeHtml(outbound.id)}" title="编辑出站">${icon("edit")}</button></div>
      <div class="entry-meta" style="margin-top:11px"><span class="type-badge">${escapeHtml(outbound.type)}</span><span>${countLabel}</span>${outbound.hidden ? '<span class="status-badge badge-orange">隐藏</span>' : ""}</div>
      <div class="outbound-members">${members.map((member, index) => `<span class="member-avatar" style="z-index:${members.length - index}" title="${escapeHtml(member)}">${escapeHtml(outboundMemberInitial(member))}</span>`).join("")}${extraCount ? `<span class="member-avatar more">+${extraCount}</span>` : ""}</div>
    </div>`;
  }

  function outboundMemberInitial(name) {
    const cleanName = String(name).replace(/^\p{Extended_Pictographic}+\s*/u, "").trim();
    if (/^[a-z]/i.test(cleanName)) return cleanName.slice(0, 2).toUpperCase();
    return cleanName.slice(0, 2) || "--";
  }

  function renderRoutingConfig() {
    return `<div class="config-layout">
      <div class="config-main">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">路由规则</h3><p class="card-subtitle">按顺序匹配，拖动可调整优先级</p></div><button class="btn btn-sm btn-primary" data-modal="route">${icon("plus")}新增规则</button></div>
          <div class="entry-list">
            ${routeRule(1, "广告拦截", "rule_set", "ads-all", "block", "red")}
            ${routeRule(2, "私有网络直连", "ip_is_private", "true", "direct", "green")}
            ${routeRule(3, "中国大陆网站", "rule_set", "geosite-cn, geoip-cn", "direct", "green")}
            ${routeRule(4, "流媒体服务", "rule_set", "youtube, netflix, spotify", "节点选择", "")}
            ${routeRule(5, "AI 服务", "domain_suffix", "openai.com, anthropic.com", "US-自动选择", "orange")}
          </div>
        </article>
      </div>
      <aside class="config-aside">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">默认规则</h3><p class="card-subtitle">无规则命中时使用</p></div></div>
          <div class="card-body">
            <label class="form-field"><span class="field-label">最终出站</span><select class="field-select" data-config-field="routeFinal"><option>节点选择</option><option>自动选择</option><option>direct</option><option>block</option></select></label>
            <div class="info-callout" style="margin-top:12px">${icon("info")}<span>推荐将最终出站设置为策略组，避免新增节点后重复修改路由规则。</span></div>
          </div>
        </article>
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">匹配统计</h3><p class="card-subtitle">过去 24 小时</p></div></div>
          <div class="card-body setting-list">
            ${statRow("中国大陆网站", "68.2%", "green")}${statRow("节点选择", "24.6%", "")}${statRow("广告拦截", "6.8%", "red")}${statRow("其他", "0.4%", "orange")}
          </div>
        </article>
      </aside>
    </div>`;
  }

  function routeRule(order, title, type, value, target, tone) {
    return `<div class="entry-card"><span class="entry-drag">${icon("grip")}</span><span class="rule-order">${order}</span><div class="entry-copy"><div class="entry-title-row"><span class="entry-title">${title}</span><span class="type-badge">${type}</span></div><div class="entry-meta"><span>${value}</span></div></div><span class="route-target ${tone ? `badge-${tone}` : ""}">${icon("route")}${target}</span><div class="entry-actions"><button class="entry-action" data-action="edit-entry">${icon("edit")}</button><button class="entry-action danger" data-action="remove-entry">${icon("trash")}</button></div></div>`;
  }

  function statRow(name, value, tone) {
    return `<div class="setting-row"><span class="legend-dot ${tone === "green" ? "" : tone === "red" ? "" : "cyan"}" style="${tone === "green" ? "background:var(--green)" : tone === "red" ? "background:var(--red)" : tone === "orange" ? "background:var(--orange)" : ""}"></span><div class="setting-copy"><div class="setting-name">${name}</div></div><strong style="font-size:11px">${value}</strong></div>`;
  }

  function renderDnsConfig() {
    return `<div class="config-layout">
      <div class="config-main">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">DNS 服务器</h3><p class="card-subtitle">解析器与传输协议</p></div><button class="btn btn-sm btn-primary" data-modal="dns">${icon("plus")}新增服务器</button></div>
          <div class="dns-server-grid">
            ${dnsServer("Cloudflare DoH", "https://1.1.1.1/dns-query", "remote", "globe", "")}
            ${dnsServer("AliDNS DoH", "https://223.5.5.5/dns-query", "domestic", "dns", "cyan")}
            ${dnsServer("FakeIP", "fakeip", "virtual", "sparkles", "orange")}
            ${dnsServer("本地 DNS", "local", "bootstrap", "server", "green")}
          </div>
        </article>
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">DNS 规则</h3><p class="card-subtitle">为不同域名选择解析器</p></div><button class="btn btn-sm" data-modal="dns-rule">${icon("plus")}新增规则</button></div>
          <div class="entry-list">
            ${routeRule(1, "中国大陆域名", "rule_set", "geosite-cn", "AliDNS DoH", "green")}
            ${routeRule(2, "国外域名", "rule_set", "geosite-geolocation-!cn", "Cloudflare DoH", "")}
            ${routeRule(3, "默认解析", "final", "未命中规则", "Cloudflare DoH", "orange")}
          </div>
        </article>
      </div>
      <aside class="config-aside">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">DNS 行为</h3><p class="card-subtitle">缓存与独立解析策略</p></div></div>
          <div class="card-body setting-list">
            ${settingToggle("启用 DNS 缓存", "减少重复查询并降低延迟", "fakeIp")}
            ${settingToggle("独立缓存", "每个 DNS 服务器使用独立缓存", "clashApi")}
            ${settingToggle("反向映射", "记录 IP 与域名映射供路由使用", "ipv6")}
          </div>
        </article>
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">FakeIP 地址池</h3><p class="card-subtitle">虚拟地址分配范围</p></div></div>
          <div class="card-body"><div class="form-grid" style="grid-template-columns:1fr">${formField("IPv4 范围", "fakeip4", "198.18.0.0/15")}${formField("IPv6 范围", "fakeip6", "fc00::/18")}</div></div>
        </article>
      </aside>
    </div>`;
  }

  function dnsServer(name, address, tag, iconName, tone) {
    return `<div class="dns-server"><div class="entry-icon ${tone}">${icon(iconName)}</div><div class="dns-server-copy"><strong>${name}</strong><span>${address}</span></div><span class="type-badge">${tag}</span><button class="entry-action" data-action="edit-entry">${icon("more")}</button></div>`;
  }

  function renderSubscriptions() {
    const totalNodes = state.subscriptions.reduce((sum, item) => sum + item.nodes, 0);
    return `<section>
      <div class="page-intro"><div class="page-intro-copy"><h2>订阅配置</h2><p>集中管理远程节点订阅，更新后自动合并到当前配置。</p></div><div class="page-actions"><button class="btn" data-action="sync-all-subscriptions">${icon("rotate")}全部同步</button><button class="btn btn-primary" data-modal="subscription">${icon("plus")}添加订阅</button></div></div>
      <div class="summary-grid">
        ${summaryMetric("subscription", "", state.subscriptions.length, "订阅来源", "全部可用")}
        ${summaryMetric("globe", "cyan", totalNodes, "可用节点", "3 个节点延迟较高")}
        ${summaryMetric("download", "green", "93.1 GB", "剩余流量", "合计 300 GB")}
      </div>
      <div class="subscription-grid">
        ${state.subscriptions.map(renderSubscriptionCard).join("")}
        <button class="card empty-state" data-modal="subscription"><div><div class="empty-icon">${icon("plus")}</div><h3>添加新的订阅</h3><p>支持 sing-box、Clash 与通用订阅链接</p><span class="btn btn-sm btn-primary">开始添加</span></div></button>
      </div>
    </section>`;
  }

  function summaryMetric(iconName, tone, value, label, detail) {
    return `<article class="card summary-card"><div class="metric-icon ${tone}">${icon(iconName)}</div><div><div class="summary-value">${value}</div><div class="summary-label">${label} · ${detail}</div></div></article>`;
  }

  function renderSubscriptionCard(item) {
    const percent = Math.round((item.used / item.total) * 100);
    return `<article class="card subscription-card">
      <div class="subscription-main">
        <div class="subscription-top"><div class="provider-logo ${item.tone === "cyan" ? "cyan" : ""}">${item.logo}</div><div class="subscription-name"><strong>${item.name}</strong><span>远程订阅 · 自动更新</span></div><span class="badge badge-success">可用</span><button class="entry-action" data-action="subscription-more">${icon("more")}</button></div>
        <div class="usage-row"><div class="usage-title">本期已用<strong>${item.used} GB / ${item.total} GB</strong></div><span class="usage-percent">${percent}%</span></div>
        <div class="progress"><div class="progress-bar" style="width:${percent}%"></div></div>
        <div class="subscription-meta"><div class="subscription-meta-item"><span>节点数量</span><strong>${item.nodes} 个</strong></div><div class="subscription-meta-item"><span>到期时间</span><strong>${item.expires}</strong></div><div class="subscription-meta-item"><span>更新频率</span><strong>${item.interval}</strong></div></div>
      </div>
      <div class="subscription-footer"><span class="subscription-update">上次更新：${item.updated}</span><div class="inline-actions"><button class="btn btn-sm btn-quiet" data-action="edit-subscription" data-id="${item.id}">${icon("edit")}编辑</button><button class="btn btn-sm" data-action="sync-subscription" data-id="${item.id}">${icon("rotate")}立即同步</button></div></div>
    </article>`;
  }

  function renderRulesets() {
    const filtered = state.rulesets.filter((item) => {
      const matchesSearch = `${item.name} ${item.source}`.toLowerCase().includes(state.rulesetSearch.toLowerCase());
      const matchesType = state.rulesetType === "all" || item.type === state.rulesetType;
      return matchesSearch && matchesType;
    });
    return `<section>
      <div class="page-intro"><div class="page-intro-copy"><h2>规则集</h2><p>维护可复用的域名与 IP 规则资源，用于路由和 DNS 策略。</p></div><div class="page-actions"><button class="btn" data-action="update-rulesets">${icon("rotate")}更新全部</button><button class="btn btn-primary" data-modal="ruleset">${icon("plus")}添加规则集</button></div></div>
      <div class="summary-grid">
        ${summaryMetric("database", "", state.rulesets.length, "规则集", "4 个远程资源")}
        ${summaryMetric("rules", "cyan", "436,713", "规则条目", "已加载到内存")}
        ${summaryMetric("memory", "orange", "16.3 MB", "磁盘占用", "缓存状态正常")}
      </div>
      <div class="card toolbar">
        <label class="search-wrap">${icon("search")}<input class="search-input" type="search" placeholder="搜索规则集名称或来源..." value="${state.rulesetSearch}" data-ruleset-search /></label>
        <select class="field-select filter-select" data-ruleset-type><option value="all" ${state.rulesetType === "all" ? "selected" : ""}>全部类型</option><option value="remote" ${state.rulesetType === "remote" ? "selected" : ""}>远程规则</option><option value="local" ${state.rulesetType === "local" ? "selected" : ""}>本地规则</option></select>
        <button class="btn btn-sm" data-action="ruleset-filter">${icon("config")}筛选</button>
      </div>
      <article class="card ruleset-table-card">
        ${filtered.length ? `<div class="table-wrap"><table class="data-table"><thead><tr><th>规则集</th><th>类型</th><th>格式</th><th>规则数</th><th>大小</th><th>最近更新</th><th style="text-align:right">操作</th></tr></thead><tbody>${filtered.map(ruleSetRow).join("")}</tbody></table></div>` : `<div class="empty-state"><div><div class="empty-icon">${icon("search")}</div><h3>没有匹配的规则集</h3><p>调整搜索关键词或筛选条件后重试</p><button class="btn btn-sm" data-action="clear-ruleset-filter">清除筛选</button></div></div>`}
      </article>
    </section>`;
  }

  function ruleSetRow(item) {
    return `<tr><td><div class="ruleset-name"><span class="ruleset-icon">${icon(item.type === "remote" ? "globe" : "folder")}</span><div><div class="ruleset-main">${item.name}</div><div class="ruleset-sub">${item.source}</div></div></div></td><td><span class="badge ${item.type === "remote" ? "badge-primary" : "badge-orange"}">${item.type === "remote" ? "远程" : "本地"}</span></td><td><span class="type-badge">${item.format}</span></td><td><span class="rule-count">${item.count}</span></td><td>${item.size}</td><td>${item.updated}</td><td><div class="table-actions"><button class="entry-action" data-action="refresh-ruleset" data-id="${item.id}" title="更新">${icon("rotate")}</button><button class="entry-action" data-action="edit-entry" title="编辑">${icon("edit")}</button><button class="entry-action" title="更多">${icon("more")}</button></div></td></tr>`;
  }

  function renderSettings() {
    return `<section>
      <div class="page-intro"><div class="page-intro-copy"><h2>系统设置</h2><p>管理核心行为、控制台偏好与安全访问方式。</p></div><div class="page-actions"><button class="btn btn-primary" data-action="save-settings">${icon("save")}保存设置</button></div></div>
      <div class="settings-layout">
        <nav class="card settings-nav" aria-label="设置分类">
          ${settingsNavItem("core-settings", "cpu", "核心设置")}${settingsNavItem("network-settings", "globe", "网络与端口")}${settingsNavItem("appearance-settings", "palette", "界面外观")}${settingsNavItem("security-settings", "lock", "安全访问")}${settingsNavItem("about-settings", "info", "关于")}
        </nav>
        <div class="settings-content">
          <article class="card settings-section" id="core-settings">
            <div class="card-head"><div><h3 class="card-title">核心设置</h3><p class="card-subtitle">sing-box 服务启动与运行行为</p></div><span class="badge badge-success">v1.11.11</span></div>
            <div class="card-body setting-list">
              ${settingsToggleRow("开机自动运行", "控制台启动后自动拉起 sing-box 核心", "autoStart")}
              ${settingsToggleRow("配置变更自动重载", "保存配置后无感重载核心服务", "clashApi")}
              ${settingsSelectRow("核心分支", "选择使用的 sing-box 发布通道", "Stable · 1.11.11", ["Stable · 1.11.11", "Beta · 1.12.0-beta.3"])}
              ${settingsActionRow("检查核心更新", "当前已是最新稳定版本", "检查更新", "check-update")}
            </div>
          </article>
          <article class="card settings-section" id="network-settings">
            <div class="card-head"><div><h3 class="card-title">网络与端口</h3><p class="card-subtitle">控制 API、代理端口与局域网访问</p></div></div>
            <div class="card-body setting-list">
              ${settingsInputRow("控制台端口", "Web 管理页面监听端口", "8080")}
              ${settingsInputRow("外部控制端口", "Clash API 兼容接口", "9090")}
              ${settingsToggleRow("允许局域网访问", "允许同一网络内的设备访问控制台", "allowLan")}
              ${settingsToggleRow("启用 IPv6", "监听 IPv6 地址并允许 IPv6 出站", "ipv6")}
            </div>
          </article>
          <article class="card settings-section" id="appearance-settings">
            <div class="card-head"><div><h3 class="card-title">界面外观</h3><p class="card-subtitle">只影响当前浏览器中的控制台展示</p></div></div>
            <div class="card-body setting-list">
              ${settingsSelectRow("显示模式", "跟随系统或固定使用浅色、深色主题", state.theme === "dark" ? "深色" : "浅色", ["浅色", "深色", "跟随系统"], "theme-select")}
              ${settingsSelectRow("界面语言", "控制台菜单与提示文本", "简体中文", ["简体中文", "繁體中文", "English"])}
              <div class="setting-row"><div class="setting-copy"><div class="setting-name">主题色</div><div class="setting-desc">用于按钮、选中状态与图表</div></div><div class="settings-control color-swatches"><button class="color-swatch ${state.accent === "blue" ? "active" : ""}" style="--swatch:#4f6ff8" data-accent="blue" aria-label="蓝色"></button><button class="color-swatch ${state.accent === "cyan" ? "active" : ""}" style="--swatch:#13aebf" data-accent="cyan" aria-label="青色"></button><button class="color-swatch ${state.accent === "violet" ? "active" : ""}" style="--swatch:#8067e8" data-accent="violet" aria-label="紫色"></button></div></div>
              ${settingsToggleRow("紧凑数据显示", "缩小表格行高，在一屏展示更多内容", "fakeIp")}
            </div>
          </article>
          <article class="card settings-section" id="security-settings">
            <div class="card-head"><div><h3 class="card-title">安全访问</h3><p class="card-subtitle">登录验证与远程管理保护</p></div></div>
            <div class="card-body setting-list">
              ${settingsToggleRow("启用登录验证", "访问控制台时需要管理员账号", "autoStart")}
              ${settingsActionRow("管理员密码", "上次修改于 30 天前", "修改密码", "change-password")}
              ${settingsSelectRow("会话有效期", "无操作后自动退出登录", "24 小时", ["1 小时", "8 小时", "24 小时", "7 天"])}
            </div>
          </article>
          <article class="card about-card" id="about-settings">
            <div class="brand-mark"><span>S</span></div><div class="about-copy"><strong>SingBox Control</strong><span>Web 原型 v0.1.0 · sing-box 核心 v1.11.11</span></div><button class="btn btn-sm" data-action="project-info">项目说明 ${icon("external")}</button>
          </article>
        </div>
      </div>
    </section>`;
  }

  function settingsNavItem(id, iconName, label) {
    return `<button class="settings-nav-item ${state.settingsSection === id ? "active" : ""}" data-settings-section="${id}">${icon(iconName)}${label}</button>`;
  }

  function settingsToggleRow(name, description, key) {
    return `<div class="setting-row"><div class="setting-copy"><div class="setting-name">${name}</div><div class="setting-desc">${description}</div></div><div class="settings-control"><button class="switch ${state[key] ? "on" : ""}" data-toggle="${key}" aria-pressed="${state[key]}"></button></div></div>`;
  }

  function settingsSelectRow(name, description, value, items, dataAttribute = "") {
    return `<div class="setting-row"><div class="setting-copy"><div class="setting-name">${name}</div><div class="setting-desc">${description}</div></div><div class="settings-control"><select class="field-select" ${dataAttribute ? `data-${dataAttribute}` : ""}>${items.map((item) => `<option ${item === value ? "selected" : ""}>${item}</option>`).join("")}</select></div></div>`;
  }

  function settingsInputRow(name, description, value) {
    return `<div class="setting-row"><div class="setting-copy"><div class="setting-name">${name}</div><div class="setting-desc">${description}</div></div><div class="settings-control"><input class="field-input" value="${value}" inputmode="numeric" data-settings-input /></div></div>`;
  }

  function settingsActionRow(name, description, button, action) {
    return `<div class="setting-row"><div class="setting-copy"><div class="setting-name">${name}</div><div class="setting-desc">${description}</div></div><div class="settings-control"><button class="btn btn-sm" data-action="${action}">${button}</button></div></div>`;
  }

  function openModal(type, itemId = null) {
    let content = "";
    state.editingCustomInboundId = type === "inbound" ? itemId : null;
    state.editingOutboundId = type === "outbound" ? itemId : null;
    if (type === "subscription") content = subscriptionModal();
    else if (type === "inbound") content = inboundModal(itemId);
    else if (type === "outbound") content = outboundModal(itemId);
    else if (["route", "dns-rule"].includes(type)) content = genericModal(type === "route" ? "新增路由规则" : "新增 DNS 规则", "规则将添加到当前列表末尾", "rule", [
      ["规则名称", "新规则"], ["匹配类型", "domain_suffix", "select", ["domain", "domain_suffix", "ip_cidr", "rule_set", "process_name"]], ["匹配内容", ""], ["目标出站", "节点选择", "select", ["节点选择", "自动选择", "direct", "block"]]
    ]);
    else if (type === "dns") content = genericModal("新增 DNS 服务器", "配置上游 DNS 解析器", "dns", [
      ["服务器标签", "dns-remote"], ["服务器类型", "https", "select", ["https", "tls", "quic", "udp", "local"]], ["服务器地址", "https://1.1.1.1/dns-query"], ["解析路由", "节点选择", "select", ["节点选择", "direct"]]
    ]);
    else if (type === "ruleset") content = genericModal("添加规则集", "从远程 URL 或本地文件加载规则", "ruleset", [
      ["规则集标签", "new-ruleset"], ["来源类型", "remote", "select", ["remote", "local"]], ["资源 URL", "https://example.com/rules.srs"], ["格式", "binary", "select", ["binary", "source"]]
    ]);
    else if (type === "json") content = jsonModal();
    else if (type === "command") content = commandModal();
    modalRoot.innerHTML = content;
    document.body.style.overflow = "hidden";
    window.setTimeout(() => modalRoot.querySelector(".modal-body input, .modal-body select, .modal-body textarea")?.focus(), 30);
  }

  function modalFrame(title, description, body, footer = "", modalClass = "") {
    return `<div class="modal-backdrop" data-action="modal-backdrop"><section class="modal ${modalClass}" role="dialog" aria-modal="true" aria-labelledby="modal-title"><div class="modal-head"><div class="modal-title-wrap"><h2 class="modal-title" id="modal-title">${title}</h2><p class="modal-description">${description}</p></div><button class="modal-close" data-action="close-modal" aria-label="关闭">${icon("close")}</button></div>${body}${footer}</section></div>`;
  }

  function validateCustomInboundJson(json) {
    try {
      const config = JSON.parse(json);
      if (!config || typeof config !== "object" || Array.isArray(config)) {
        return { valid: false, message: "请输入单个入站 JSON 对象，不能使用数组。" };
      }
      if (typeof config.type !== "string" || !config.type.trim()) {
        return { valid: false, message: "入站配置必须包含非空的 type 字段。" };
      }
      return {
        valid: true,
        config,
        message: `JSON 有效 · ${config.tag ? `标签 ${config.tag}` : "未设置 tag"} · 类型 ${config.type}`,
      };
    } catch (error) {
      return { valid: false, message: `JSON 语法错误：${error.message}` };
    }
  }

  function customValidationMarkup(result) {
    return `<div class="custom-json-validation ${result.valid ? "valid" : "invalid"}" id="custom-json-validation">${icon(result.valid ? "check" : "info")}<span>${escapeHtml(result.message)}</span></div>`;
  }

  function inboundInput(label, name, value, options = {}) {
    const { type = "text", full = false, hint = "", required = false, placeholder = "" } = options;
    return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}${hint ? `<small>${hint}</small>` : ""}</span><input class="field-input" name="${name}" type="${type}" value="${escapeHtml(value)}" placeholder="${escapeHtml(placeholder)}" ${required ? "required" : ""} /></label>`;
  }

  function inboundSelect(label, name, value, items, options = {}) {
    return `<label class="form-field ${options.full ? "full" : ""}"><span class="field-label">${label}${options.hint ? `<small>${options.hint}</small>` : ""}</span><select class="field-select" name="${name}" ${options.dataAttribute || ""}>${items.map(([itemValue, itemLabel]) => `<option value="${itemValue}" ${String(itemValue) === String(value) ? "selected" : ""}>${itemLabel}</option>`).join("")}</select></label>`;
  }

  function inboundToggle(label, description, name, checked = false) {
    return `<div class="tun-toggle-card"><div class="tun-toggle-copy"><strong>${label}</strong><span>${description}</span></div><label class="form-switch"><input type="checkbox" name="${name}" value="true" ${checked ? "checked" : ""} /><span class="switch"></span></label></div>`;
  }

  function inboundStackControl(value = "mixed") {
    return `<div class="form-field full"><span class="field-label">TUN 模式堆栈<small>stack</small></span><div class="tun-stack-control">${[["system", "System"], ["gvisor", "gVisor"], ["mixed", "Mixed"]].map(([stackValue, label]) => `<label><input type="radio" name="stack" value="${stackValue}" ${value === stackValue ? "checked" : ""} /><span>${label}</span></label>`).join("")}</div></div>`;
  }

  function tunListItem(name, value) {
    return `<div class="tun-list-item"><input class="field-input" name="${name}" value="${escapeHtml(value)}" aria-label="${name}" /><button class="tun-list-remove" type="button" data-action="remove-tun-list-item" title="删除">${icon("close")}</button></div>`;
  }

  function tunListField(label, name, values, placeholder, hint) {
    return `<div class="form-field full tun-list-field" data-tun-list-field data-list-name="${name}"><span class="field-label">${label}<small>${hint}</small></span><div class="tun-list-items" data-tun-list-items>${values.map((value) => tunListItem(name, value)).join("")}</div><div class="tun-list-add"><input class="field-input" data-tun-list-input placeholder="${escapeHtml(placeholder)}" /><button type="button" data-action="add-tun-list-item" title="添加">${icon("plus")}</button></div></div>`;
  }

  function renderCustomInboundEditor(json = customInboundExample) {
    const validation = validateCustomInboundJson(json);
    return `<div class="custom-json-editor">
      <div class="custom-json-toolbar"><div><span class="custom-json-language">JSON</span><span>单个入站对象</span></div><div class="inline-actions"><button class="editor-action" type="button" data-action="reset-custom-json">示例</button><button class="editor-action" type="button" data-action="format-custom-json">${icon("sparkles")}格式化</button></div></div>
      <textarea class="custom-json-textarea" name="inbound-json" data-custom-inbound-json aria-describedby="custom-json-validation" aria-invalid="${!validation.valid}" spellcheck="false">${escapeHtml(json)}</textarea>
    </div>
    ${customValidationMarkup(validation)}
    <div class="info-callout custom-json-help">${icon("info")}<span>请输入一个完整的 sing-box 入站对象，而不是整个配置文件或 inbounds 数组。原型会检查 JSON 语法和必需的 <code>type</code> 字段。</span></div>`;
  }

  function renderInboundTypeFields(type, customJson = customInboundExample) {
    const booleanOptions = [["true", "启用"], ["false", "关闭"]];
    if (type === "custom") return renderCustomInboundEditor(customJson);
    if (type === "tun") {
      return `<div class="tun-settings">
        <div class="tun-toggle-grid">
          ${inboundToggle("启用", "允许此入站监听并接管流量", "enabled", true)}
          ${inboundToggle("自动设置全局路由", "由 sing-box 自动写入系统路由", "auto_route", true)}
          ${inboundToggle("严格路由", "减少流量泄漏并强化路由约束", "strict_route", false)}
          ${inboundToggle("独立于端点的 NAT", "为 UDP 使用 endpoint-independent NAT", "endpoint_independent_nat", false)}
        </div>
        <div class="form-grid tun-basic-grid">
          ${inboundInput("TUN 网卡名称", "interface_name", "", { hint: "无", placeholder: "留空由系统自动命名" })}
          ${inboundInput("最大传输单元", "mtu", "9000", { type: "number", hint: "MTU" })}
          ${inboundStackControl("mixed")}
          ${tunListField("IPv4 和 IPv6 前缀", "address", ["11.11.11.1/30"], "例如 fd00::1/126", "address")}
          ${tunListField("自定义路由", "route_address", [], "例如 0.0.0.0/1 或 ::/1", "route_address")}
          ${tunListField("排除自定义路由", "route_exclude_address", ["172.100.0.0/16", "192.168.0.0/16", "172.17.0.0/16", "172.22.0.0/16"], "例如 fc00::/7", "route_exclude_address")}
        </div>
      </div><div class="info-callout inbound-type-note">${icon("shield")}<span>TUN 地址与路由字段支持多个 IPv4/IPv6 CIDR；点击加号添加，列表中的值可直接编辑或删除。</span></div>`;
    }
    if (type === "socks") {
      return `<div class="form-grid">
        ${inboundInput("监听地址", "listen", "127.0.0.1", { required: true })}
        ${inboundInput("监听端口", "listen_port", "1080", { type: "number", required: true })}
        ${inboundInput("用户名", "username", "", { hint: "可选" })}
        ${inboundInput("密码", "password", "", { type: "password", hint: "可选" })}
        ${inboundInput("UDP 超时", "udp_timeout", "5m", { hint: "例如 5m" })}
        ${inboundSelect("协议嗅探", "sniff", "true", booleanOptions)}
      </div><div class="info-callout inbound-type-note">${icon("info")}<span>SOCKS 入站支持 SOCKS4、SOCKS4a 与 SOCKS5；填写用户名后会生成 users 认证配置。</span></div>`;
    }
    if (type === "http") {
      return `<div class="form-grid">
        ${inboundInput("监听地址", "listen", "127.0.0.1", { required: true })}
        ${inboundInput("监听端口", "listen_port", "8080", { type: "number", required: true })}
        ${inboundInput("用户名", "username", "", { hint: "可选" })}
        ${inboundInput("密码", "password", "", { type: "password", hint: "可选" })}
        ${inboundSelect("协议嗅探", "sniff", "true", booleanOptions)}
        ${inboundSelect("覆盖目标地址", "sniff_override_destination", "false", booleanOptions)}
      </div><div class="info-callout inbound-type-note">${icon("globe")}<span>HTTP 入站接收标准 HTTP 代理与 CONNECT 请求，可选择启用用户认证。</span></div>`;
    }
    if (type === "direct") {
      return `<div class="form-grid">
        ${inboundInput("监听地址", "listen", "127.0.0.1", { required: true })}
        ${inboundInput("监听端口", "listen_port", "5353", { type: "number", required: true })}
        ${inboundSelect("网络协议", "network", "udp", [["", "TCP + UDP"], ["tcp", "TCP"], ["udp", "UDP"]])}
        ${inboundInput("覆盖目标地址", "override_address", "", { hint: "可选" })}
        ${inboundInput("覆盖目标端口", "override_port", "", { type: "number", hint: "可选" })}
        ${inboundSelect("协议嗅探", "sniff", "false", booleanOptions)}
      </div><div class="info-callout inbound-type-note">${icon("route")}<span>Direct 入站适合透明转发或 DNS 劫持，可按需覆盖原始目标地址和端口。</span></div>`;
    }
    return `<div class="form-grid">
      ${inboundInput("监听地址", "listen", "127.0.0.1", { required: true })}
      ${inboundInput("监听端口", "listen_port", "7891", { type: "number", required: true })}
      ${inboundInput("用户名", "username", "", { hint: "可选" })}
      ${inboundInput("密码", "password", "", { type: "password", hint: "可选" })}
      ${inboundSelect("协议嗅探", "sniff", "true", booleanOptions)}
      ${inboundSelect("覆盖目标地址", "sniff_override_destination", "false", booleanOptions)}
    </div><div class="info-callout inbound-type-note">${icon("inbound")}<span>Mixed 入站在同一端口同时提供 HTTP 与 SOCKS 代理，适合本机通用代理入口。</span></div>`;
  }

  function inboundModal(itemId) {
    const item = state.customInbounds.find((inbound) => inbound.id === itemId);
    const json = item?.json || customInboundExample;
    const selectedType = item ? "custom" : "mixed";
    const validation = selectedType === "custom" ? validateCustomInboundJson(json) : { valid: true };
    const body = `<form class="modal-body" id="inbound-form" data-form="inbound">
      <div class="form-grid inbound-base-fields">
        <label class="form-field ${selectedType === "custom" ? "is-hidden" : ""}" data-inbound-name-field><span class="field-label">入站名称<small>对应 tag</small></span><input class="field-input" name="tag" value="${escapeHtml(item?.config.tag || "Mixed Proxy 2")}" required ${selectedType === "custom" ? "disabled" : ""} /></label>
        ${inboundSelect("入站类型", "inbound-type", selectedType, [["mixed", "mixed"], ["tun", "tun"], ["socks", "socks"], ["http", "http"], ["direct", "direct"], ["custom", "自定义"]], { dataAttribute: "data-inbound-type", full: selectedType === "custom" })}
      </div>
      <div class="inbound-type-divider"><span>类型专属配置</span></div>
      <div class="inbound-type-fields" data-inbound-type-fields>${renderInboundTypeFields(selectedType, json)}</div>
    </form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" id="inbound-submit" type="submit" form="inbound-form" ${validation.valid ? "" : "disabled"}>${icon(item ? "save" : "check")}${item ? "保存入站" : "确认添加"}</button></div>`;
    return modalFrame(item ? "编辑入站" : "新增入站", "不同入站类型使用各自对应的配置参数", body, footer, "modal-wide");
  }

  function updateCustomInboundValidation(textarea) {
    const result = validateCustomInboundJson(textarea.value);
    textarea.classList.toggle("invalid", !result.valid);
    textarea.setAttribute("aria-invalid", String(!result.valid));
    const feedback = document.querySelector("#custom-json-validation");
    if (feedback) feedback.outerHTML = customValidationMarkup(result);
    const submit = document.querySelector("#inbound-submit");
    if (submit) submit.disabled = !result.valid;
    return result;
  }

  function buildInboundConfig(form, type) {
    const data = new FormData(form);
    const read = (name) => String(data.get(name) || "").trim();
    const asNumber = (name) => {
      const value = read(name);
      return value === "" ? null : Number(value);
    };
    const asBoolean = (name) => read(name) === "true";
    const readList = (name) => data.getAll(name).map((value) => String(value).trim()).filter(Boolean);
    const config = { type, tag: read("tag") };

    if (type === "tun") {
      const interfaceName = read("interface_name");
      const mtu = asNumber("mtu");
      const routeAddress = readList("route_address");
      const routeExcludeAddress = readList("route_exclude_address");
      if (interfaceName) config.interface_name = interfaceName;
      config.address = readList("address");
      if (mtu !== null) config.mtu = mtu;
      config.stack = read("stack");
      config.auto_route = asBoolean("auto_route");
      config.strict_route = asBoolean("strict_route");
      config.endpoint_independent_nat = asBoolean("endpoint_independent_nat");
      if (routeAddress.length) config.route_address = routeAddress;
      if (routeExcludeAddress.length) config.route_exclude_address = routeExcludeAddress;
      return config;
    }

    config.listen = read("listen");
    config.listen_port = asNumber("listen_port");
    if (type === "direct") {
      const network = read("network");
      const overrideAddress = read("override_address");
      const overridePort = asNumber("override_port");
      if (network) config.network = network;
      if (overrideAddress) config.override_address = overrideAddress;
      if (overridePort !== null) config.override_port = overridePort;
      config.sniff = asBoolean("sniff");
      return config;
    }

    const username = read("username");
    const password = read("password");
    if (username || password) config.users = [{ username, password }];
    if (type === "socks") config.udp_timeout = read("udp_timeout");
    config.sniff = asBoolean("sniff");
    if (type !== "socks") config.sniff_override_destination = asBoolean("sniff_override_destination");
    return config;
  }

  function outboundTypeControl(value) {
    const options = [
      ["direct", "直连", "lightning"],
      ["block", "阻断", "shield"],
      ["selector", "手动选择", "globe"],
      ["urltest", "自动选择", "gauge"],
    ];
    return `<div class="outbound-type-control">${options.map(([type, label, iconName]) => `<label><input type="radio" name="outbound-type" value="${type}" data-outbound-type ${type === value ? "checked" : ""} /><span>${icon(iconName)}${label}</span></label>`).join("")}</div>`;
  }

  function subscriptionNodeMembers(subscription) {
    const regions = ["香港 01", "香港 02", "日本 01", "新加坡 01", "美国 01", "德国 01", "台湾 01", "韩国 01"];
    return regions.slice(0, Math.min(regions.length, subscription.nodes)).map((region, index) => ({
      id: `${subscription.id}-node-${index + 1}`,
      tag: `${subscription.name} · ${region}`,
      type: subscription.id,
      badge: region.slice(0, 2),
    }));
  }

  function outboundReferenceGroups(outbound) {
    const builtIn = [
      { id: "direct", tag: "direct", type: "Built-In", badge: "🎯" },
      { id: "block", tag: "block", type: "Built-In", badge: "🛑" },
      ...state.outbounds
        .filter((item) => item.id !== outbound.id)
        .map((item) => ({ id: item.id, tag: item.tag, type: item.type, badge: outboundMemberInitial(item.tag) })),
    ];
    const subscriptionRefs = state.subscriptions.map((subscription) => ({
      id: subscription.id,
      tag: subscription.name,
      type: "Subscription",
      badge: subscription.logo,
    }));
    return [
      { id: "built-in", name: "内置", total: builtIn.length, members: builtIn, expanded: true },
      { id: "subscriptions", name: "订阅", total: subscriptionRefs.length, members: subscriptionRefs, expanded: true },
      ...state.subscriptions.map((subscription) => ({
        id: subscription.id,
        name: subscription.name,
        total: subscription.nodes,
        members: subscriptionNodeMembers(subscription),
        expanded: false,
      })),
    ];
  }

  function outboundReferenceTile(groupId, member, selectedKeys) {
    const key = `${groupId}:${member.id}`;
    const reference = JSON.stringify({ groupId, id: member.id, tag: member.tag, type: member.type });
    return `<label class="outbound-reference-tile">
      <input type="checkbox" name="outbound-ref" value="${escapeHtml(reference)}" ${selectedKeys.has(key) ? "checked" : ""} />
      <span class="outbound-reference-content"><span class="outbound-reference-badge">${escapeHtml(member.badge || outboundMemberInitial(member.tag))}</span><span><strong>${escapeHtml(member.tag)}</strong><small>${escapeHtml(member.type)}</small></span>${icon("check", "outbound-reference-check")}</span>
    </label>`;
  }

  function outboundReferenceGroup(group, selectedKeys) {
    const selectedCount = group.members.filter((member) => selectedKeys.has(`${group.id}:${member.id}`)).length;
    return `<section class="outbound-reference-group ${group.expanded ? "expanded" : ""}" data-outbound-reference-group>
      <button class="outbound-reference-head" type="button" data-action="toggle-outbound-group" aria-expanded="${group.expanded}">
        <span>${escapeHtml(group.name)}</span>${selectedCount ? `<span class="outbound-selected-count">已选 ${selectedCount}</span>` : ""}<strong>${group.total}</strong>${icon("chevron")}
      </button>
      <div class="outbound-reference-body" ${group.expanded ? "" : "hidden"}>
        ${group.members.length ? `<div class="outbound-reference-grid">${group.members.map((member) => outboundReferenceTile(group.id, member, selectedKeys)).join("")}</div>` : `<div class="outbound-reference-empty">该分组暂无可引用项目</div>`}
        ${group.total > group.members.length ? `<p class="outbound-reference-note">已展示 ${group.members.length} 个示例节点；引用整个订阅可自动包含全部 ${group.total} 个节点。</p>` : ""}
      </div>
    </section>`;
  }

  function outboundModal(itemId) {
    const existing = state.outbounds.find((outbound) => outbound.id === itemId);
    const outbound = existing || {
      id: "",
      tag: "🚀 节点选择",
      type: "selector",
      hidden: false,
      include: "",
      exclude: "",
      icon: "",
      interrupt_exist_connections: true,
      url: "https://www.gstatic.com/generate_204",
      interval: "3m",
      tolerance: 150,
      refs: [],
    };
    const isPolicy = ["selector", "urltest"].includes(outbound.type);
    const selectedKeys = new Set((outbound.refs || []).map((reference) => `${reference.groupId}:${reference.id}`));
    const referenceGroups = outboundReferenceGroups(outbound);
    const body = `<form class="modal-body outbound-editor-body" id="outbound-form" data-form="outbound">
      <div class="outbound-editor-fields">
        <label class="outbound-editor-row"><span class="outbound-editor-label">名称<small>tag</small></span><input class="field-input" name="tag" value="${escapeHtml(outbound.tag)}" placeholder="例如 🚀 节点选择" required /></label>
        <div class="outbound-editor-row"><span class="outbound-editor-label">类型<small>type</small></span>${outboundTypeControl(outbound.type)}</div>
        <div data-outbound-policy-fields ${isPolicy ? "" : "hidden"}>
          <div class="outbound-editor-row"><span class="outbound-editor-label">隐藏<small>仅控制面板</small></span><div class="outbound-toggle-control"><span>不在代理组快捷选择中显示</span><label class="form-switch"><input type="checkbox" name="hidden" value="true" ${outbound.hidden ? "checked" : ""} /><span class="switch"></span></label></div></div>
          <label class="outbound-editor-row"><span class="outbound-editor-label">包含<small>include</small></span><input class="field-input" name="include" value="${escapeHtml(outbound.include || "")}" placeholder="keywords1|keywords2" /></label>
          <label class="outbound-editor-row"><span class="outbound-editor-label">排除<small>exclude</small></span><input class="field-input" name="exclude" value="${escapeHtml(outbound.exclude || "")}" placeholder="keywords1|keywords2" /></label>
          <label class="outbound-editor-row"><span class="outbound-editor-label">图标<small>icon</small></span><span class="outbound-icon-control"><span class="outbound-icon-preview ${outbound.icon ? "" : "is-empty"}" data-outbound-icon-preview>${outbound.icon ? `<img src="${escapeHtml(outbound.icon)}" alt="" />` : icon("globe")}</span><input class="field-input" name="icon" value="${escapeHtml(outbound.icon || "")}" placeholder="https://" data-outbound-icon-input /></span></label>
          <div class="outbound-urltest-fields" data-outbound-urltest-fields ${outbound.type === "urltest" ? "" : "hidden"}>
            <label class="outbound-editor-row"><span class="outbound-editor-label">测延迟链接<small>url</small></span><input class="field-input" name="url" value="${escapeHtml(outbound.url || "")}" placeholder="https://www.gstatic.com/generate_204" /></label>
            <label class="outbound-editor-row"><span class="outbound-editor-label">测试间隔<small>interval</small></span><input class="field-input" name="interval" value="${escapeHtml(outbound.interval || "3m")}" placeholder="3m" /></label>
            <label class="outbound-editor-row"><span class="outbound-editor-label">测试容差<small>tolerance</small></span><span class="input-suffix-wrap"><input class="field-input" name="tolerance" type="number" min="0" value="${escapeHtml(outbound.tolerance ?? 150)}" /><span class="input-suffix">ms</span></span></label>
          </div>
          <div class="outbound-reference-divider"><span>引用出站 &amp; 引用订阅</span><strong data-outbound-reference-total>已选 ${selectedKeys.size}</strong></div>
          <div class="outbound-reference-list">${referenceGroups.map((group) => outboundReferenceGroup(group, selectedKeys)).join("")}</div>
        </div>
        <div class="outbound-no-options" data-outbound-no-options ${isPolicy ? "hidden" : ""}>${icon(outbound.type === "block" ? "shield" : "lightning")}<strong data-outbound-empty-title>${outbound.type === "block" ? "阻断出站无需额外参数" : "直连出站无需额外参数"}</strong><span>保存后会生成只包含 <code>type</code> 与 <code>tag</code> 的 sing-box 出站对象。</span></div>
      </div>
    </form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" id="outbound-submit" type="submit" form="outbound-form">${icon(existing ? "save" : "check")}${existing ? "保存" : "确认添加"}</button></div>`;
    return modalFrame("出站", existing ? "编辑出站类型、筛选条件及引用关系" : "创建内置出站或节点选择策略组", body, footer, "modal-outbound");
  }

  function updateOutboundTypeFields(form, type) {
    const isPolicy = ["selector", "urltest"].includes(type);
    const policyFields = form.querySelector("[data-outbound-policy-fields]");
    const urltestFields = form.querySelector("[data-outbound-urltest-fields]");
    const emptyState = form.querySelector("[data-outbound-no-options]");
    if (policyFields) policyFields.hidden = !isPolicy;
    if (urltestFields) urltestFields.hidden = type !== "urltest";
    if (emptyState) emptyState.hidden = isPolicy;
    const emptyTitle = form.querySelector("[data-outbound-empty-title]");
    if (emptyTitle) emptyTitle.textContent = type === "block" ? "阻断出站无需额外参数" : "直连出站无需额外参数";
  }

  function updateOutboundReferenceCount(form) {
    const selected = form.querySelectorAll('input[name="outbound-ref"]:checked').length;
    const total = form.querySelector("[data-outbound-reference-total]");
    if (total) total.textContent = `已选 ${selected}`;
    form.querySelectorAll("[data-outbound-reference-group]").forEach((group) => {
      const count = group.querySelectorAll('input[name="outbound-ref"]:checked').length;
      let badge = group.querySelector(".outbound-selected-count");
      if (count && !badge) {
        badge = document.createElement("span");
        badge.className = "outbound-selected-count";
        group.querySelector(".outbound-reference-head span")?.after(badge);
      }
      if (badge) {
        badge.textContent = count ? `已选 ${count}` : "";
        badge.hidden = !count;
      }
    });
  }

  function buildOutboundConfig(form, itemId = null) {
    const data = new FormData(form);
    const type = String(data.get("outbound-type") || "selector");
    const config = {
      id: itemId || `outbound-${Date.now()}`,
      tag: String(data.get("tag") || "新出站").trim(),
      type,
      refs: [],
    };
    if (!["selector", "urltest"].includes(type)) return config;
    config.hidden = data.get("hidden") === "true";
    config.include = String(data.get("include") || "").trim();
    config.exclude = String(data.get("exclude") || "").trim();
    config.icon = String(data.get("icon") || "").trim();
    config.interrupt_exist_connections = true;
    config.refs = data.getAll("outbound-ref").flatMap((value) => {
      try {
        return [JSON.parse(String(value))];
      } catch {
        return [];
      }
    });
    if (type === "urltest") {
      config.url = String(data.get("url") || "").trim();
      config.interval = String(data.get("interval") || "3m").trim();
      config.tolerance = Number(data.get("tolerance") || 0);
    }
    return config;
  }

  function copyText(text) {
    if (navigator.clipboard?.writeText) return navigator.clipboard.writeText(text).catch(() => copyTextFallback(text));
    copyTextFallback(text);
    return Promise.resolve();
  }

  function copyTextFallback(text) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.cssText = "position:fixed;left:-9999px;top:0";
    document.body.append(textarea);
    textarea.select();
    document.execCommand("copy");
    textarea.remove();
  }

  function subscriptionModal() {
    const body = `<form class="modal-body" id="subscription-form" data-form="subscription"><div class="form-grid">
      ${modalField("订阅名称", "name", "我的节点订阅")}${modalField("订阅地址", "url", "https://example.com/subscribe", "url", true)}${modalField("更新频率", "interval", "每 6 小时", "select", false, ["每 1 小时", "每 6 小时", "每天", "手动更新"])}${modalField("用户代理", "userAgent", "sing-box")}
      <div class="form-field full"><div class="info-callout">${icon("info")}<span>原型不会请求或保存真实订阅地址，提交后只会生成一张演示卡片。</span></div></div>
    </div></form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" type="submit" form="subscription-form">${icon("plus")}添加订阅</button></div>`;
    return modalFrame("添加订阅", "导入远程节点与流量信息", body, footer);
  }

  function genericModal(title, description, formType, fields) {
    const body = `<form class="modal-body" id="generic-form" data-form="${formType}"><div class="form-grid">${fields.map(([label, nameValue, type = "text", items = []], index) => modalField(label, `field-${index}`, nameValue, type, index === 2, items)).join("")}</div></form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" type="submit" form="generic-form">${icon("check")}确认添加</button></div>`;
    return modalFrame(title, description, body, footer);
  }

  function modalField(label, name, value, type = "text", full = false, items = []) {
    if (type === "select") return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}</span><select class="field-select" name="${name}">${items.map((item) => `<option>${item}</option>`).join("")}</select></label>`;
    return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}</span><input class="field-input" name="${name}" type="${type}" value="${value}" ${name === "name" ? "required" : ""} /></label>`;
  }

  function jsonModal() {
    const body = `<div class="modal-body">${renderCodePreview(true)}<div class="info-callout" style="margin-top:12px">${icon("check")}<span>配置结构检查通过，共包含 ${inboundCount()} 个入站、12 个出站与 16 条路由规则。</span></div></div>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="copy-json">${icon("copy")}复制 JSON</button><button class="btn btn-primary" data-action="close-modal">完成</button></div>`;
    return modalFrame("配置 JSON 预览", "由当前表单实时生成，只读展示", body, footer);
  }

  function commandModal() {
    const body = `<div class="modal-body"><label class="search-wrap" style="display:block"><span class="icon">${icon("search")}</span><input class="search-input" style="height:43px" placeholder="输入页面名称..." autofocus /></label><div class="entry-list" style="padding:12px 0 0">${[
      ["dashboard", "仪表盘", "dashboard"], ["config", "通用配置", "general"], ["config", "入站配置", "inbounds"], ["config", "出站配置", "outbounds"], ["config", "路由设置", "routing"], ["config", "DNS 设置", "dns"], ["subscriptions", "订阅配置", "subscription"], ["rulesets", "规则集", "rules"], ["settings", "系统设置", "settings"]
    ].map(([page, label, iconName]) => `<button class="entry-card" data-command-page="${page}" data-command-config="${page === "config" ? iconName : ""}" style="width:100%;text-align:left"><div class="entry-icon">${icon(iconName)}</div><div class="entry-copy"><div class="entry-title">${label}</div><div class="entry-meta">快速跳转</div></div>${icon("chevron", "faint")}</button>`).join("")}</div></div>`;
    return modalFrame("快速跳转", "选择要打开的控制台页面", body);
  }

  function closeModal() {
    modalRoot.innerHTML = "";
    state.editingCustomInboundId = null;
    state.editingOutboundId = null;
    document.body.style.overflow = "";
  }

  function showToast(title, message, type = "success") {
    const toast = document.createElement("div");
    toast.className = "toast";
    toast.innerHTML = `<div class="toast-icon ${type === "info" ? "info" : ""}">${icon(type === "info" ? "info" : "check")}</div><div class="toast-copy"><div class="toast-title">${escapeHtml(title)}</div><div class="toast-message">${escapeHtml(message)}</div></div>`;
    toastStack.append(toast);
    window.setTimeout(() => {
      toast.classList.add("leaving");
      window.setTimeout(() => toast.remove(), 190);
    }, 3200);
  }

  function navigate(page, configId = null) {
    state.page = page;
    if (page === "config") {
      state.configExpanded = true;
      if (configId) state.activeConfig = configId;
    }
    state.sidebarOpen = false;
    closeModal();
    const route = page === "config" ? `#/${page}/${state.activeConfig}` : `#/${page}`;
    window.history.pushState({ page, configId: state.activeConfig }, "", route);
    render();
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function setAccent(accent) {
    state.accent = accent;
    const palettes = {
      blue: ["#4f6ff8", "#3758e7", "#edf1ff"],
      cyan: ["#13aebf", "#0b8f9e", "#e7f9fb"],
      violet: ["#8067e8", "#684bd8", "#f0edff"],
    };
    const [primary, strong, soft] = palettes[accent];
    const darkSoft = { blue: "#253054", cyan: "#163b42", violet: "#30284d" }[accent];
    document.documentElement.style.setProperty("--primary", primary);
    document.documentElement.style.setProperty("--primary-strong", strong);
    document.documentElement.style.setProperty("--primary-soft", state.theme === "dark" ? darkSoft : soft);
    writeStorage("singbox-accent", accent);
  }

  modalRoot.addEventListener("click", (event) => {
    const outboundGroupButton = event.target.closest('[data-action="toggle-outbound-group"]');
    if (outboundGroupButton) {
      event.stopPropagation();
      const group = outboundGroupButton.closest("[data-outbound-reference-group]");
      const body = group?.querySelector(".outbound-reference-body");
      const expanded = outboundGroupButton.getAttribute("aria-expanded") === "true";
      outboundGroupButton.setAttribute("aria-expanded", String(!expanded));
      group?.classList.toggle("expanded", !expanded);
      if (body) body.hidden = expanded;
      return;
    }

    const addButton = event.target.closest('[data-action="add-tun-list-item"]');
    if (addButton) {
      event.stopPropagation();
      const field = addButton.closest("[data-tun-list-field]");
      const input = field?.querySelector("[data-tun-list-input]");
      const value = input?.value.trim();
      if (!field || !input || !value) {
        input?.focus();
        return;
      }
      field.querySelector("[data-tun-list-items]")?.insertAdjacentHTML("beforeend", tunListItem(field.dataset.listName, value));
      input.value = "";
      input.focus();
      return;
    }

    const removeButton = event.target.closest('[data-action="remove-tun-list-item"]');
    if (removeButton) {
      event.stopPropagation();
      removeButton.closest(".tun-list-item")?.remove();
    }
  });

  document.addEventListener("click", (event) => {
    const command = event.target.closest("[data-command-page]");
    if (command) {
      navigate(command.dataset.commandPage, command.dataset.commandConfig || null);
      return;
    }

    const configTarget = event.target.closest("[data-config]");
    if (configTarget) {
      navigate("config", configTarget.dataset.config);
      return;
    }

    const pageTarget = event.target.closest("[data-page]");
    if (pageTarget) {
      navigate(pageTarget.dataset.page);
      return;
    }

    const modalTarget = event.target.closest("[data-modal]");
    if (modalTarget) {
      openModal(modalTarget.dataset.modal, modalTarget.dataset.id || null);
      return;
    }

    const accentTarget = event.target.closest("[data-accent]");
    if (accentTarget) {
      setAccent(accentTarget.dataset.accent);
      render();
      showToast("主题色已更新", "新的强调色已应用到控制台。", "info");
      return;
    }

    const settingsTarget = event.target.closest("[data-settings-section]");
    if (settingsTarget) {
      state.settingsSection = settingsTarget.dataset.settingsSection;
      document.querySelectorAll(".settings-nav-item").forEach((item) => item.classList.toggle("active", item === settingsTarget));
      document.getElementById(state.settingsSection)?.scrollIntoView({ behavior: "smooth", block: "start" });
      return;
    }

    const modeTarget = event.target.closest("[data-mode]");
    if (modeTarget) {
      state.routeMode = modeTarget.dataset.mode;
      render();
      showToast("路由模式已切换", `当前使用${{ rule: "规则", global: "全局", direct: "直连" }[state.routeMode]}模式。`);
      return;
    }

    const toggleTarget = event.target.closest("[data-toggle]");
    if (toggleTarget) {
      const key = toggleTarget.dataset.toggle;
      state[key] = !state[key];
      if (state.page === "config") state.dirty = true;
      render();
      showToast("设置已更新", "该更改已在原型中即时生效。", "info");
      return;
    }

    const actionTarget = event.target.closest("[data-action]");
    if (!actionTarget) return;
    const action = actionTarget.dataset.action;

    if (action === "toggle-sidebar") {
      state.sidebarOpen = !state.sidebarOpen;
      document.body.classList.toggle("sidebar-open", state.sidebarOpen);
    } else if (action === "close-sidebar") {
      state.sidebarOpen = false;
      document.body.classList.remove("sidebar-open");
    } else if (action === "config-parent") {
      if (state.page !== "config") navigate("config", state.activeConfig);
      else {
        state.configExpanded = !state.configExpanded;
        render();
      }
    } else if (action === "toggle-theme") {
      state.theme = state.theme === "dark" ? "light" : "dark";
      writeStorage("singbox-theme", state.theme);
      setAccent(state.accent);
      render();
      showToast("外观已切换", `已使用${state.theme === "dark" ? "深色" : "浅色"}主题。`, "info");
    } else if (action === "toggle-core") {
      state.coreRunning = !state.coreRunning;
      render();
      showToast(state.coreRunning ? "核心已启动" : "核心已停止", state.coreRunning ? "sing-box 服务恢复运行。" : "所有代理入口已暂停。", state.coreRunning ? "success" : "info");
    } else if (action === "restart-core") {
      state.coreRunning = false;
      render();
      showToast("正在重启核心", "正在验证配置并重新加载服务...", "info");
      window.setTimeout(() => {
        state.coreRunning = true;
        render();
        showToast("核心重启完成", "sing-box 已使用自定义配置 A 恢复运行。");
      }, 900);
    } else if (action === "refresh-dashboard") {
      showToast("状态已刷新", "流量、连接与核心状态均已更新。", "info");
    } else if (action === "save-config") {
      state.dirty = false;
      state.lastSaved = new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
      render();
      showToast("配置已保存并应用", "语法检查通过，核心已无感重载。", "success");
    } else if (action === "save-settings") {
      showToast("设置已保存", "系统偏好已写入本地配置。", "success");
    } else if (action === "notifications") {
      showToast("暂无待处理通知", "核心与全部订阅资源运行正常。", "info");
    } else if (action === "close-modal") {
      closeModal();
    } else if (action === "modal-backdrop" && event.target === actionTarget) {
      closeModal();
    } else if (action === "copy-json") {
      const text = '{"log":{"level":"info"},"route":{"final":"节点选择"}}';
      copyText(text);
      showToast("已复制", "配置 JSON 已复制到剪贴板。", "success");
    } else if (action === "format-custom-json") {
      const textarea = document.querySelector("[data-custom-inbound-json]");
      if (!textarea) return;
      const result = updateCustomInboundValidation(textarea);
      if (!result.valid) {
        showToast("暂时无法格式化", "请先修正 JSON 语法错误。", "info");
        return;
      }
      textarea.value = JSON.stringify(result.config, null, 2);
      updateCustomInboundValidation(textarea);
      textarea.focus();
    } else if (action === "reset-custom-json") {
      const textarea = document.querySelector("[data-custom-inbound-json]");
      if (!textarea) return;
      textarea.value = customInboundExample;
      updateCustomInboundValidation(textarea);
      textarea.focus();
    } else if (action === "copy-custom-inbound") {
      const item = state.customInbounds.find((inbound) => inbound.id === actionTarget.dataset.id);
      if (!item) return;
      copyText(item.json);
      showToast("已复制自定义入站", "完整入站 JSON 已复制到剪贴板。", "success");
    } else if (action === "remove-custom-inbound") {
      const item = state.customInbounds.find((inbound) => inbound.id === actionTarget.dataset.id);
      state.customInbounds = state.customInbounds.filter((inbound) => inbound.id !== actionTarget.dataset.id);
      state.dirty = true;
      render();
      showToast("已移除自定义入站", `${item?.config.tag || "自定义入站"} 已从配置草稿移除。`, "info");
    } else if (action === "remove-form-inbound") {
      const item = state.formInbounds.find((inbound) => inbound.id === actionTarget.dataset.id);
      state.formInbounds = state.formInbounds.filter((inbound) => inbound.id !== actionTarget.dataset.id);
      state.dirty = true;
      render();
      showToast("已移除入站", `${item?.config.tag || "入站配置"} 已从配置草稿移除。`, "info");
    } else if (action === "sync-subscription") {
      const item = state.subscriptions.find((sub) => sub.id === actionTarget.dataset.id);
      if (item) item.updated = "刚刚";
      render();
      showToast("订阅同步完成", `${item?.name || "订阅"} 已更新，节点列表没有冲突。`);
    } else if (action === "sync-all-subscriptions") {
      state.subscriptions.forEach((item) => (item.updated = "刚刚"));
      render();
      showToast("全部订阅已同步", `已更新 ${state.subscriptions.length} 个订阅和 ${state.subscriptions.reduce((sum, item) => sum + item.nodes, 0)} 个节点。`);
    } else if (action === "update-rulesets") {
      state.rulesets.forEach((item) => (item.updated = "刚刚"));
      render();
      showToast("规则集更新完成", "5 个规则集均已更新并重新加载。", "success");
    } else if (action === "refresh-ruleset") {
      const item = state.rulesets.find((ruleset) => ruleset.id === Number(actionTarget.dataset.id));
      if (item) item.updated = "刚刚";
      render();
      showToast("规则集已更新", `${item?.name || "规则集"} 已加载最新版本。`);
    } else if (action === "clear-ruleset-filter") {
      state.rulesetSearch = "";
      state.rulesetType = "all";
      render();
    } else if (action === "check-update") {
      showToast("已是最新版本", "当前 sing-box Stable 1.11.11 无需更新。", "info");
    } else if (action === "change-password") {
      showToast("原型操作", "正式接入后此处将打开密码修改流程。", "info");
    } else if (action === "project-info") {
      showToast("SingBox Control", "当前为交互原型，后续可对接 sing-box 与 Clash API。", "info");
    } else if (["edit-entry", "duplicate-entry", "remove-entry", "edit-subscription", "subscription-more", "view-connections", "ruleset-filter"].includes(action)) {
      const messages = {
        "edit-entry": "编辑面板将在正式版本中读取完整配置字段。",
        "duplicate-entry": "已复制一个配置项草稿。",
        "remove-entry": "原型不会删除现有配置项。",
        "edit-subscription": "订阅编辑流程已预留。",
        "subscription-more": "更多操作包括复制、禁用与删除。",
        "view-connections": "连接详情页已预留，可接入 Clash API。",
        "ruleset-filter": "筛选条件已应用。",
      };
      showToast("原型交互", messages[action], "info");
    }
  });

  document.addEventListener("input", (event) => {
    if (event.target.matches("[data-custom-inbound-json]")) {
      updateCustomInboundValidation(event.target);
    }
    if (event.target.matches("[data-config-field], [data-settings-input]")) {
      state.dirty = true;
      const button = document.querySelector("#save-config-button");
      if (button) button.innerHTML = `${icon("save")}保存更改`;
    }
    if (event.target.matches("[data-ruleset-search]")) {
      state.rulesetSearch = event.target.value;
      window.clearTimeout(window.__singboxSearchTimer);
      window.__singboxSearchTimer = window.setTimeout(render, 120);
    }
    if (event.target.matches("[data-outbound-icon-input]")) {
      const preview = document.querySelector("[data-outbound-icon-preview]");
      if (!preview) return;
      const value = event.target.value.trim();
      preview.classList.toggle("is-empty", !value);
      preview.innerHTML = value ? `<img src="${escapeHtml(value)}" alt="" />` : icon("globe");
    }
  });

  document.addEventListener("change", (event) => {
    if (event.target.matches("[data-outbound-type]")) {
      const form = event.target.closest("#outbound-form");
      if (form) updateOutboundTypeFields(form, event.target.value);
      return;
    }
    if (event.target.matches('input[name="outbound-ref"]')) {
      const form = event.target.closest("#outbound-form");
      if (form) updateOutboundReferenceCount(form);
      return;
    }
    if (event.target.matches("[data-inbound-type]")) {
      const selectedType = event.target.value;
      const fields = document.querySelector("[data-inbound-type-fields]");
      const currentEditor = fields?.querySelector("[data-custom-inbound-json]");
      if (currentEditor) fields.dataset.customJson = currentEditor.value;
      if (fields) fields.innerHTML = renderInboundTypeFields(selectedType, fields.dataset.customJson || customInboundExample);

      const nameField = document.querySelector("[data-inbound-name-field]");
      const nameInput = nameField?.querySelector("input");
      const typeField = event.target.closest(".form-field");
      const isCustom = selectedType === "custom";
      nameField?.classList.toggle("is-hidden", isCustom);
      typeField?.classList.toggle("full", isCustom);
      if (nameInput) {
        nameInput.disabled = isCustom;
        nameInput.required = !isCustom;
      }
      const submit = document.querySelector("#inbound-submit");
      if (submit) submit.disabled = false;
      const editor = fields?.querySelector("[data-custom-inbound-json]");
      if (editor) updateCustomInboundValidation(editor);
      return;
    }
    if (event.target.matches("[data-ruleset-type]")) {
      state.rulesetType = event.target.value;
      render();
    }
    if (event.target.matches("[data-theme-select]")) {
      const value = event.target.value;
      state.theme = value === "深色" ? "dark" : value === "跟随系统" && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
      writeStorage("singbox-theme", state.theme);
      setAccent(state.accent);
      render();
    }
  });

  document.addEventListener("submit", (event) => {
    const form = event.target.closest("[data-form]");
    if (!form) return;
    event.preventDefault();
    const type = form.dataset.form;
    if (type === "inbound") {
      const inboundType = form.querySelector("[data-inbound-type]").value;
      if (inboundType === "custom") {
        const textarea = form.querySelector("[data-custom-inbound-json]");
        const result = updateCustomInboundValidation(textarea);
        if (!result.valid) return;
        const json = JSON.stringify(result.config, null, 2);
        const existing = state.customInbounds.find((inbound) => inbound.id === state.editingCustomInboundId);
        if (existing) {
          existing.json = json;
          existing.config = result.config;
        } else {
          state.customInbounds.push({ id: `custom-in-${Date.now()}`, json, config: result.config });
        }
        state.dirty = true;
        closeModal();
        render();
        showToast(existing ? "自定义入站已更新" : "自定义入站已添加", `${result.config.tag || result.config.type} 已加入当前配置草稿，请保存后应用。`, "success");
      } else {
        const config = buildInboundConfig(form, inboundType);
        if (inboundType === "tun" && !config.address.length) {
          showToast("缺少 TUN 地址", "请至少添加一个 IPv4 或 IPv6 前缀。", "info");
          form.querySelector('[data-list-name="address"] [data-tun-list-input]')?.focus();
          return;
        }
        if (state.editingCustomInboundId) {
          state.customInbounds = state.customInbounds.filter((inbound) => inbound.id !== state.editingCustomInboundId);
        }
        const enabled = inboundType !== "tun" || new FormData(form).get("enabled") === "true";
        state.formInbounds.push({ id: `form-in-${Date.now()}`, config, enabled });
        state.dirty = true;
        closeModal();
        render();
        showToast("入站已添加", `${config.tag}（${config.type}）已加入当前配置草稿，请保存后应用。`, "success");
      }
    } else if (type === "outbound") {
      const config = buildOutboundConfig(form, state.editingOutboundId);
      if (["selector", "urltest"].includes(config.type) && !config.refs.length) {
        showToast("缺少引用项目", "手动选择和自动选择至少需要引用一个出站、订阅或订阅节点。", "info");
        form.querySelector('[name="outbound-ref"]')?.focus();
        return;
      }
      const existingIndex = state.outbounds.findIndex((outbound) => outbound.id === state.editingOutboundId);
      if (existingIndex >= 0) state.outbounds.splice(existingIndex, 1, config);
      else state.outbounds.unshift(config);
      state.dirty = true;
      closeModal();
      render();
      showToast(existingIndex >= 0 ? "出站已更新" : "出站已添加", `${config.tag}（${config.type}）已加入当前配置草稿，请保存后应用。`, "success");
    } else if (type === "subscription") {
      const data = new FormData(form);
      state.subscriptions.push({
        id: `sub-${Date.now()}`,
        name: data.get("name") || "新订阅",
        logo: "NS",
        tone: state.subscriptions.length % 2 ? "blue" : "cyan",
        nodes: 24,
        used: 0,
        total: 100,
        expires: "2026-12-31",
        interval: data.get("interval") || "每 6 小时",
        updated: "刚刚",
      });
      closeModal();
      render();
      showToast("订阅已添加", "已生成演示订阅卡片并完成首次同步。", "success");
    } else {
      closeModal();
      state.dirty = true;
      render();
      showToast("配置项已添加", "新项目已加入当前配置草稿，请保存后应用。", "success");
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.target.matches("[data-tun-list-input]") && event.key === "Enter") {
      event.preventDefault();
      event.target.closest("[data-tun-list-field]")?.querySelector('[data-action="add-tun-list-item"]')?.click();
      return;
    }
    if (event.target.matches("[data-custom-inbound-json]") && event.key === "Tab") {
      event.preventDefault();
      const textarea = event.target;
      textarea.setRangeText("  ", textarea.selectionStart, textarea.selectionEnd, "end");
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
      return;
    }
    if (event.target.matches("[data-custom-inbound-json]") && (event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      const submit = document.querySelector("#inbound-submit");
      if (!submit?.disabled) document.querySelector("#inbound-form")?.requestSubmit();
      return;
    }
    if (event.key === "Escape" && modalRoot.innerHTML) closeModal();
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      openModal("command");
    }
  });

  window.addEventListener("popstate", () => {
    const route = window.location.hash.replace(/^#\/?/, "").split("/").filter(Boolean);
    state.page = ["dashboard", "config", "subscriptions", "rulesets", "settings"].includes(route[0]) ? route[0] : "dashboard";
    if (state.page === "config" && configTabs.some((tab) => tab.id === route[1])) state.activeConfig = route[1];
    closeModal();
    render();
  });

  applyTheme();
  setAccent(state.accent);
  render();
})();
