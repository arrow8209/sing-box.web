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

  function readJsonStorage(key, fallback = null) {
    try {
      const value = window.localStorage.getItem(key);
      return value ? JSON.parse(value) : fallback;
    } catch {
      return fallback;
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

  const profileJsonExample = `{
  "log": {
    "level": "info"
  },
  "inbounds": [],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    }
  ],
  "route": {
    "final": "direct",
    "rules": []
  },
  "dns": {
    "servers": [],
    "rules": []
  }
}`;

  const subscriptionJsonExample = `{
  "name": "我的 JSON 订阅",
  "outbounds": [
    {
      "type": "shadowsocks",
      "tag": "香港 01",
      "server": "hk01.example.com",
      "server_port": 8388,
      "method": "2022-blake3-aes-128-gcm",
      "password": "change-me"
    },
    {
      "type": "trojan",
      "tag": "新加坡 01",
      "server": "sg01.example.com",
      "server_port": 443,
      "password": "change-me",
      "tls": {
        "enabled": true,
        "server_name": "sg01.example.com"
      }
    }
  ]
}`;

  function createDemoSubscriptionNodes(prefix, count, offset = 0) {
    const regions = [
      ["香港", "hk"], ["日本", "jp"], ["新加坡", "sg"], ["美国", "us"],
      ["德国", "de"], ["台湾", "tw"], ["韩国", "kr"], ["英国", "uk"],
    ];
    const protocols = [
      ["shadowsocks", 8388], ["trojan", 443], ["vmess", 443], ["vless", 443], ["hysteria2", 8443],
    ];
    return Array.from({ length: count }, (_, index) => {
      const position = index + offset;
      const [region, regionCode] = regions[position % regions.length];
      const [type, serverPort] = protocols[position % protocols.length];
      const sequence = String(Math.floor(position / regions.length) + 1).padStart(2, "0");
      return {
        type,
        tag: `${prefix} · ${region} ${sequence}`,
        server: `${regionCode}${sequence}.example.net`,
        server_port: serverPort,
      };
    });
  }

  const cloudRouteNodes = createDemoSubscriptionNodes("Cloud Route", 42);
  const edgeNetworkNodes = createDemoSubscriptionNodes("Edge Network", 18, 3);

  const manualRulesetExample = `{
  "version": 3,
  "rules": [
    {
      "domain_suffix": [
        "example.com",
        "example.org"
      ]
    },
    {
      "ip_cidr": [
        "10.0.0.0/8",
        "192.168.0.0/16"
      ]
    }
  ]
}`;

  const rulesetUpdateIntervals = [
    ["manual", "仅手动更新"],
    ["15m", "每 15 分钟"],
    ["1h", "每 1 小时"],
    ["6h", "每 6 小时"],
    ["12h", "每 12 小时"],
    ["1d", "每天"],
    ["7d", "每 7 天"],
  ];

  const rulesetHubItems = [
    { id: "geosite-cn", name: "geosite-cn", category: "geosite", count: 124632, source: "SagerNet/sing-geosite", description: "中国大陆常用域名集合" },
    { id: "geosite-private", name: "geosite-private", category: "geosite", count: 1934, source: "SagerNet/sing-geosite", description: "局域网与私有域名集合" },
    { id: "geosite-geolocation-not-cn", name: "geosite-geolocation-!cn", category: "geosite", count: 86319, source: "SagerNet/sing-geosite", description: "中国大陆以外的域名集合" },
    { id: "geosite-category-ads-all", name: "geosite-category-ads-all", category: "geosite", count: 216804, source: "SagerNet/sing-geosite", description: "广告与跟踪域名集合" },
    { id: "geosite-google", name: "geosite-google", category: "geosite", count: 11482, source: "SagerNet/sing-geosite", description: "Google 服务域名集合" },
    { id: "geosite-github", name: "geosite-github", category: "geosite", count: 1246, source: "SagerNet/sing-geosite", description: "GitHub 及相关服务域名" },
    { id: "geoip-cn", name: "geoip-cn", category: "geoip", count: 8926, source: "SagerNet/sing-geoip", description: "中国大陆 IPv4 与 IPv6 地址段" },
    { id: "geoip-private", name: "geoip-private", category: "geoip", count: 32, source: "SagerNet/sing-geoip", description: "私有网络 IP 地址段" },
    { id: "geoip-cloudflare", name: "geoip-cloudflare", category: "geoip", count: 486, source: "SagerNet/sing-geoip", description: "Cloudflare 网络地址段" },
  ];

  const routeRuleTypeOptions = [
    ["inbound", "入站 (inbound)"],
    ["network", "网络 (network)"],
    ["protocol", "协议 (protocol)"],
    ["domain", "域名 (domain)"],
    ["domain_suffix", "域名后缀 (domain_suffix)"],
    ["domain_keyword", "域名关键词 (domain_keyword)"],
    ["domain_regex", "域名正则 (domain_regex)"],
    ["source_ip_cidr", "源 IP 地址段 (source_ip_cidr)"],
    ["ip_cidr", "IP 地址段 (ip_cidr)"],
    ["ip_is_private", "是否为私有 IP (ip_is_private)"],
    ["source_port", "源端口 (source_port)"],
    ["source_port_range", "源端口范围 (source_port_range)"],
    ["port", "端口 (port)"],
    ["port_range", "端口范围 (port_range)"],
    ["process_name", "进程名称 (process_name)"],
    ["process_path", "进程路径 (process_path)"],
    ["process_path_regex", "进程路径正则 (process_path_regex)"],
    ["clash_mode", "Clash 模式 (clash_mode)"],
    ["rule_set", "规则集 (rule_set)"],
    ["inline", "内联 (inline)"],
  ];

  const routeRuleActionOptions = [
    ["route", "路由", "route"],
    ["route-options", "路由设置选项", "config"],
    ["reject", "拒绝连接", "shield"],
    ["hijack-dns", "劫持 DNS 请求", "dns"],
    ["sniff", "协议嗅探", "search"],
    ["resolve", "解析 DNS", "globe"],
  ];

  const initialRoute = window.location.hash.replace(/^#\/?/, "").split("/").filter(Boolean);
  const initialPage = ["dashboard", "config", "subscriptions", "rulesets", "settings"].includes(initialRoute[0])
    ? initialRoute[0]
    : "dashboard";
  const initialConfigTab = configTabs.some((tab) => tab.id === initialRoute[2])
    ? initialRoute[2]
    : configTabs.some((tab) => tab.id === initialRoute[1])
      ? initialRoute[1]
      : "general";

  const state = {
    page: initialPage,
    activeConfig: initialConfigTab,
    activeProfileId: "config-a",
    profiles: [
      {
        id: "config-a",
        name: "自定义配置 A",
        source: "Web 手工配置",
        lastSaved: "10:24",
        version: 12,
        draft: null,
      },
    ],
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
    proxyTesting: false,
    proxyGroups: [
      {
        id: "node-select",
        icon: "🚀",
        name: "节点选择",
        type: "Selector",
        selected: "nas-ss",
        expanded: true,
        members: [
          { id: "nas-ss", name: "nas-ss", type: "Shadowsocks", network: "udp", latency: 206 },
          { id: "vless-60016", name: "vless-60016", type: "VLESS", network: "udp", latency: 0 },
          { id: "ssh-nas-600001", name: "ssh-nas-600001", type: "SSH", network: "", latency: 0 },
        ],
      },
      {
        id: "auto-select",
        icon: "📍",
        name: "自动选择",
        type: "URLTest",
        selected: "node-select-ref",
        expanded: true,
        members: [
          { id: "node-select-ref", name: "节点选择", type: "Selector", network: "udp", latency: 206, icon: "🚀", groupId: "node-select" },
        ],
      },
      {
        id: "global-direct",
        icon: "🎯",
        name: "全球直连",
        type: "Selector",
        selected: "direct",
        expanded: false,
        members: [{ id: "direct", name: "direct", type: "Direct", network: "", latency: 4 }],
      },
      {
        id: "global-block",
        icon: "🛑",
        name: "全球拦截",
        type: "Selector",
        selected: "block",
        expanded: false,
        members: [{ id: "block", name: "block", type: "Block", network: "", latency: null }],
      },
      {
        id: "fallback",
        icon: "🐟",
        name: "漏网之鱼",
        type: "Selector",
        selected: "nas-ss",
        expanded: false,
        members: [
          { id: "nas-ss", name: "nas-ss", type: "Shadowsocks", network: "udp", latency: 206 },
          { id: "vless-60016", name: "vless-60016", type: "VLESS", network: "udp", latency: 0 },
          { id: "direct", name: "direct", type: "Direct", network: "", latency: 4 },
        ],
      },
      {
        id: "global",
        icon: "🌐",
        name: "GLOBAL",
        type: "Selector",
        selected: "global-direct-ref",
        expanded: false,
        members: [
          { id: "global-direct-ref", name: "全球直连", type: "Selector", network: "", latency: 4, icon: "🎯", groupId: "global-direct" },
          { id: "direct", name: "direct", type: "Direct", network: "", latency: 4 },
        ],
      },
    ],
    theme: readStorage("singbox-theme", "light"),
    accent: readStorage("singbox-accent", "blue"),
    dirty: false,
    lastSaved: "10:24",
    configFields: {
      profileName: "自定义配置 A",
      logLevel: "info",
      mixedPort: "7890",
      apiPort: "9090",
      cacheFile: "cache.db",
      interface: "自动选择",
      testUrl: "https://www.gstatic.com/generate_204",
      testInterval: "5",
      tolerance: "50",
    },
    builtInInbounds: true,
    customInbounds: [],
    formInbounds: [],
    editingProfileId: null,
    editingCustomInboundId: null,
    editingOutboundId: null,
    editingSubscriptionId: null,
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
    routeFindProcess: false,
    routeAutoDetectInterface: true,
    routeDefaultInterface: "",
    routeDefaultDns: "dns-remote",
    routeFinal: "outbound-select",
    editingRouteRuleId: null,
    editingRouteRulesetId: null,
    routeRuleSets: [
      { id: "rs-category-ads", tag: "Category-Ads", type: "remote", format: "binary", url: "https://example.com/category-ads.srs", download_detour: "outbound-direct", update_interval: "1d", path: "", rules: "" },
      { id: "rs-geosite-private", tag: "GeoSite-Private", type: "local", format: "source", url: "", download_detour: "", update_interval: "", path: "4", rules: "" },
      { id: "rs-geosite-cn", tag: "GeoSite-CN", type: "remote", format: "binary", url: "https://example.com/geosite-cn.srs", download_detour: "outbound-direct", update_interval: "1d", path: "", rules: "" },
      { id: "rs-geoip-private", tag: "GeoIP-Private", type: "remote", format: "binary", url: "https://example.com/geoip-private.srs", download_detour: "outbound-direct", update_interval: "1d", path: "", rules: "" },
      { id: "rs-geoip-cn", tag: "GeoIP-CN", type: "remote", format: "binary", url: "https://example.com/geoip-cn.srs", download_detour: "outbound-direct", update_interval: "1d", path: "", rules: "" },
      { id: "rs-geolocation-not-cn", tag: "GeoLocation-!CN", type: "remote", format: "binary", url: "https://example.com/geolocation-not-cn.srs", download_detour: "outbound-direct", update_interval: "1d", path: "", rules: "" },
    ],
    routeRules: [
      { id: "route-rule-tun-sniff", type: "inbound", enable: true, payload: "tun-in", invert: false, action: "sniff", outbound: "", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-dns-hijack", type: "protocol", enable: true, payload: "dns", invert: false, action: "hijack-dns", outbound: "", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-direct-mode", type: "clash_mode", enable: true, payload: "direct", invert: false, action: "route", outbound: "outbound-direct", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-global-mode", type: "clash_mode", enable: true, payload: "global", invert: false, action: "route", outbound: "outbound-select", sniffer: [], strategy: "", server: "" },
      { id: "route-insertion-point", type: "insertion", enable: true, payload: "", invert: false, action: "route", outbound: "", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-lan", type: "ip_cidr", enable: true, payload: "172.16.1.0/24", invert: false, action: "route", outbound: "outbound-select", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-cloud", type: "domain_suffix", enable: true, payload: "maxvisioncloud.com", invert: false, action: "route", outbound: "outbound-direct", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-quic", type: "protocol", enable: false, payload: "quic", invert: false, action: "reject", outbound: "drop", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-ads", type: "rule_set", enable: false, payload: "rs-category-ads", invert: false, action: "route", outbound: "block", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-private", type: "rule_set", enable: false, payload: "rs-geosite-private", invert: false, action: "route", outbound: "outbound-direct", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-cn", type: "rule_set", enable: true, payload: "rs-geosite-cn,rs-geoip-cn", invert: false, action: "route", outbound: "outbound-direct", sniffer: [], strategy: "", server: "" },
      { id: "route-rule-overseas", type: "rule_set", enable: true, payload: "rs-geolocation-not-cn", invert: false, action: "route", outbound: "outbound-select", sniffer: [], strategy: "", server: "" },
    ],
    rulesetSearch: "",
    rulesetType: "all",
    rulesetHubSearch: "",
    rulesetHubCategory: "all",
    rulesetHubUpdated: "今天 09:42",
    editingResourceRulesetId: null,
    settingsSection: "core-settings",
    subscriptions: [
      {
        id: "sub-1",
        name: "Cloud Route Premium",
        logo: "CR",
        tone: "blue",
        nodes: 42,
        nodeList: cloudRouteNodes,
        sourceJson: JSON.stringify({ name: "Cloud Route Premium", outbounds: cloudRouteNodes }, null, 2),
        updated: "8 分钟前",
      },
      {
        id: "sub-2",
        name: "Edge Network",
        logo: "EN",
        tone: "cyan",
        nodes: 18,
        nodeList: edgeNetworkNodes,
        sourceJson: JSON.stringify({ name: "Edge Network", outbounds: edgeNetworkNodes }, null, 2),
        updated: "2 小时前",
      },
    ],
    rulesets: [
      { id: 1, hubId: "geosite-cn", name: "geosite-cn", source: "SagerNet/sing-geosite", type: "remote", format: "binary", count: 124632, size: "3.8 MB", updated: "今天 09:42", url: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", updateInterval: "6h", nextUpdate: "约 6 小时后", disabled: false },
      { id: 2, hubId: "geoip-cn", name: "geoip-cn", source: "SagerNet/sing-geoip", type: "remote", format: "binary", count: 8926, size: "1.2 MB", updated: "今天 09:42", url: "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs", updateInterval: "12h", nextUpdate: "约 12 小时后", disabled: false },
      { id: 3, hubId: "geosite-geolocation-not-cn", name: "geosite-geolocation-!cn", source: "SagerNet/sing-geosite", type: "remote", format: "binary", count: 86319, size: "4.6 MB", updated: "昨天 22:16", url: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-geolocation-!cn.srs", updateInterval: "1d", nextUpdate: "约 1 天后", disabled: false },
      { id: 4, name: "private-custom", source: "手工规则 · JSON", type: "manual", format: "source", count: 4, size: "238 B", updated: "8 月 16 日", url: "", updateInterval: "manual", nextUpdate: "手工维护", disabled: false, content: manualRulesetExample },
      { id: 5, hubId: "geosite-category-ads-all", name: "ads-all", source: "privacy-protection", type: "remote", format: "binary", count: 216804, size: "6.7 MB", updated: "8 月 17 日", url: "https://example.com/rules/ads-all.srs", updateInterval: "7d", nextUpdate: "约 7 天后", disabled: false },
    ],
  };

  const profileDraftKeys = [
    "ipv6",
    "fakeIp",
    "clashApi",
    "allowLan",
    "dirty",
    "lastSaved",
    "configFields",
    "builtInInbounds",
    "customInbounds",
    "formInbounds",
    "outbounds",
    "routeFindProcess",
    "routeAutoDetectInterface",
    "routeDefaultInterface",
    "routeDefaultDns",
    "routeFinal",
    "routeRuleSets",
    "routeRules",
  ];

  function cloneData(value) {
    return JSON.parse(JSON.stringify(value));
  }

  function activeProfile() {
    return state.profiles.find((profile) => profile.id === state.activeProfileId) || state.profiles[0];
  }

  function captureProfileDraft() {
    return Object.fromEntries(profileDraftKeys.map((key) => [key, cloneData(state[key])]));
  }

  function saveActiveProfileDraft() {
    const profile = activeProfile();
    if (profile) profile.draft = captureProfileDraft();
  }

  function createBlankProfileDraft(name) {
    return {
      ipv6: true,
      fakeIp: false,
      clashApi: false,
      allowLan: false,
      dirty: true,
      lastSaved: "尚未保存",
      configFields: {
        profileName: name,
        logLevel: "info",
        mixedPort: "7890",
        apiPort: "9090",
        cacheFile: "cache.db",
        interface: "自动选择",
        testUrl: "https://www.gstatic.com/generate_204",
        testInterval: "5",
        tolerance: "50",
      },
      builtInInbounds: false,
      customInbounds: [],
      formInbounds: [],
      outbounds: [],
      routeFindProcess: false,
      routeAutoDetectInterface: true,
      routeDefaultInterface: "",
      routeDefaultDns: "",
      routeFinal: "direct",
      routeRuleSets: [],
      routeRules: [
        { id: `route-insertion-${Date.now()}`, type: "insertion", enable: true, payload: "", invert: false, action: "route", outbound: "", sniffer: [], strategy: "", server: "" },
      ],
    };
  }

  function importedRouteRule(rule, index) {
    const ignoredKeys = new Set(["action", "outbound", "invert", "sniffer", "strategy", "server"]);
    const type = Object.keys(rule).find((key) => !ignoredKeys.has(key));
    const rawPayload = type ? rule[type] : rule;
    const payload = Array.isArray(rawPayload)
      ? rawPayload.join(",")
      : typeof rawPayload === "object" && rawPayload !== null
        ? JSON.stringify(rawPayload)
        : String(rawPayload ?? "");
    const actionValue = typeof rule.action === "string" ? rule.action : rule.action?.type;
    const action = routeRuleActionOptions.some(([value]) => value === actionValue) ? actionValue : "route";
    return {
      id: `imported-route-${Date.now()}-${index}`,
      type: type || "inline",
      enable: true,
      payload: type ? payload : JSON.stringify(rule),
      invert: Boolean(rule.invert),
      action,
      outbound: String(rule.outbound || rule.action?.outbound || ""),
      sniffer: Array.isArray(rule.sniffer) ? rule.sniffer : [],
      strategy: String(rule.strategy || ""),
      server: String(rule.server || ""),
    };
  }

  function profileDraftFromJson(config, name) {
    const inbounds = Array.isArray(config.inbounds) ? config.inbounds : [];
    const outbounds = Array.isArray(config.outbounds) ? config.outbounds : [];
    const route = config.route && typeof config.route === "object" ? config.route : {};
    const routeRules = Array.isArray(route.rules) ? route.rules.map(importedRouteRule) : [];
    routeRules.push({ id: `route-insertion-${Date.now()}`, type: "insertion", enable: true, payload: "", invert: false, action: "route", outbound: "", sniffer: [], strategy: "", server: "" });
    const mixedInbound = inbounds.find((inbound) => inbound?.type === "mixed");
    const externalController = String(config.experimental?.clash_api?.external_controller || "");
    return {
      ...createBlankProfileDraft(name),
      ipv6: config.dns?.strategy !== "ipv4_only",
      fakeIp: Boolean(config.dns?.fakeip),
      clashApi: Boolean(config.experimental?.clash_api),
      dirty: true,
      configFields: {
        ...createBlankProfileDraft(name).configFields,
        profileName: name,
        logLevel: String(config.log?.level || "info"),
        mixedPort: String(mixedInbound?.listen_port || "7890"),
        apiPort: externalController.split(":").pop() || "9090",
        cacheFile: String(config.experimental?.cache_file?.path || "cache.db"),
        interface: String(route.default_interface || "自动选择"),
      },
      builtInInbounds: false,
      customInbounds: inbounds.map((inbound, index) => ({
        id: `imported-in-${Date.now()}-${index}`,
        json: JSON.stringify(inbound, null, 2),
        config: cloneData(inbound),
      })),
      outbounds: outbounds.map((outbound, index) => ({
        ...cloneData(outbound),
        id: `imported-out-${Date.now()}-${index}`,
        tag: String(outbound.tag || `${outbound.type || "custom"}-${index + 1}`),
        type: String(outbound.type || "custom"),
        refs: Array.isArray(outbound.outbounds)
          ? outbound.outbounds.map((tag) => ({ groupId: "imported", id: String(tag), tag: String(tag), type: "Imported" }))
          : [],
      })),
      routeFindProcess: Boolean(route.find_process),
      routeAutoDetectInterface: route.auto_detect_interface !== false,
      routeDefaultInterface: String(route.default_interface || ""),
      routeDefaultDns: String(route.default_domain_resolver || ""),
      routeFinal: String(route.final || "direct"),
      routeRuleSets: (Array.isArray(route.rule_set) ? route.rule_set : []).map((ruleset, index) => ({
        id: String(ruleset.tag || `imported-ruleset-${index + 1}`),
        tag: String(ruleset.tag || `规则集 ${index + 1}`),
        type: String(ruleset.type || "inline"),
        format: String(ruleset.format || "source"),
        url: String(ruleset.url || ""),
        download_detour: String(ruleset.download_detour || ""),
        update_interval: String(ruleset.update_interval || ""),
        path: String(ruleset.path || ""),
        rules: JSON.stringify(ruleset.rules || [], null, 2),
      })),
      routeRules,
    };
  }

  function outboundConfigForExport(outbound) {
    const { id, refs = [], ...config } = cloneData(outbound);
    if (["selector", "urltest"].includes(config.type)) {
      config.outbounds = refs.map((reference) => reference.tag).filter(Boolean);
    }
    return config;
  }

  function routeRuleForExport(rule) {
    if (rule.type === "inline") {
      try {
        return JSON.parse(rule.payload);
      } catch {
        return { action: rule.action || "route", outbound: rule.outbound || undefined };
      }
    }
    const listTypes = new Set(["inbound", "network", "protocol", "domain", "domain_suffix", "domain_keyword", "domain_regex", "source_ip_cidr", "ip_cidr", "source_port", "source_port_range", "port", "port_range", "process_name", "process_path", "process_path_regex", "clash_mode", "rule_set"]);
    const values = String(rule.payload || "").split(",").map((item) => item.trim()).filter(Boolean);
    const config = {
      [rule.type]: listTypes.has(rule.type) ? values : rule.payload,
    };
    if (rule.invert) config.invert = true;
    if (rule.action && rule.action !== "route") config.action = rule.action;
    if (rule.outbound) config.outbound = rule.outbound;
    if (rule.sniffer?.length) config.sniffer = rule.sniffer;
    if (rule.strategy) config.strategy = rule.strategy;
    if (rule.server) config.server = rule.server;
    return config;
  }

  function routeRulesetForExport(ruleset) {
    const config = { tag: ruleset.tag, type: ruleset.type };
    if (ruleset.type === "remote") {
      config.format = ruleset.format;
      config.url = ruleset.url;
      if (ruleset.download_detour) config.download_detour = ruleset.download_detour;
      if (ruleset.update_interval) config.update_interval = ruleset.update_interval;
    } else if (ruleset.type === "local") {
      config.format = ruleset.format;
      config.path = ruleset.path;
    } else {
      try { config.rules = JSON.parse(ruleset.rules); } catch { config.rules = []; }
    }
    return config;
  }

  function currentProfileConfigObject() {
    const profile = activeProfile();
    const config = profile.rawConfig ? cloneData(profile.rawConfig) : {};
    config.log = { ...(config.log || {}), level: state.configFields.logLevel };
    const builtIn = state.builtInInbounds
      ? [
          { type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: Number(state.configFields.mixedPort) || 7890 },
          { type: "tun", tag: "tun-in", interface_name: "singtun0", address: ["172.19.0.1/30"], auto_route: true, strict_route: true },
          { type: "direct", tag: "dns-in", listen: "127.0.0.1", listen_port: 5353 },
        ]
      : [];
    config.inbounds = [
      ...builtIn,
      ...state.formInbounds.map((item) => cloneData(item.config)),
      ...state.customInbounds.map((item) => cloneData(item.config)),
    ];
    config.outbounds = state.outbounds.map(outboundConfigForExport);
    config.route = {
      ...(config.route || {}),
      auto_detect_interface: state.routeAutoDetectInterface,
      final: state.routeFinal,
      rules: state.routeRules.filter((rule) => rule.type !== "insertion").map(routeRuleForExport),
      rule_set: state.routeRuleSets.map(routeRulesetForExport),
    };
    if (state.routeFindProcess) config.route.find_process = true;
    else delete config.route.find_process;
    if (state.routeDefaultInterface) config.route.default_interface = state.routeDefaultInterface;
    else delete config.route.default_interface;
    if (state.routeDefaultDns) config.route.default_domain_resolver = state.routeDefaultDns;
    else delete config.route.default_domain_resolver;
    if (!config.dns) config.dns = { servers: [], rules: [] };
    return config;
  }

  function loadProfileDraft(profile) {
    const defaults = createBlankProfileDraft(profile.name);
    const saved = profile.draft || defaults;
    const draft = {
      ...defaults,
      ...cloneData(saved),
      configFields: { ...defaults.configFields, ...(saved.configFields || {}), profileName: saved.configFields?.profileName || profile.name },
    };
    profile.draft = cloneData(draft);
    profileDraftKeys.forEach((key) => {
      state[key] = cloneData(draft[key]);
    });
  }

  function persistProfiles() {
    writeStorage("singbox-config-profiles", JSON.stringify({
      activeProfileId: state.activeProfileId,
      profiles: state.profiles,
    }));
  }

  function initializeProfiles() {
    const stored = readJsonStorage("singbox-config-profiles");
    const storedProfiles = Array.isArray(stored?.profiles)
      ? stored.profiles.filter((profile) => profile?.id && profile?.name && profile?.draft)
      : [];
    if (storedProfiles.length) {
      state.profiles = storedProfiles;
      const routeProfileId = initialRoute[0] === "config" && initialRoute[2] ? initialRoute[1] : null;
      state.activeProfileId = storedProfiles.some((profile) => profile.id === routeProfileId)
        ? routeProfileId
        : storedProfiles.some((profile) => profile.id === stored.activeProfileId)
          ? stored.activeProfileId
          : storedProfiles[0].id;
      loadProfileDraft(activeProfile());
      return;
    }
    state.profiles[0].draft = captureProfileDraft();
  }

  const pageMeta = {
    dashboard: { title: "仪表盘", parent: "控制台" },
    config: { title: "配置工作台", parent: "配置" },
    subscriptions: { title: "订阅配置", parent: "资源管理" },
    rulesets: { title: "规则集中心", parent: "资源管理" },
    settings: { title: "设置", parent: "系统" },
  };

  function inboundCount() {
    return (state.builtInInbounds ? 3 : 0) + state.customInbounds.length + state.formInbounds.length;
  }

  function routeRuleCount() {
    return state.routeRules.filter((rule) => rule.type !== "insertion").length;
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
        <div class="nav-parent-row">
          <button class="nav-parent ${configActive ? "active" : ""} ${state.configExpanded ? "expanded" : ""}" data-action="config-parent">
            ${icon("config")}<span>配置</span><span class="nav-count">${state.profiles.length}</span>${icon("chevron", "nav-chevron")}
          </button>
          <button class="nav-parent-add" data-modal="profile-new" aria-label="新增配置" title="新增配置">${icon("plus")}</button>
        </div>
        ${
          state.configExpanded
            ? `<div class="nav-config-tree">
                ${state.profiles.map((profile) => `<div class="config-tree-item ${profile.id === state.activeProfileId ? "active" : ""}">
                  <button class="config-name" data-profile="${escapeHtml(profile.id)}" title="切换到 ${escapeHtml(profile.name)}">
                    <span class="config-status-dot"></span><span>${escapeHtml(profile.name)}</span>${(profile.id === state.activeProfileId ? state.dirty : profile.draft?.dirty) ? '<i class="config-dirty-dot" title="有未保存更改"></i>' : ""}
                  </button>
                  <button class="config-item-action" data-modal="profile-manage" data-id="${escapeHtml(profile.id)}" aria-label="管理 ${escapeHtml(profile.name)}" title="管理配置">${icon("more")}</button>
                </div>`).join("")}
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
        <h1>${state.page === "config" ? escapeHtml(activeProfile().name) : meta.title}</h1>
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
                <div class="hero-meta-item"><span>活动配置</span><strong>${escapeHtml(activeProfile().name)}</strong></div>
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
              <div class="profile-name">${escapeHtml(activeProfile().name)}</div>
              <div class="profile-description">规则模式 · 最后应用于今天 10:24</div>
              <div class="profile-stats">
                <div class="profile-stat"><strong>${inboundCount()}</strong><span>入站</span></div>
                <div class="profile-stat"><strong>${state.outbounds.length}</strong><span>出站</span></div>
                <div class="profile-stat"><strong>${routeRuleCount()}</strong><span>路由规则</span></div>
              </div>
            </article>
            <article class="card route-mode-card">
              <div class="mode-title-row"><h3 class="card-title">路由模式</h3><span class="faint" style="font-size:9px">即时生效</span></div>
              <div class="mode-options">
                ${["rule", "global", "direct"].map((mode) => `<button class="mode-option ${state.routeMode === mode ? "active" : ""}" data-mode="${mode}">${{ rule: "规则", global: "全局", direct: "直连" }[mode]}</button>`).join("")}
              </div>
            </article>
          </div>

          ${renderProxySelector()}
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

  function proxyGroupSelectedLabel(group) {
    const selected = group.members.find((member) => member.id === group.selected);
    if (!selected) return "未选择";
    if (!selected.groupId) return selected.name;
    const referencedGroup = state.proxyGroups.find((item) => item.id === selected.groupId);
    const referencedNode = referencedGroup?.members.find((member) => member.id === referencedGroup.selected);
    return referencedNode ? `${selected.name} / ${referencedNode.name}` : selected.name;
  }

  function proxyLatencyMarkup(member) {
    if (state.proxyTesting) return `<span class="proxy-latency testing">${icon("rotate")}测速中</span>`;
    if (member.latency === null) return `<span class="proxy-latency unavailable">不适用</span>`;
    if (!member.latency) return `<span class="proxy-latency timeout">超时</span>`;
    const tone = member.latency <= 100 ? "fast" : member.latency <= 300 ? "medium" : "slow";
    return `<span class="proxy-latency ${tone}">${member.latency} ms</span>`;
  }

  function proxyNodeCard(group, member) {
    const selected = member.id === group.selected;
    const initial = member.icon || (/^[a-z]/i.test(member.name) ? member.name.slice(0, 2).toUpperCase() : member.name.slice(0, 1));
    return `<button class="proxy-node-card ${selected ? "selected" : ""}" data-proxy-node="${escapeHtml(member.id)}" data-group="${escapeHtml(group.id)}" aria-pressed="${selected}">
      <span class="proxy-node-avatar ${member.icon ? "emoji" : ""}">${escapeHtml(initial)}</span>
      <span class="proxy-node-copy"><strong>${escapeHtml(member.name)}</strong>${proxyLatencyMarkup(member)}<small>${escapeHtml(member.type)}${member.network ? ` · ${escapeHtml(member.network)}` : ""}</small></span>
      <span class="proxy-node-check">${selected ? icon("check") : ""}</span>
    </button>`;
  }

  function renderProxyGroup(group) {
    return `<section class="proxy-group ${group.expanded ? "expanded" : ""}" data-proxy-group-section="${escapeHtml(group.id)}">
      <div class="proxy-group-head">
        <button class="proxy-group-main" data-proxy-group="${escapeHtml(group.id)}" aria-expanded="${group.expanded}">
          <span class="proxy-group-avatar">${escapeHtml(group.icon)}</span>
          <span class="proxy-group-copy"><strong>${escapeHtml(group.name)}</strong><small><b>${escapeHtml(group.type)}</b><i></i>${escapeHtml(proxyGroupSelectedLabel(group))}</small></span>
        </button>
        <div class="proxy-group-actions">
          <button class="proxy-icon-button" data-action="test-proxy-group" data-id="${escapeHtml(group.id)}" title="测试该组延迟" aria-label="测试 ${escapeHtml(group.name)} 延迟">${icon("gauge")}</button>
          <button class="proxy-icon-button proxy-expand-button" data-proxy-group="${escapeHtml(group.id)}" aria-expanded="${group.expanded}" title="${group.expanded ? "收起" : "展开"}">${icon("chevron")}</button>
        </div>
      </div>
      ${group.expanded ? `<div class="proxy-node-grid">${group.members.map((member) => proxyNodeCard(group, member)).join("")}</div>` : ""}
    </section>`;
  }

  function renderProxySelector() {
    const onlineNodes = state.proxyGroups.flatMap((group) => group.members).filter((member) => Number(member.latency) > 0).length;
    return `<article class="card proxy-selector-card">
      <div class="card-head proxy-selector-head">
        <div><h3 class="card-title">节点选择</h3><p class="card-subtitle">选择策略组当前使用的出站节点</p></div>
        <div class="proxy-selector-actions"><span class="proxy-online-summary"><i></i>${onlineNodes} 个节点可用</span><button class="btn btn-sm" data-action="test-all-proxies" ${state.proxyTesting ? "disabled" : ""}>${icon("gauge")}${state.proxyTesting ? "测速中" : "全部测速"}</button></div>
      </div>
      <div class="proxy-group-list">${state.proxyGroups.map(renderProxyGroup).join("")}</div>
    </article>`;
  }

  function refreshProxyLatencies(groups) {
    groups.forEach((group, groupIndex) => {
      group.members.forEach((member, memberIndex) => {
        if (member.latency === null) return;
        member.latency = 28 + ((groupIndex * 61 + memberIndex * 97 + Date.now()) % 330);
      });
    });
  }

  function configIntro() {
    const profile = activeProfile();
    return `<div class="page-intro">
      <div class="page-intro-copy"><h2>配置工作台</h2><p>通过表单管理 sing-box 配置，保存后可直接应用到运行中的核心。</p></div>
      <div class="page-actions">
        <button class="btn" data-modal="json">${icon("code")}JSON 预览</button>
        <button class="btn btn-primary" id="save-config-button" data-action="save-config">${icon("save")}${state.dirty ? "保存更改" : "保存并应用"}</button>
      </div>
    </div>
    <article class="card profile-banner">
      <div class="profile-banner-icon">${icon("config")}</div>
      <div class="profile-banner-copy"><div class="profile-banner-title">${escapeHtml(profile.name)} <span class="badge badge-success">当前配置</span></div><div class="profile-banner-desc">${escapeHtml(profile.source)} · ${escapeHtml(profile.id)}</div></div>
      <div class="profile-banner-meta"><div><span class="faint">最后保存</span><strong>${escapeHtml(state.lastSaved)}</strong></div><div><span class="faint">配置版本</span><strong>v${profile.version}</strong></div><button class="btn btn-sm" data-modal="profile-manage" data-id="${escapeHtml(profile.id)}">${icon("edit")}管理</button></div>
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
      return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}${hint ? `<small>${hint}</small>` : ""}</span><select class="field-select" data-config-field="${name}">${options.items.map((item) => `<option ${item === value ? "selected" : ""}>${escapeHtml(item)}</option>`).join("")}</select></label>`;
    }
    return `<label class="form-field ${full ? "full" : ""}"><span class="field-label">${label}${hint ? `<small>${hint}</small>` : ""}</span><span class="${suffix ? "input-suffix-wrap" : ""}"><input class="field-input" type="${type}" value="${escapeHtml(value)}" data-config-field="${name}" />${suffix ? `<span class="input-suffix">${suffix}</span>` : ""}</span></label>`;
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
            ${formField("配置名称", "profileName", state.configFields.profileName, { hint: "保存后同步更新配置列表" })}
            ${formField("日志等级", "logLevel", state.configFields.logLevel, { type: "select", items: ["debug", "info", "warn", "error", "panic"] })}
            ${formField("Mixed 入站端口", "mixedPort", state.configFields.mixedPort, { type: "number", suffix: "端口" })}
            ${formField("Clash API 端口", "apiPort", state.configFields.apiPort, { type: "number", suffix: "端口" })}
            ${formField("缓存文件", "cacheFile", state.configFields.cacheFile, { hint: "核心工作目录下" })}
            ${formField("网络接口", "interface", state.configFields.interface, { type: "select", items: ["自动选择", "eth0", "wlan0", "en0"] })}
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
            ${summarySetting("出站", `${state.outbounds.length} 个项目`, "outbound")}
            ${summarySetting("路由", `${routeRuleCount()} 条规则`, "route")}
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
    const json = JSON.stringify(currentProfileConfigObject(), null, 2);
    return `<div class="code-preview ${full ? "code-preview-full" : ""}">
      <div class="code-preview-head"><span class="window-dots"><i></i><i></i><i></i></span><span>config.json</span><button class="entry-action" data-action="copy-json" title="复制 JSON">${icon("copy")}</button></div>
      <pre>${escapeHtml(json)}</pre>
    </div>`;
  }

  function renderInboundConfig() {
    return `<div class="config-layout">
      <div class="config-main">
        <article class="card">
          <div class="card-head"><div><h3 class="card-title">入站监听器</h3><p class="card-subtitle">流量进入 sing-box 的入口</p></div><button class="btn btn-sm btn-primary" data-modal="inbound">${icon("plus")}新增入站</button></div>
          <div class="entry-list">
            ${state.builtInInbounds ? `${entryCard("mixed-in", "Mixed Proxy", "mixed", "127.0.0.1:7890", "HTTP 与 SOCKS 复合代理", "inbound", "")}
            ${entryCard("tun-in", "TUN Interface", "tun", "singtun0 · 172.19.0.1/30", "自动路由 · 严格路由", "shield", "cyan")}
            ${entryCard("dns-in", "DNS Listener", "direct", "127.0.0.1:5353", "DNS 劫持入口", "dns", "green")}` : ""}
            ${state.formInbounds.map(formInboundCard).join("")}
            ${state.customInbounds.map(customInboundCard).join("")}
            ${inboundCount() ? "" : `<button class="config-empty-state" data-modal="inbound">${icon("inbound")}<strong>尚未添加入站</strong><span>点击创建第一个监听入口</span></button>`}
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
            ${state.outbounds.length ? "" : `<button class="config-empty-state" data-modal="outbound">${icon("outbound")}<strong>尚未添加出站</strong><span>点击创建第一个出站或策略组</span></button>`}
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
            ${formField("测试地址", "testUrl", state.configFields.testUrl)}
            ${formField("测试间隔", "testInterval", state.configFields.testInterval, { suffix: "分钟" })}
            ${formField("容差", "tolerance", state.configFields.tolerance, { suffix: "ms" })}
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
    const enabledRules = state.routeRules.filter((rule) => rule.type !== "insertion" && rule.enable).length;
    return `<div class="route-workspace">
      <article class="card route-section" id="route-common-section">
        <div class="card-head"><div><h3 class="card-title">通用设置</h3><p class="card-subtitle">控制接口探测、域名解析与未命中规则时的默认出口</p></div><span class="route-section-badge">全局行为</span></div>
        <div class="route-common-body">
          <div class="route-common-toggles">
            ${routeCommonToggle("查找进程信息", "为路由规则补充进程名称与路径", "routeFindProcess")}
            ${routeCommonToggle("自动检测出站接口", "自动选择可用的系统网络接口", "routeAutoDetectInterface")}
          </div>
          <div class="route-common-fields">
            ${state.routeAutoDetectInterface ? "" : routeCommonSelect("出站接口名称", "关闭自动检测后使用的默认接口", "routeDefaultInterface", state.routeDefaultInterface, [["", "请选择网络接口"], ["eth0", "eth0"], ["wlan0", "wlan0"], ["en0", "en0"]])}
            ${routeCommonSelect("解析节点域名的 DNS 服务器", "解析代理节点服务器域名时使用", "routeDefaultDns", state.routeDefaultDns, routeDnsOptions())}
            ${routeCommonSelect("默认出站标签", "所有规则均未命中时使用", "routeFinal", state.routeFinal, routeOutboundOptions())}
          </div>
        </div>
      </article>

      <article class="card route-section" id="route-ruleset-section">
        <div class="card-head"><div><h3 class="card-title">规则集</h3><p class="card-subtitle">管理路由规则可引用的本地、远程与内联规则集</p></div><div class="route-section-actions"><span class="route-section-badge">${state.routeRuleSets.length} 个</span><button class="btn btn-sm" data-modal="route-ruleset">${icon("plus")}新增规则集</button></div></div>
        <div class="route-ruleset-grid">${state.routeRuleSets.length ? state.routeRuleSets.map((ruleset) => routeRulesetCard(ruleset)).join("") : `<button class="route-empty-section" data-modal="route-ruleset">${icon("plus")}<strong>添加第一个规则集</strong><span>支持本地、远程与内联来源</span></button>`}</div>
      </article>

      <article class="card route-section" id="route-rules-section">
        <div class="card-head"><div><h3 class="card-title">路由规则</h3><p class="card-subtitle">按从上到下的顺序匹配；新增规则会放在插入点之后</p></div><div class="route-section-actions"><span class="route-section-badge">已启用 ${enabledRules}/${routeRuleCount()}</span><button class="btn btn-sm btn-primary" data-modal="route">${icon("plus")}新增规则</button></div></div>
        <div class="route-rule-list">${state.routeRules.map((rule, index) => routeRuleCard(rule, index)).join("")}</div>
      </article>
    </div>`;
  }

  function routeOutboundOptions() {
    const options = [["direct", "direct（内置）"], ["block", "block（内置）"], ...state.outbounds.map((outbound) => [outbound.id, outbound.tag])];
    return options.filter(([value], index) => options.findIndex(([candidate]) => candidate === value) === index);
  }

  function routeDnsOptions() {
    return [["", "由 DNS 路由规则决定"], ["dns-remote", "Cloudflare DoH"], ["dns-domestic", "AliDNS DoH"], ["dns-local", "本地 DNS"]];
  }

  function routeInboundOptions() {
    const options = state.builtInInbounds ? [["mixed-in", "mixed-in"], ["tun-in", "tun-in"], ["dns-in", "dns-in"]] : [];
    state.formInbounds.forEach((item) => options.push([item.id, item.config.tag || item.config.type]));
    state.customInbounds.forEach((item) => options.push([item.id, item.config.tag || item.config.type]));
    return options;
  }

  function selectOptionsMarkup(options, selected) {
    return options.map(([value, label]) => `<option value="${escapeHtml(value)}" ${String(value) === String(selected) ? "selected" : ""}>${escapeHtml(label)}</option>`).join("");
  }

  function routeCommonToggle(label, description, key) {
    return `<div class="route-common-toggle"><div class="route-common-icon">${icon(key === "routeFindProcess" ? "search" : "globe")}</div><div><strong>${label}</strong><span>${description}</span></div><button class="switch ${state[key] ? "on" : ""}" data-route-toggle="${key}" aria-label="切换${label}" aria-pressed="${state[key]}"></button></div>`;
  }

  function routeCommonSelect(label, description, name, value, options) {
    return `<label class="route-common-field"><span><strong>${label}</strong><small>${description}</small></span><select class="field-select" name="${name}" data-route-common>${selectOptionsMarkup(options, value)}</select></label>`;
  }

  function routeRulesetCard(ruleset) {
    const source = ruleset.type === "remote" ? ruleset.url : ruleset.type === "local" ? `资源 #${ruleset.path}` : "页面内联规则";
    const tone = ruleset.type === "remote" ? "" : ruleset.type === "local" ? "cyan" : "violet";
    return `<div class="route-ruleset-card" data-route-ruleset-id="${escapeHtml(ruleset.id)}">
      <div class="route-ruleset-icon ${tone}">${icon(ruleset.type === "remote" ? "download" : ruleset.type === "local" ? "folder" : "code")}</div>
      <div class="route-ruleset-copy"><div><strong>${escapeHtml(ruleset.tag)}</strong><span class="type-badge">${escapeHtml(ruleset.type)}</span><span class="type-badge">${escapeHtml(ruleset.type === "inline" ? "source" : ruleset.format)}</span></div><small title="${escapeHtml(source)}">${escapeHtml(source)}</small></div>
      <div class="entry-actions"><button class="entry-action" data-modal="route-ruleset" data-id="${escapeHtml(ruleset.id)}" title="编辑规则集">${icon("edit")}</button><button class="entry-action danger" data-route-remove="ruleset" data-id="${escapeHtml(ruleset.id)}" title="删除规则集">${icon("trash")}</button></div>
    </div>`;
  }

  function routeOutboundLabel(value) {
    return routeOutboundOptions().find(([id]) => id === value)?.[1] || value;
  }

  function routeRulePayloadLabel(rule) {
    if (rule.type === "rule_set") {
      return rule.payload.split(",").filter(Boolean).map((id) => state.routeRuleSets.find((ruleset) => ruleset.id === id)?.tag || id).join(", ");
    }
    if (rule.type === "inbound") return routeInboundOptions().find(([id]) => id === rule.payload)?.[1] || rule.payload;
    return rule.payload;
  }

  function routeRuleActionLabel(rule) {
    if (rule.action === "route") return routeOutboundLabel(rule.outbound);
    if (rule.action === "route-options") return "JSON 路由选项";
    if (rule.action === "reject") return { default: "默认拒绝", drop: "静默丢弃", reply: "返回响应" }[rule.outbound] || "拒绝连接";
    if (rule.action === "hijack-dns") return "劫持至 DNS";
    if (rule.action === "sniff") return rule.sniffer?.length ? rule.sniffer.join(", ") : "全部探测器";
    if (rule.action === "resolve") return routeDnsOptions().find(([id]) => id === rule.server)?.[1] || "由 DNS 规则决定";
    return rule.action;
  }

  function routeRuleCard(rule, index) {
    if (rule.type === "insertion") {
      return `<div class="route-insertion-point"><span></span><button data-modal="route">${icon("plus")}新规则将插入到这里</button><span></span></div>`;
    }
    const displayIndex = state.routeRules.slice(0, index + 1).filter((item) => item.type !== "insertion").length;
    const payload = routeRulePayloadLabel(rule);
    const actionLabel = routeRuleActionOptions.find(([value]) => value === rule.action)?.[1] || rule.action;
    return `<div class="route-rule-row ${rule.enable ? "" : "is-disabled"}" data-route-rule-id="${escapeHtml(rule.id)}">
      <span class="entry-drag">${icon("grip")}</span>
      <label class="route-rule-switch" title="${rule.enable ? "停用规则" : "启用规则"}"><input type="checkbox" data-route-rule-enabled data-id="${escapeHtml(rule.id)}" ${rule.enable ? "checked" : ""} /><span></span></label>
      <span class="route-rule-index">${String(displayIndex).padStart(2, "0")}</span>
      <div class="route-rule-copy"><strong>${escapeHtml(rule.type)}<span>,</span>${escapeHtml(payload || "未设置载荷")}${rule.invert ? '<em>反向</em>' : ""}</strong><small>${escapeHtml(actionLabel)} · ${escapeHtml(routeRuleActionLabel(rule))}</small></div>
      <span class="route-action-badge action-${escapeHtml(rule.action)}">${icon(rule.action === "reject" ? "shield" : rule.action.includes("dns") || rule.action === "resolve" ? "dns" : rule.action === "sniff" ? "search" : "route")}${escapeHtml(actionLabel)}</span>
      <div class="entry-actions"><button class="entry-action" data-modal="route" data-id="${escapeHtml(rule.id)}" title="编辑规则">${icon("edit")}</button><button class="entry-action danger" data-route-remove="rule" data-id="${escapeHtml(rule.id)}" title="删除规则">${icon("trash")}</button></div>
    </div>`;
  }

  function dnsRouteRule(order, title, type, value, target, tone) {
    return `<div class="entry-card">
      <span class="entry-drag">${icon("grip")}</span>
      <span class="rule-order">${order}</span>
      <div class="entry-copy">
        <div class="entry-title-row">
          <span class="entry-title">${title}</span>
          <span class="type-badge">${type}</span>
        </div>
        <div class="entry-meta"><span>${value}</span></div>
      </div>
      <span class="route-target ${tone ? `badge-${tone}` : ""}">${icon("route")}${target}</span>
      <div class="entry-actions">
        <button class="entry-action" data-action="edit-entry">${icon("edit")}</button>
        <button class="entry-action danger" data-action="remove-entry">${icon("trash")}</button>
      </div>
    </div>`;
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
            ${dnsRouteRule(1, "中国大陆域名", "rule_set", "geosite-cn", "AliDNS DoH", "green")}
            ${dnsRouteRule(2, "国外域名", "rule_set", "geosite-geolocation-!cn", "Cloudflare DoH", "")}
            ${dnsRouteRule(3, "默认解析", "final", "未命中规则", "Cloudflare DoH", "orange")}
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

  function getSubscriptionNodes(subscription) {
    return Array.isArray(subscription?.nodeList) ? subscription.nodeList : [];
  }

  function subscriptionNodeCount(subscription) {
    return getSubscriptionNodes(subscription).length;
  }

  function subscriptionProtocols(subscription) {
    return [...new Set(getSubscriptionNodes(subscription).map((node) => String(node.type || "unknown")))];
  }

  function renderSubscriptions() {
    const totalNodes = state.subscriptions.reduce((sum, item) => sum + subscriptionNodeCount(item), 0);
    const protocolCount = new Set(state.subscriptions.flatMap(subscriptionProtocols)).size;
    return `<section>
      <div class="page-intro"><div class="page-intro-copy"><h2>订阅配置</h2><p>仅支持通过 JSON 编辑器批量导入 sing-box 出站节点。</p></div><div class="page-actions"><button class="btn btn-primary" data-modal="subscription">${icon("code")}导入 JSON 订阅</button></div></div>
      <div class="summary-grid">
        ${summaryMetric("subscription", "", state.subscriptions.length, "JSON 订阅", "本地编辑与维护")}
        ${summaryMetric("globe", "cyan", totalNodes, "已导入节点", "完整保留 outbounds")}
        ${summaryMetric("code", "orange", protocolCount, "协议类型", "由节点 JSON 自动识别")}
      </div>
      <div class="subscription-grid">
        ${state.subscriptions.map(renderSubscriptionCard).join("")}
        <button class="card empty-state" data-modal="subscription"><div><div class="empty-icon">${icon("code")}</div><h3>添加 JSON 订阅</h3><p>粘贴完整配置或节点数组，一次导入全部节点</p><span class="btn btn-sm btn-primary">打开 JSON 编辑器</span></div></button>
      </div>
    </section>`;
  }

  function summaryMetric(iconName, tone, value, label, detail) {
    return `<article class="card summary-card"><div class="metric-icon ${tone}">${icon(iconName)}</div><div><div class="summary-value">${value}</div><div class="summary-label">${label} · ${detail}</div></div></article>`;
  }

  function renderSubscriptionCard(item) {
    const nodes = getSubscriptionNodes(item);
    const protocols = subscriptionProtocols(item);
    return `<article class="card subscription-card" data-subscription-card data-id="${escapeHtml(item.id)}" tabindex="0" aria-label="${escapeHtml(`双击查看 ${item.name} 的全部 ${nodes.length} 个节点`)}">
      <div class="subscription-main">
        <div class="subscription-top"><div class="provider-logo ${item.tone === "cyan" ? "cyan" : ""}">${escapeHtml(item.logo)}</div><div class="subscription-name"><strong>${escapeHtml(item.name)}</strong><span>JSON 节点集合 · ${nodes.length} 个出站</span></div><span class="badge badge-success">JSON</span></div>
        <div class="subscription-json-source"><div class="subscription-source-icon">${icon("code")}</div><div><strong>完整节点已导入</strong><span>编辑 JSON 会整体更新该订阅中的全部节点</span></div></div>
        <div class="subscription-meta"><div class="subscription-meta-item"><span>节点数量</span><strong>${nodes.length} 个</strong></div><div class="subscription-meta-item"><span>协议类型</span><strong>${protocols.length} 种</strong></div><div class="subscription-meta-item"><span>最后编辑</span><strong>${escapeHtml(item.updated)}</strong></div></div>
        <div class="subscription-protocols">${protocols.slice(0, 4).map((type) => `<span class="type-badge">${escapeHtml(type)}</span>`).join("")}${protocols.length > 4 ? `<span class="type-badge">+${protocols.length - 4}</span>` : ""}</div>
      </div>
      <div class="subscription-footer"><span class="subscription-update">${icon("info")}双击卡片查看全部节点</span><div class="inline-actions"><button class="btn btn-sm btn-quiet" data-action="view-subscription-nodes" data-id="${escapeHtml(item.id)}">${icon("globe")}查看节点</button><button class="btn btn-sm" data-action="edit-subscription" data-id="${escapeHtml(item.id)}">${icon("edit")}编辑 JSON</button></div></div>
    </article>`;
  }

  function rulesetCountValue(item) {
    if (typeof item.count === "number") return item.count;
    return Number(String(item.count || 0).replace(/,/g, "")) || 0;
  }

  function formatRulesetCount(value) {
    return Number(value || 0).toLocaleString("zh-CN");
  }

  function rulesetTypeMeta(type) {
    return {
      remote: { label: "网络", iconName: "globe", badge: "badge-primary" },
      manual: { label: "纯手工", iconName: "code", badge: "badge-success" },
    }[type] || { label: type, iconName: "rules", badge: "" };
  }

  function rulesetIntervalLabel(interval) {
    return rulesetUpdateIntervals.find(([value]) => value === interval)?.[1] || interval || "仅手动更新";
  }

  function rulesetNextUpdateLabel(interval) {
    return {
      "15m": "约 15 分钟后",
      "1h": "约 1 小时后",
      "6h": "约 6 小时后",
      "12h": "约 12 小时后",
      "1d": "约 1 天后",
      "7d": "约 7 天后",
    }[interval] || "等待手动更新";
  }

  function renderRulesets() {
    const filtered = state.rulesets.filter((item) => {
      const matchesSearch = `${item.name} ${item.source} ${item.url || ""}`.toLowerCase().includes(state.rulesetSearch.toLowerCase());
      const matchesType = state.rulesetType === "all" || item.type === state.rulesetType;
      return matchesSearch && matchesType;
    });
    const remoteRulesets = state.rulesets.filter((item) => item.type === "remote");
    const scheduledRulesets = remoteRulesets.filter((item) => item.updateInterval && item.updateInterval !== "manual");
    const manualRulesets = state.rulesets.filter((item) => item.type === "manual");
    const totalRules = state.rulesets.reduce((total, item) => total + rulesetCountValue(item), 0);
    const nextScheduled = scheduledRulesets[0];
    return `<section>
      <div class="page-intro"><div class="page-intro-copy"><h2>规则集中心</h2><p>集中导入、更新和维护 sing-box 域名与 IP 规则资源。</p></div><div class="page-actions"><button class="btn" data-modal="ruleset-hub">${icon("database")}规则集中心</button><button class="btn" data-action="update-rulesets">${icon("rotate")}更新网络规则</button><button class="btn btn-primary" data-modal="resource-ruleset">${icon("plus")}添加规则集</button></div></div>
      <div class="summary-grid">
        ${summaryMetric("database", "", state.rulesets.length, "规则集", `${formatRulesetCount(totalRules)} 条规则`)}
        ${summaryMetric("clock", "cyan", scheduledRulesets.length, "定时更新", `${remoteRulesets.length} 个网络资源`)}
        ${summaryMetric("code", "orange", manualRulesets.length, "纯手工规则", "直接编辑 JSON 源文件")}
      </div>
      <article class="card ruleset-schedule-banner">
        <div class="ruleset-schedule-icon">${icon("clock")}</div>
        <div class="ruleset-schedule-copy"><strong>网络规则定时更新已启用</strong><span>${nextScheduled ? `最近计划：${escapeHtml(nextScheduled.name)} · ${escapeHtml(nextScheduled.nextUpdate)}` : "尚未配置自动更新周期"}</span></div>
        <div class="ruleset-schedule-stats"><span><strong>${scheduledRulesets.length}</strong> 个计划</span><span><strong>${remoteRulesets.length - scheduledRulesets.length}</strong> 个仅手动</span></div>
      </article>
      <div class="card toolbar ruleset-toolbar">
        <label class="search-wrap">${icon("search")}<input class="search-input" type="search" placeholder="搜索规则集名称、来源或 URL..." value="${escapeHtml(state.rulesetSearch)}" data-ruleset-search /></label>
        <select class="field-select filter-select" data-ruleset-type><option value="all" ${state.rulesetType === "all" ? "selected" : ""}>全部来源</option><option value="remote" ${state.rulesetType === "remote" ? "selected" : ""}>网络规则集</option><option value="manual" ${state.rulesetType === "manual" ? "selected" : ""}>纯手工规则</option></select>
        <span class="ruleset-result-count">${filtered.length} / ${state.rulesets.length}</span>
      </div>
      <article class="card ruleset-table-card">
        ${filtered.length ? `<div class="table-wrap"><table class="data-table ruleset-data-table"><thead><tr><th>规则集</th><th>来源</th><th>格式</th><th>规则数</th><th>更新策略</th><th>最近更新</th><th style="text-align:right">操作</th></tr></thead><tbody>${filtered.map(ruleSetRow).join("")}</tbody></table></div>` : `<div class="empty-state"><div><div class="empty-icon">${icon("search")}</div><h3>没有匹配的规则集</h3><p>调整搜索关键词或来源筛选后重试</p><button class="btn btn-sm" data-action="clear-ruleset-filter">清除筛选</button></div></div>`}
      </article>
    </section>`;
  }

  function ruleSetRow(item) {
    const meta = rulesetTypeMeta(item.type);
    const sourceTitle = item.type === "remote" ? item.url : "Web 中手工维护的 JSON 规则";
    const schedule = item.type === "remote"
      ? `<div class="ruleset-update-plan"><span class="${item.updateInterval !== "manual" ? "is-scheduled" : ""}">${icon("clock")}${escapeHtml(rulesetIntervalLabel(item.updateInterval))}</span><small>${escapeHtml(item.nextUpdate || rulesetNextUpdateLabel(item.updateInterval))}</small></div>`
      : `<div class="ruleset-update-plan"><span>${icon("edit")}手工维护</span><small>直接编辑规则内容</small></div>`;
    return `<tr data-ruleset-row data-id="${escapeHtml(item.id)}"><td><div class="ruleset-name"><span class="ruleset-icon ${item.type}">${icon(meta.iconName)}</span><div><div class="ruleset-main">${escapeHtml(item.name)}</div><div class="ruleset-sub" title="${escapeHtml(sourceTitle || item.source)}">${escapeHtml(item.source)}</div></div></div></td><td><span class="badge ${meta.badge}">${meta.label}</span></td><td><span class="type-badge">${escapeHtml(item.format)}</span></td><td><span class="rule-count">${formatRulesetCount(rulesetCountValue(item))}</span><small class="ruleset-size">${escapeHtml(item.size || "--")}</small></td><td>${schedule}</td><td><div class="ruleset-updated"><strong>${escapeHtml(item.updated || "尚未更新")}</strong><span>${item.type === "remote" ? "网络内容" : "手工内容"}</span></div></td><td><div class="table-actions">${item.type === "remote" ? `<button class="entry-action" data-action="refresh-ruleset" data-id="${escapeHtml(item.id)}" title="立即更新">${icon("rotate")}</button>` : ""}<button class="entry-action" data-modal="resource-ruleset" data-id="${escapeHtml(item.id)}" title="${item.type === "manual" ? "编辑规则内容" : "编辑规则集"}">${icon(item.type === "manual" ? "code" : "edit")}</button></div></td></tr>`;
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
    state.editingProfileId = ["profile-manage", "profile-delete"].includes(type) ? itemId : null;
    state.editingCustomInboundId = type === "inbound" ? itemId : null;
    state.editingOutboundId = type === "outbound" ? itemId : null;
    state.editingRouteRuleId = type === "route" ? itemId : null;
    state.editingRouteRulesetId = type === "route-ruleset" ? itemId : null;
    state.editingSubscriptionId = type === "subscription" ? itemId : null;
    state.editingResourceRulesetId = type === "resource-ruleset" ? itemId : null;
    if (type === "profile-new") content = profileCreateModal();
    else if (type === "profile-manage") content = profileManageModal(itemId);
    else if (type === "profile-delete") content = profileDeleteModal(itemId);
    else if (type === "subscription") content = subscriptionModal(itemId);
    else if (type === "subscription-nodes") content = subscriptionNodesModal(itemId);
    else if (type === "ruleset-hub") content = rulesetHubModal();
    else if (type === "ruleset-hub-preview") content = rulesetHubPreviewModal(itemId);
    else if (type === "resource-ruleset") content = resourceRulesetModal(itemId);
    else if (type === "inbound") content = inboundModal(itemId);
    else if (type === "outbound") content = outboundModal(itemId);
    else if (type === "route") content = routeRuleModal(itemId);
    else if (type === "route-ruleset") content = routeRulesetModal(itemId);
    else if (type === "dns-rule") content = genericModal("新增 DNS 规则", "规则将添加到当前列表末尾", "rule", [
      ["规则名称", "新规则"], ["匹配类型", "domain_suffix", "select", ["domain", "domain_suffix", "ip_cidr", "rule_set", "process_name"]], ["匹配内容", ""], ["目标出站", "节点选择", "select", ["节点选择", "自动选择", "direct", "block"]]
    ]);
    else if (type === "dns") content = genericModal("新增 DNS 服务器", "配置上游 DNS 解析器", "dns", [
      ["服务器标签", "dns-remote"], ["服务器类型", "https", "select", ["https", "tls", "quic", "udp", "local"]], ["服务器地址", "https://1.1.1.1/dns-query"], ["解析路由", "节点选择", "select", ["节点选择", "direct"]]
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
    return getSubscriptionNodes(subscription).map((node, index) => ({
      id: `${subscription.id}-node-${index + 1}`,
      tag: String(node.tag || `未命名节点 ${index + 1}`),
      type: String(node.type || "unknown"),
      badge: outboundMemberInitial(node.tag || node.type),
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
        total: subscriptionNodeCount(subscription),
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

  function routeSegmentedControl(name, value, options, className = "") {
    return `<div class="route-segmented ${className}">${options.map(([optionValue, label, iconName]) => `<label><input type="radio" name="${name}" value="${escapeHtml(optionValue)}" ${String(optionValue) === String(value) ? "checked" : ""} /><span>${iconName ? icon(iconName) : ""}${escapeHtml(label)}</span></label>`).join("")}</div>`;
  }

  function routeEditorRow(label, hint, content, className = "") {
    return `<div class="route-editor-row ${className}"><span class="route-editor-label">${label}${hint ? `<small>${hint}</small>` : ""}</span><div class="route-editor-control">${content}</div></div>`;
  }

  function routeRuleSetChoice(ruleset, selectedIds) {
    return `<label class="route-choice-card"><input type="checkbox" name="route-rule-ruleset" value="${escapeHtml(ruleset.id)}" ${selectedIds.has(ruleset.id) ? "checked" : ""} /><span><strong>${escapeHtml(ruleset.tag)}</strong><small>${escapeHtml(ruleset.type)} · ${escapeHtml(ruleset.type === "inline" ? "source" : ruleset.format)}</small>${icon("check")}</span></label>`;
  }

  function renderRouteRulePayloadFields(type, rule = {}) {
    const payload = rule.payload || "";
    if (type === "rule_set") {
      const selectedIds = new Set(payload.split(",").filter(Boolean));
      return `<div class="route-modal-divider"><span>选择规则集</span><strong data-route-rule-set-count>已选 ${selectedIds.size}</strong></div>${state.routeRuleSets.length ? `<div class="route-choice-grid route-ruleset-choices">${state.routeRuleSets.map((ruleset) => routeRuleSetChoice(ruleset, selectedIds)).join("")}</div>` : `<div class="route-modal-empty">${icon("rules")}<strong>尚未配置规则集</strong><span>请先关闭弹窗，在页面的“规则集”区块中新增。</span></div>`}`;
    }
    if (type === "inbound") {
      return routeEditorRow("入站标签", "payload", `<select class="field-select" name="route-rule-payload">${selectOptionsMarkup([["", "请选择入站"], ...routeInboundOptions()], payload)}</select>`);
    }
    if (type === "clash_mode") {
      return routeEditorRow("Clash 模式", "payload", routeSegmentedControl("route-rule-payload", payload || "global", [["global", "全局"], ["direct", "直连"]], "two"));
    }
    if (type === "ip_is_private") {
      return routeEditorRow("是否为私有 IP", "payload", `<div class="route-inline-toggle"><span>匹配私有网络地址</span><label class="form-switch"><input type="checkbox" name="route-rule-private" value="true" ${payload !== "false" ? "checked" : ""} /><span class="switch"></span></label></div>`);
    }
    if (type === "inline") {
      return routeEditorRow("内联规则", "payload", `<textarea class="field-textarea route-code-input" name="route-rule-payload" spellcheck="false" placeholder='{"domain_suffix":["example.com"]}'>${escapeHtml(payload || "{\n  \"domain_suffix\": [\"example.com\"]\n}")}</textarea>`, "align-start");
    }
    const placeholders = {
      network: "tcp 或 udp",
      protocol: "例如 dns、quic、http",
      domain: "example.com",
      domain_suffix: "example.com",
      domain_keyword: "keyword",
      domain_regex: "^.+\\.example\\.com$",
      source_ip_cidr: "192.168.1.0/24",
      ip_cidr: "172.16.0.0/12",
      source_port: "443",
      source_port_range: "1000:2000",
      port: "443",
      port_range: "80:443",
      process_name: "chrome",
      process_path: "/usr/bin/chrome",
      process_path_regex: ".*chrome$",
    };
    const numeric = ["source_port", "port"].includes(type);
    return routeEditorRow("匹配内容", "payload", `<input class="field-input" name="route-rule-payload" type="${numeric ? "number" : "text"}" value="${escapeHtml(payload)}" placeholder="${escapeHtml(placeholders[type] || "输入匹配内容，多个值用逗号分隔")}" />`);
  }

  function renderRouteRuleActionFields(action, rule = {}) {
    if (action === "route") {
      return routeEditorRow("出站标签", "outbound", `<select class="field-select" name="route-rule-outbound">${selectOptionsMarkup([["", "请选择目标出站"], ...routeOutboundOptions()], rule.outbound || "")}</select>`);
    }
    if (action === "route-options") {
      return routeEditorRow("路由选项", "JSON", `<textarea class="field-textarea route-code-input" name="route-rule-outbound" spellcheck="false">${escapeHtml(rule.outbound || "{\n  \"override_address\": \"1.1.1.1\"\n}")}</textarea>`, "align-start");
    }
    if (action === "reject") {
      return routeEditorRow("拒绝方式", "method", routeSegmentedControl("route-rule-reject", rule.outbound || "default", [["default", "默认"], ["drop", "静默丢弃"], ["reply", "返回响应"]], "three"));
    }
    if (action === "hijack-dns") {
      return `<div class="route-action-empty">${icon("dns")}<strong>DNS 请求将被劫持到 sing-box DNS 模块</strong><span>此动作不需要额外参数。</span></div>`;
    }
    if (action === "sniff") {
      const selected = new Set(rule.sniffer || []);
      const sniffers = ["http", "tls", "quic", "stun", "dns", "bittorrent", "dtls", "ssh", "rdp", "ntp"];
      return routeEditorRow("探测器", "留空表示全部", `<div class="route-sniffer-grid">${sniffers.map((sniffer) => `<label><input type="checkbox" name="route-rule-sniffer" value="${sniffer}" ${selected.has(sniffer) ? "checked" : ""} /><span>${sniffer}${icon("check")}</span></label>`).join("")}</div>`, "align-start");
    }
    return `${routeEditorRow("解析策略", "strategy", `<select class="field-select" name="route-rule-strategy">${selectOptionsMarkup([["", "默认"], ["prefer_ipv4", "IPv4 优先"], ["prefer_ipv6", "IPv6 优先"], ["ipv4_only", "仅 IPv4"], ["ipv6_only", "仅 IPv6"]], rule.strategy || "")}</select>`)}${routeEditorRow("DNS 服务器", "server", `<select class="field-select" name="route-rule-server">${selectOptionsMarkup(routeDnsOptions(), rule.server || "")}</select>`)}`;
  }

  function routeRuleModal(itemId) {
    const existing = state.routeRules.find((rule) => rule.id === itemId && rule.type !== "insertion");
    const rule = existing || { type: "rule_set", payload: "", invert: false, action: "route", outbound: "", sniffer: [], strategy: "", server: "" };
    const body = `<form class="modal-body route-editor-body" id="route-rule-form" data-form="route-rule">
      <div class="route-editor-grid">
        ${routeEditorRow("规则类型", "type", `<select class="field-select" name="route-rule-type" data-route-rule-type>${selectOptionsMarkup(routeRuleTypeOptions, rule.type)}</select>`)}
        ${routeEditorRow("规则动作", "action", routeSegmentedControl("route-rule-action", rule.action, routeRuleActionOptions, "six"))}
        ${routeEditorRow("反向匹配", "invert", `<div class="route-inline-toggle"><span>匹配不符合当前条件的流量</span><label class="form-switch"><input type="checkbox" name="route-rule-invert" value="true" ${rule.invert ? "checked" : ""} /><span class="switch"></span></label></div>`)}
        <div data-route-rule-payload-fields>${renderRouteRulePayloadFields(rule.type, rule)}</div>
        <div class="route-action-panel" data-route-rule-action-fields>${renderRouteRuleActionFields(rule.action, rule)}</div>
      </div>
    </form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" type="submit" form="route-rule-form">${icon(existing ? "save" : "check")}${existing ? "保存规则" : "确认添加"}</button></div>`;
    return modalFrame(existing ? "编辑规则" : "新增规则", "设置匹配条件、执行动作以及对应的出站或 DNS 参数", body, footer, "modal-route-rule");
  }

  function buildRouteRuleConfig(form, itemId = null) {
    const data = new FormData(form);
    const type = String(data.get("route-rule-type") || "rule_set");
    const action = String(data.get("route-rule-action") || "route");
    let payload = String(data.get("route-rule-payload") || "").trim();
    if (type === "rule_set") payload = data.getAll("route-rule-ruleset").map(String).join(",");
    if (type === "ip_is_private") payload = data.get("route-rule-private") === "true" ? "true" : "false";
    let outbound = "";
    if (["route", "route-options"].includes(action)) outbound = String(data.get("route-rule-outbound") || "").trim();
    if (action === "reject") outbound = String(data.get("route-rule-reject") || "default");
    const existing = state.routeRules.find((rule) => rule.id === itemId);
    return {
      id: itemId || `route-rule-${Date.now()}`,
      type,
      enable: existing?.enable ?? true,
      payload,
      invert: data.get("route-rule-invert") === "true",
      action,
      outbound,
      sniffer: data.getAll("route-rule-sniffer").map(String),
      strategy: String(data.get("route-rule-strategy") || ""),
      server: String(data.get("route-rule-server") || ""),
    };
  }

  function routeResourceChoice(resource, selectedPath) {
    return `<label class="route-choice-card"><input type="radio" name="route-ruleset-resource" value="${escapeHtml(resource.id)}" data-route-resource data-resource-name="${escapeHtml(resource.name)}" ${String(resource.id) === String(selectedPath) ? "checked" : ""} /><span><strong>${escapeHtml(resource.name)}</strong><small>${escapeHtml(resource.type)} · ${escapeHtml(resource.format)}</small>${icon("check")}</span></label>`;
  }

  function renderRouteRulesetTypeFields(type, ruleset = {}) {
    if (type === "local") {
      return `<div class="route-modal-divider"><span>选择本地规则资源</span></div>${state.rulesets.length ? `<div class="route-choice-grid">${state.rulesets.map((resource) => routeResourceChoice(resource, ruleset.path || "")).join("")}</div>` : `<div class="route-modal-empty">${icon("folder")}<strong>暂无本地规则资源</strong></div>`}`;
    }
    if (type === "remote") {
      return `<div class="route-ruleset-fields">${routeEditorRow("格式", "format", routeSegmentedControl("route-ruleset-format", ruleset.format || "binary", [["source", "源文件"], ["binary", "二进制"]], "two"))}${routeEditorRow("远程链接", "url", `<input class="field-input" name="route-ruleset-url" type="url" value="${escapeHtml(ruleset.url || "")}" placeholder="https://example.com/rules.srs" />`)}${routeEditorRow("下载方式", "download_detour", `<select class="field-select" name="route-ruleset-detour">${selectOptionsMarkup([["", "默认网络"], ...routeOutboundOptions()], ruleset.download_detour || "")}</select>`)}${routeEditorRow("更新间隔", "update_interval", `<input class="field-input" name="route-ruleset-interval" value="${escapeHtml(ruleset.update_interval || "1d")}" placeholder="1d" />`)}</div>`;
    }
    return routeEditorRow("内联规则", "JSON", `<textarea class="field-textarea route-code-input route-inline-rules" name="route-ruleset-rules" spellcheck="false">${escapeHtml(ruleset.rules || "{\n  \"version\": 1,\n  \"rules\": []\n}")}</textarea>`, "align-start");
  }

  function routeRulesetModal(itemId) {
    const existing = state.routeRuleSets.find((ruleset) => ruleset.id === itemId);
    const ruleset = existing || { tag: "", type: "local", format: "binary", url: "", download_detour: "", update_interval: "", path: "", rules: "" };
    const body = `<form class="modal-body route-editor-body" id="route-ruleset-form" data-form="route-ruleset">
      <div class="route-editor-grid">
        ${routeEditorRow("名称", "tag", `<input class="field-input" name="route-ruleset-tag" value="${escapeHtml(ruleset.tag)}" placeholder="例如 GeoSite-CN" required />`)}
        ${routeEditorRow("来源类型", "type", routeSegmentedControl("route-ruleset-type", ruleset.type, [["inline", "内联", "code"], ["local", "本地", "folder"], ["remote", "远程", "download"]], "three"))}
        <div data-route-ruleset-type-fields>${renderRouteRulesetTypeFields(ruleset.type, ruleset)}</div>
      </div>
    </form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" type="submit" form="route-ruleset-form">${icon(existing ? "save" : "check")}${existing ? "保存规则集" : "确认添加"}</button></div>`;
    return modalFrame(existing ? "编辑规则集" : "新增规则集", "配置规则集名称、来源与更新方式", body, footer, "modal-route-rule");
  }

  function buildRouteRulesetConfig(form, itemId = null) {
    const data = new FormData(form);
    const type = String(data.get("route-ruleset-type") || "local");
    const resource = state.rulesets.find((item) => String(item.id) === String(data.get("route-ruleset-resource")));
    return {
      id: itemId || `route-ruleset-${Date.now()}`,
      tag: String(data.get("route-ruleset-tag") || resource?.name || "新规则集").trim(),
      type,
      format: type === "inline" ? "source" : type === "local" ? resource?.format || "binary" : String(data.get("route-ruleset-format") || "binary"),
      url: type === "remote" ? String(data.get("route-ruleset-url") || "").trim() : "",
      download_detour: type === "remote" ? String(data.get("route-ruleset-detour") || "") : "",
      update_interval: type === "remote" ? String(data.get("route-ruleset-interval") || "").trim() : "",
      path: type === "local" ? String(data.get("route-ruleset-resource") || "") : "",
      rules: type === "inline" ? String(data.get("route-ruleset-rules") || "").trim() : "",
    };
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

  function validateSubscriptionJson(json) {
    try {
      const config = JSON.parse(json);
      const nodes = Array.isArray(config)
        ? config
        : config && typeof config === "object" && Array.isArray(config.outbounds)
          ? config.outbounds
          : config && typeof config === "object" && Array.isArray(config.nodes)
            ? config.nodes
            : null;
      if (!nodes) {
        return { valid: false, message: "JSON 必须是节点数组，或包含 outbounds 数组的完整配置对象。" };
      }
      if (!nodes.length) {
        return { valid: false, message: "JSON 中没有可导入的节点，请至少添加一个出站对象。" };
      }
      const invalidIndex = nodes.findIndex((node) => !node || typeof node !== "object" || Array.isArray(node) || typeof node.type !== "string" || !node.type.trim() || typeof node.tag !== "string" || !node.tag.trim());
      if (invalidIndex >= 0) {
        return { valid: false, message: `第 ${invalidIndex + 1} 个节点必须是包含非空 type 和 tag 的对象。` };
      }
      const tags = nodes.map((node) => node.tag.trim());
      const duplicateTag = tags.find((tag, index) => tags.indexOf(tag) !== index);
      if (duplicateTag) {
        return { valid: false, message: `节点标签不能重复：${duplicateTag}` };
      }
      const name = !Array.isArray(config) && typeof config.name === "string" ? config.name.trim() : "";
      const protocols = new Set(nodes.map((node) => node.type.trim()));
      return {
        valid: true,
        config,
        nodes,
        name,
        message: `JSON 有效 · 将导入 ${nodes.length} 个节点 · ${protocols.size} 种协议`,
      };
    } catch (error) {
      return { valid: false, message: `JSON 语法错误：${error.message}` };
    }
  }

  function subscriptionValidationMarkup(result) {
    return `<div class="custom-json-validation ${result.valid ? "valid" : "invalid"}" id="subscription-json-validation">${icon(result.valid ? "check" : "info")}<span>${escapeHtml(result.message)}</span></div>`;
  }

  function subscriptionJsonFor(item) {
    if (item?.sourceJson) return item.sourceJson;
    if (item) return JSON.stringify({ name: item.name, outbounds: getSubscriptionNodes(item) }, null, 2);
    return subscriptionJsonExample;
  }

  function subscriptionModal(itemId) {
    const item = state.subscriptions.find((subscription) => subscription.id === itemId);
    const json = subscriptionJsonFor(item);
    const validation = validateSubscriptionJson(json);
    const body = `<form class="modal-body subscription-json-body" id="subscription-form" data-form="subscription">
      <div class="subscription-json-intro">${icon("code")}<div><strong>粘贴完整节点 JSON</strong><span>支持完整 sing-box 配置的 <code>outbounds</code>，也支持直接粘贴节点数组。</span></div></div>
      <div class="custom-json-editor">
        <div class="custom-json-toolbar"><div><span class="custom-json-language">JSON</span><span>全部订阅节点</span></div><div class="inline-actions"><button class="editor-action" type="button" data-action="reset-subscription-json">示例</button><button class="editor-action" type="button" data-action="format-subscription-json">${icon("sparkles")}格式化</button></div></div>
        <textarea class="custom-json-textarea subscription-json-textarea ${validation.valid ? "" : "invalid"}" name="subscription-json" data-subscription-json aria-describedby="subscription-json-validation" aria-invalid="${!validation.valid}" spellcheck="false">${escapeHtml(json)}</textarea>
      </div>
      ${subscriptionValidationMarkup(validation)}
      <div class="info-callout custom-json-help">${icon("info")}<span>保存时会整体替换该订阅的节点集合，JSON 中除 <code>name</code> 和 <code>outbounds</code> 外的其他顶层配置不会进入订阅卡片。</span></div>
    </form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" id="subscription-submit" type="submit" form="subscription-form" ${validation.valid ? "" : "disabled"}>${icon(item ? "save" : "plus")}${item ? "保存全部节点" : "导入全部节点"}</button></div>`;
    return modalFrame(item ? "编辑 JSON 订阅" : "导入 JSON 订阅", item ? `整体编辑 ${escapeHtml(item.name)} 的全部节点` : "通过 JSON 一次添加订阅中的所有节点", body, footer, "modal-wide modal-subscription-json");
  }

  function updateSubscriptionValidation(textarea) {
    const result = validateSubscriptionJson(textarea.value);
    textarea.classList.toggle("invalid", !result.valid);
    textarea.setAttribute("aria-invalid", String(!result.valid));
    const feedback = document.querySelector("#subscription-json-validation");
    if (feedback) feedback.outerHTML = subscriptionValidationMarkup(result);
    const submit = document.querySelector("#subscription-submit");
    if (submit) submit.disabled = !result.valid;
    return result;
  }

  function subscriptionNodeEndpoint(node) {
    const server = node.server || node.address || node.server_address || "";
    const port = node.server_port ?? node.port ?? "";
    if (server) return `${server}${port !== "" ? `:${port}` : ""}`;
    if (Array.isArray(node.outbounds)) return `${node.outbounds.length} 个引用出站`;
    return "未设置远程地址";
  }

  function subscriptionNodeRow(node, index) {
    return `<div class="subscription-node-row">
      <span class="subscription-node-index">${String(index + 1).padStart(2, "0")}</span>
      <span class="subscription-node-logo">${escapeHtml(outboundMemberInitial(node.tag || node.type))}</span>
      <div class="subscription-node-copy"><strong title="${escapeHtml(node.tag)}">${escapeHtml(node.tag)}</strong><small>订阅节点 #${index + 1}</small></div>
      <span class="type-badge">${escapeHtml(node.type)}</span>
      <code title="${escapeHtml(subscriptionNodeEndpoint(node))}">${escapeHtml(subscriptionNodeEndpoint(node))}</code>
    </div>`;
  }

  function subscriptionNodesModal(itemId) {
    const item = state.subscriptions.find((subscription) => subscription.id === itemId);
    if (!item) return genericModal("订阅不存在", "找不到对应的订阅节点集合", "missing-subscription", []);
    const nodes = getSubscriptionNodes(item);
    const protocols = subscriptionProtocols(item);
    const body = `<div class="modal-body subscription-nodes-body">
      <div class="subscription-node-overview"><div class="provider-logo ${item.tone === "cyan" ? "cyan" : ""}">${escapeHtml(item.logo)}</div><div><strong>${escapeHtml(item.name)}</strong><span>${nodes.length} 个节点 · ${protocols.length} 种协议 · 完整来自 JSON</span></div><div class="subscription-node-protocols">${protocols.map((type) => `<span class="type-badge">${escapeHtml(type)}</span>`).join("")}</div></div>
      <div class="subscription-node-list-head"><span>#</span><span>节点</span><span>协议</span><span>服务器</span></div>
      <div class="subscription-node-list">${nodes.map(subscriptionNodeRow).join("")}</div>
    </div>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="copy-subscription-json" data-id="${escapeHtml(item.id)}">${icon("copy")}复制全部 JSON</button><button class="btn btn-primary" data-action="close-modal">完成</button></div>`;
    return modalFrame("全部订阅节点", `${escapeHtml(item.name)} 中的所有节点`, body, footer, "modal-wide modal-subscription-nodes");
  }

  function filteredRulesetHubItems() {
    const keyword = state.rulesetHubSearch.trim().toLowerCase();
    return rulesetHubItems.filter((item) => {
      const categoryMatched = state.rulesetHubCategory === "all" || item.category === state.rulesetHubCategory;
      const keywordMatched = !keyword || `${item.name} ${item.source} ${item.description} ${item.category}`.toLowerCase().includes(keyword);
      return categoryMatched && keywordMatched;
    });
  }

  function rulesetHubUrl(item, format) {
    const repository = item.category === "geoip" ? "sing-geoip" : "sing-geosite";
    const suffix = format === "source" ? "json" : "srs";
    return `https://raw.githubusercontent.com/SagerNet/${repository}/rule-set/${item.name}.${suffix}`;
  }

  function isRulesetHubItemAdded(itemId, format) {
    return state.rulesets.some((item) => item.hubId === itemId && item.format === format);
  }

  function rulesetHubPreviewContent(item) {
    const rules = item.category === "geoip"
      ? [{ ip_cidr: ["203.0.113.0/24", "2001:db8::/32"] }]
      : [{ domain_suffix: [`${item.id.replace(/^geosite-/, "")}.example`, "example.net"] }];
    return JSON.stringify({ version: 3, rules }, null, 2);
  }

  function rulesetHubCard(item) {
    const sourceAdded = isRulesetHubItemAdded(item.id, "source");
    const binaryAdded = isRulesetHubItemAdded(item.id, "binary");
    return `<article class="ruleset-hub-card">
      <div class="ruleset-hub-card-head"><span class="ruleset-hub-card-icon ${item.category}">${icon(item.category === "geoip" ? "globe" : "rules")}</span><div><strong>${escapeHtml(item.name)}</strong><span>${escapeHtml(item.source)}</span></div><span class="type-badge">${escapeHtml(item.category)}</span></div>
      <p>${escapeHtml(item.description)}</p>
      <div class="ruleset-hub-card-meta"><span>${icon("database")} ${formatRulesetCount(item.count)} 条</span><button data-modal="ruleset-hub-preview" data-id="${escapeHtml(item.id)}">预览</button></div>
      <div class="ruleset-hub-card-actions"><button class="btn btn-sm btn-quiet" data-action="import-hub-ruleset" data-id="${escapeHtml(item.id)}" data-format="source" ${sourceAdded ? "disabled" : ""}>${icon(sourceAdded ? "check" : "plus")}${sourceAdded ? "source 已添加" : "添加 source"}</button><button class="btn btn-sm ${binaryAdded ? "btn-quiet" : "btn-primary"}" data-action="import-hub-ruleset" data-id="${escapeHtml(item.id)}" data-format="binary" ${binaryAdded ? "disabled" : ""}>${icon(binaryAdded ? "check" : "download")}${binaryAdded ? "binary 已添加" : "添加 binary"}</button></div>
    </article>`;
  }

  function rulesetHubResultsMarkup() {
    const items = filteredRulesetHubItems();
    return `<div class="ruleset-hub-results-head"><span>显示 ${items.length} / ${rulesetHubItems.length} 个模板</span><small>中心更新于 ${escapeHtml(state.rulesetHubUpdated)}</small></div>${items.length ? `<div class="ruleset-hub-grid">${items.map(rulesetHubCard).join("")}</div>` : `<div class="ruleset-hub-empty">${icon("search")}<strong>没有匹配的规则模板</strong><span>尝试更换关键词或分类。</span></div>`}`;
  }

  function updateRulesetHubResults() {
    const container = document.querySelector("[data-ruleset-hub-results]");
    if (container) container.innerHTML = rulesetHubResultsMarkup();
  }

  function rulesetHubModal() {
    const body = `<div class="modal-body ruleset-hub-body">
      <div class="ruleset-hub-toolbar"><label class="search-wrap">${icon("search")}<input class="search-input" data-ruleset-hub-search value="${escapeHtml(state.rulesetHubSearch)}" placeholder="搜索名称、分类或说明..." /></label><select class="field-select" data-ruleset-hub-category><option value="all" ${state.rulesetHubCategory === "all" ? "selected" : ""}>全部分类</option><option value="geosite" ${state.rulesetHubCategory === "geosite" ? "selected" : ""}>GeoSite</option><option value="geoip" ${state.rulesetHubCategory === "geoip" ? "selected" : ""}>GeoIP</option></select><button class="btn btn-sm" data-action="refresh-ruleset-hub">${icon("rotate")}刷新中心</button></div>
      <div class="ruleset-hub-callout">${icon("database")}<div><strong>规则集中心</strong><span>从公共模板快速创建网络规则集；导入后可单独设置定时更新周期。</span></div></div>
      <div data-ruleset-hub-results>${rulesetHubResultsMarkup()}</div>
    </div>`;
    const footer = `<div class="modal-footer"><span class="modal-footer-note">source 可查看源码，binary 体积更小</span><button class="btn btn-primary" data-action="close-modal">完成</button></div>`;
    return modalFrame("规则集中心", "参照 GUI.for.SingBox 的 Ruleset Hub 组织模板并快速导入", body, footer, "modal-ruleset-hub");
  }

  function rulesetHubPreviewModal(itemId) {
    const item = rulesetHubItems.find((candidate) => candidate.id === itemId);
    if (!item) return modalFrame("模板不存在", "规则集中心中未找到该项目", `<div class="modal-body"></div>`);
    const sourceAdded = isRulesetHubItemAdded(item.id, "source");
    const binaryAdded = isRulesetHubItemAdded(item.id, "binary");
    const body = `<div class="modal-body ruleset-hub-preview-body"><div class="ruleset-preview-summary"><span class="ruleset-hub-card-icon ${item.category}">${icon(item.category === "geoip" ? "globe" : "rules")}</span><div><strong>${escapeHtml(item.name)}</strong><span>${escapeHtml(item.description)} · ${formatRulesetCount(item.count)} 条规则</span></div><span class="type-badge">${escapeHtml(item.category)}</span></div><div class="custom-json-editor"><div class="custom-json-toolbar"><div><span class="custom-json-language">JSON</span><span>source 格式结构预览</span></div></div><pre class="ruleset-preview-code">${escapeHtml(rulesetHubPreviewContent(item))}</pre></div><div class="info-callout custom-json-help">${icon("info")}<span>这是结构预览；实际导入由网络 URL 获取完整的 ${formatRulesetCount(item.count)} 条规则。</span></div></div>`;
    const footer = `<div class="modal-footer"><button class="btn" data-modal="ruleset-hub">${icon("arrowDown")}返回中心</button><button class="btn btn-quiet" data-action="import-hub-ruleset" data-id="${escapeHtml(item.id)}" data-format="source" ${sourceAdded ? "disabled" : ""}>${sourceAdded ? "source 已添加" : "添加 source"}</button><button class="btn btn-primary" data-action="import-hub-ruleset" data-id="${escapeHtml(item.id)}" data-format="binary" ${binaryAdded ? "disabled" : ""}>${binaryAdded ? "binary 已添加" : "添加 binary"}</button></div>`;
    return modalFrame("规则模板预览", escapeHtml(item.source), body, footer, "modal-wide modal-ruleset-preview");
  }

  function countManualRules(config) {
    return config.rules.reduce((total, rule) => total + Object.values(rule).reduce((count, value) => {
      if (Array.isArray(value)) return count + value.length;
      return value === null || value === undefined || value === "" ? count : count + 1;
    }, 0), 0);
  }

  function validateManualRulesetJson(json) {
    try {
      const config = JSON.parse(json);
      if (!config || typeof config !== "object" || Array.isArray(config)) {
        return { valid: false, message: "手工规则必须是一个 JSON 对象。" };
      }
      if (!Number.isInteger(config.version) || config.version < 1) {
        return { valid: false, message: "手工规则必须包含有效的数字 version。" };
      }
      if (!Array.isArray(config.rules) || !config.rules.length) {
        return { valid: false, message: "rules 必须是非空数组，请至少输入一条规则。" };
      }
      const invalidIndex = config.rules.findIndex((rule) => !rule || typeof rule !== "object" || Array.isArray(rule) || !Object.keys(rule).length);
      if (invalidIndex >= 0) {
        return { valid: false, message: `第 ${invalidIndex + 1} 条规则必须是非空对象。` };
      }
      const count = countManualRules(config);
      if (!count) return { valid: false, message: "规则对象中没有可用的匹配内容。" };
      return { valid: true, config, count, message: `JSON 有效 · ${config.rules.length} 个规则块 · ${count} 条规则信息` };
    } catch (error) {
      return { valid: false, message: `JSON 语法错误：${error.message}` };
    }
  }

  function manualRulesetValidationMarkup(result) {
    return `<div class="custom-json-validation ${result.valid ? "valid" : "invalid"}" id="manual-ruleset-validation">${icon(result.valid ? "check" : "info")}<span>${escapeHtml(result.message)}</span></div>`;
  }

  function renderManualRulesetEditor(content = manualRulesetExample) {
    const validation = validateManualRulesetJson(content);
    return `<div class="ruleset-manual-editor"><div class="custom-json-editor"><div class="custom-json-toolbar"><div><span class="custom-json-language">JSON</span><span>sing-box source 规则</span></div><div class="inline-actions"><button class="editor-action" type="button" data-action="reset-manual-ruleset">示例</button><button class="editor-action" type="button" data-action="format-manual-ruleset">${icon("sparkles")}格式化</button></div></div><textarea class="custom-json-textarea manual-ruleset-textarea ${validation.valid ? "" : "invalid"}" name="manual-ruleset-json" data-manual-ruleset-json aria-describedby="manual-ruleset-validation" aria-invalid="${!validation.valid}" spellcheck="false">${escapeHtml(content)}</textarea></div>${manualRulesetValidationMarkup(validation)}<div class="info-callout custom-json-help">${icon("info")}<span>直接填写完整 source 规则对象；保存后整个 JSON 会作为一个纯手工规则集维护，不会访问文件系统或网络。</span></div></div>`;
  }

  function updateManualRulesetValidation(textarea) {
    const result = validateManualRulesetJson(textarea.value);
    textarea.classList.toggle("invalid", !result.valid);
    textarea.setAttribute("aria-invalid", String(!result.valid));
    const feedback = document.querySelector("#manual-ruleset-validation");
    if (feedback) feedback.outerHTML = manualRulesetValidationMarkup(result);
    const submit = document.querySelector("#resource-ruleset-submit");
    if (submit) submit.disabled = !result.valid;
    return result;
  }

  function resourceRulesetTypeFields(type, item = {}) {
    if (type === "manual") return renderManualRulesetEditor(item.content || manualRulesetExample);
    return `<div class="resource-ruleset-remote-fields">
      ${routeEditorRow("保存格式", "format", routeSegmentedControl("resource-ruleset-format", item.format || "binary", [["source", "源文件", "code"], ["binary", "二进制", "database"]], "two"))}
      ${routeEditorRow("网络地址", "url", `<input class="field-input" name="resource-ruleset-url" type="url" value="${escapeHtml(item.url || "")}" placeholder="https://example.com/rules.srs" required />`)}
      ${routeEditorRow("定时更新", "update interval", `<select class="field-select" name="resource-ruleset-interval">${selectOptionsMarkup(rulesetUpdateIntervals, item.updateInterval || "1d")}</select>`)}
      <div class="ruleset-automation-callout">${icon("clock")}<div><strong>自动更新计划</strong><span>浏览器原型会保存所选周期并显示下一次计划；接入后端后由调度任务按此周期下载和校验规则。</span></div></div>
    </div>`;
  }

  function resourceRulesetModal(itemId) {
    const item = state.rulesets.find((ruleset) => String(ruleset.id) === String(itemId));
    const ruleset = item || { name: "", type: "remote", format: "binary", url: "", updateInterval: "1d", content: manualRulesetExample };
    const validation = ruleset.type === "manual" ? validateManualRulesetJson(ruleset.content || manualRulesetExample) : { valid: true };
    const body = `<form class="modal-body resource-ruleset-body" id="resource-ruleset-form" data-form="resource-ruleset"><div class="ruleset-kind-section"><span class="field-label">规则集来源</span>${routeSegmentedControl("resource-ruleset-type", ruleset.type, [["remote", "网络规则集", "globe"], ["manual", "纯手工规则", "code"]], "two")}</div><div class="route-editor-grid resource-ruleset-grid">${routeEditorRow("规则集名称", "name", `<input class="field-input" name="resource-ruleset-name" value="${escapeHtml(ruleset.name)}" placeholder="例如 geosite-cn" required />`)}<div data-resource-ruleset-type-fields data-manual-content="${escapeHtml(ruleset.content || manualRulesetExample)}">${resourceRulesetTypeFields(ruleset.type, ruleset)}</div></div></form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" id="resource-ruleset-submit" type="submit" form="resource-ruleset-form" ${validation.valid ? "" : "disabled"}>${icon(item ? "save" : "plus")}${item ? "保存规则集" : "添加规则集"}</button></div>`;
    return modalFrame(item ? "编辑规则集" : "添加规则集", ruleset.type === "manual" ? "直接维护完整的手工规则 JSON" : "配置网络地址与定时更新周期", body, footer, "modal-wide modal-resource-ruleset");
  }

  function formatRulesetTextSize(text) {
    const bytes = new TextEncoder().encode(text).length;
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  }

  function estimateRulesetSize(count, format) {
    const bytes = count * (format === "binary" ? 28 : 52);
    if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
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

  function nextProfileName() {
    const index = state.profiles.length;
    return index < 26 ? `自定义配置 ${String.fromCharCode(65 + index)}` : `自定义配置 ${index + 1}`;
  }

  function validateProfileJson(json) {
    try {
      const config = JSON.parse(json);
      if (!config || typeof config !== "object" || Array.isArray(config)) {
        return { valid: false, message: "完整配置必须是一个 JSON 对象。" };
      }
      const knownKeys = ["log", "dns", "inbounds", "outbounds", "route", "experimental"];
      if (!knownKeys.some((key) => Object.hasOwn(config, key))) {
        return { valid: false, message: "没有找到 sing-box 配置字段，请检查输入内容。" };
      }
      if (config.inbounds !== undefined && !Array.isArray(config.inbounds)) {
        return { valid: false, message: "inbounds 必须是数组。" };
      }
      if (config.outbounds !== undefined && !Array.isArray(config.outbounds)) {
        return { valid: false, message: "outbounds 必须是数组。" };
      }
      if (config.route !== undefined && (!config.route || typeof config.route !== "object" || Array.isArray(config.route))) {
        return { valid: false, message: "route 必须是对象。" };
      }
      if (config.dns !== undefined && (!config.dns || typeof config.dns !== "object" || Array.isArray(config.dns))) {
        return { valid: false, message: "dns 必须是对象。" };
      }
      const inboundTotal = config.inbounds?.length || 0;
      const outboundTotal = config.outbounds?.length || 0;
      const ruleTotal = Array.isArray(config.route?.rules) ? config.route.rules.length : 0;
      return { valid: true, config, message: `JSON 有效 · ${inboundTotal} 个入站 · ${outboundTotal} 个出站 · ${ruleTotal} 条路由规则` };
    } catch (error) {
      return { valid: false, message: `JSON 语法错误：${error.message}` };
    }
  }

  function profileJsonValidationMarkup(result) {
    return `<div class="custom-json-validation ${result.valid ? "valid" : "invalid"}" id="profile-json-validation">${icon(result.valid ? "check" : "info")}<span>${escapeHtml(result.message)}</span></div>`;
  }

  function updateProfileJsonValidation(textarea) {
    const result = validateProfileJson(textarea.value);
    textarea.classList.toggle("invalid", !result.valid);
    textarea.setAttribute("aria-invalid", String(!result.valid));
    const feedback = document.querySelector("#profile-json-validation");
    if (feedback) feedback.outerHTML = profileJsonValidationMarkup(result);
    const mode = document.querySelector('input[name="profile-create-mode"]:checked')?.value;
    const submit = document.querySelector("#profile-create-submit");
    if (submit && mode === "json") submit.disabled = !result.valid;
    return result;
  }

  function profileModeNote(mode) {
    const notes = {
      blank: ["从空白开始", "创建不含入站、出站和路由规则的新配置。"],
      copy: ["复制当前配置", `完整复制“${activeProfile().name}”的当前草稿，之后可独立编辑。`],
      json: ["导入完整 JSON", "粘贴一个完整 sing-box 配置对象，并将可识别内容载入各配置页。"],
    };
    const [title, description] = notes[mode] || notes.blank;
    return `${icon(mode === "json" ? "code" : mode === "copy" ? "copy" : "sparkles")}<div><strong>${escapeHtml(title)}</strong><span>${escapeHtml(description)}</span></div>`;
  }

  function profileCreateModal() {
    const name = nextProfileName();
    const validation = validateProfileJson(profileJsonExample);
    const body = `<form class="modal-body profile-create-body" id="profile-create-form" data-form="profile-create">
      <label class="form-field full"><span class="field-label">配置名称<small>用于列表与页面标题</small></span><input class="field-input" name="profile-name" value="${escapeHtml(name)}" maxlength="48" required /></label>
      <div class="profile-create-mode"><span class="field-label">创建方式</span>${routeSegmentedControl("profile-create-mode", "blank", [["blank", "空白配置", "sparkles"], ["copy", "复制当前", "copy"], ["json", "粘贴 JSON", "code"]])}</div>
      <div class="profile-mode-note" data-profile-mode-note>${profileModeNote("blank")}</div>
      <div class="profile-json-section" data-profile-json-section hidden>
        <div class="custom-json-editor"><div class="custom-json-toolbar"><div><span class="custom-json-language">JSON</span><span>完整 sing-box 配置</span></div><button class="editor-action" type="button" data-action="format-profile-json">${icon("sparkles")}格式化</button></div><textarea class="custom-json-textarea profile-json-textarea" name="profile-json" data-profile-json aria-describedby="profile-json-validation" spellcheck="false">${escapeHtml(profileJsonExample)}</textarea></div>
        ${profileJsonValidationMarkup(validation)}
      </div>
    </form>`;
    const footer = `<div class="modal-footer"><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" id="profile-create-submit" type="submit" form="profile-create-form">${icon("plus")}创建并打开</button></div>`;
    return modalFrame("新增配置", "创建独立的 sing-box 配置草稿", body, footer, "modal-wide modal-profile");
  }

  function profileManageModal(itemId) {
    const profile = state.profiles.find((item) => item.id === itemId);
    if (!profile) return modalFrame("配置不存在", "无法找到要管理的配置", `<div class="modal-body"></div>`);
    const draft = profile.id === state.activeProfileId ? captureProfileDraft() : profile.draft;
    const inboundTotal = (draft.builtInInbounds ? 3 : 0) + draft.customInbounds.length + draft.formInbounds.length;
    const ruleTotal = draft.routeRules.filter((rule) => rule.type !== "insertion").length;
    const body = `<form class="modal-body profile-manage-body" id="profile-manage-form" data-form="profile-manage" data-id="${escapeHtml(profile.id)}">
      <label class="form-field full"><span class="field-label">配置名称<small>保存后立即更新侧栏和标题</small></span><input class="field-input" name="profile-name" value="${escapeHtml(profile.name)}" maxlength="48" required /></label>
      <div class="profile-manage-summary"><div><strong>${inboundTotal}</strong><span>入站</span></div><div><strong>${draft.outbounds.length}</strong><span>出站</span></div><div><strong>${ruleTotal}</strong><span>路由规则</span></div><div><strong>v${profile.version}</strong><span>配置版本</span></div></div>
      <div class="info-callout">${icon("info")}<span>${escapeHtml(profile.source)} · 最后保存 ${escapeHtml(profile.lastSaved || draft.lastSaved)}</span></div>
    </form>`;
    const deleteDisabled = state.profiles.length === 1;
    const footer = `<div class="modal-footer profile-manage-footer"><button class="btn btn-danger" type="button" data-modal="profile-delete" data-id="${escapeHtml(profile.id)}" ${deleteDisabled ? "disabled title=\"至少保留一个配置\"" : ""}>${icon("trash")}删除配置</button><span class="modal-footer-spacer"></span><button class="btn" data-action="close-modal">取消</button><button class="btn btn-primary" type="submit" form="profile-manage-form">${icon("save")}保存名称</button></div>`;
    return modalFrame("管理配置", profile.id === state.activeProfileId ? "当前正在编辑此配置" : "重命名或删除配置", body, footer, "modal-profile-manage");
  }

  function profileDeleteModal(itemId) {
    const profile = state.profiles.find((item) => item.id === itemId);
    if (!profile) return modalFrame("配置不存在", "无法找到要删除的配置", `<div class="modal-body"></div>`);
    const body = `<div class="modal-body profile-delete-body"><div class="profile-delete-icon">${icon("trash")}</div><div><strong>确定删除“${escapeHtml(profile.name)}”吗？</strong><p>该配置的全部草稿内容会从浏览器中移除，此操作无法撤销。</p></div></div>`;
    const footer = `<div class="modal-footer"><button class="btn" data-modal="profile-manage" data-id="${escapeHtml(profile.id)}">返回</button><button class="btn btn-danger" data-action="confirm-delete-profile" data-id="${escapeHtml(profile.id)}">${icon("trash")}确认删除</button></div>`;
    return modalFrame("删除配置", "请确认本次操作", body, footer, "modal-profile-delete");
  }

  function jsonModal() {
    const body = `<div class="modal-body">${renderCodePreview(true)}<div class="info-callout" style="margin-top:12px">${icon("check")}<span>配置结构检查通过，共包含 ${inboundCount()} 个入站、${state.outbounds.length} 个出站与 ${routeRuleCount()} 条路由规则。</span></div></div>`;
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
    state.editingProfileId = null;
    state.editingCustomInboundId = null;
    state.editingOutboundId = null;
    state.editingRouteRuleId = null;
    state.editingRouteRulesetId = null;
    state.editingSubscriptionId = null;
    state.editingResourceRulesetId = null;
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
    const route = page === "config" ? `#/${page}/${state.activeProfileId}/${state.activeConfig}` : `#/${page}`;
    window.history.pushState({ page, profileId: state.activeProfileId, configId: state.activeConfig }, "", route);
    render();
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function switchProfile(profileId) {
    const profile = state.profiles.find((item) => item.id === profileId);
    if (!profile) return;
    if (profile.id !== state.activeProfileId) {
      saveActiveProfileDraft();
      state.activeProfileId = profile.id;
      loadProfileDraft(profile);
      persistProfiles();
    }
    navigate("config", state.activeConfig);
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
    const routeRemoveTarget = event.target.closest("[data-route-remove]");
    if (routeRemoveTarget) {
      const id = routeRemoveTarget.dataset.id;
      if (routeRemoveTarget.dataset.routeRemove === "rule") {
        const rule = state.routeRules.find((item) => item.id === id);
        state.routeRules = state.routeRules.filter((item) => item.id !== id);
        state.dirty = true;
        render();
        showToast("路由规则已移除", `${rule?.type || "规则"} 已从配置草稿移除。`, "info");
      } else {
        const ruleset = state.routeRuleSets.find((item) => item.id === id);
        const referenceCount = state.routeRules.filter((rule) => rule.type === "rule_set" && rule.payload.split(",").includes(id)).length;
        state.routeRuleSets = state.routeRuleSets.filter((item) => item.id !== id);
        state.dirty = true;
        render();
        showToast("规则集已移除", referenceCount ? `${ruleset?.tag || "规则集"} 仍被 ${referenceCount} 条规则引用，请继续检查。` : `${ruleset?.tag || "规则集"} 已从配置草稿移除。`, "info");
      }
      return;
    }

    const routeToggleTarget = event.target.closest("[data-route-toggle]");
    if (routeToggleTarget) {
      const key = routeToggleTarget.dataset.routeToggle;
      state[key] = !state[key];
      state.dirty = true;
      render();
      showToast("路由设置已更新", "该更改已在当前配置草稿中生效。", "info");
      return;
    }

    const proxyNodeTarget = event.target.closest("[data-proxy-node]");
    if (proxyNodeTarget) {
      const group = state.proxyGroups.find((item) => item.id === proxyNodeTarget.dataset.group);
      const member = group?.members.find((item) => item.id === proxyNodeTarget.dataset.proxyNode);
      if (!group || !member) return;
      group.selected = member.id;
      render();
      showToast("节点已切换", `${group.name} → ${member.name}`, "success");
      return;
    }

    const proxyGroupTarget = event.target.closest("[data-proxy-group]");
    if (proxyGroupTarget) {
      const group = state.proxyGroups.find((item) => item.id === proxyGroupTarget.dataset.proxyGroup);
      if (!group) return;
      group.expanded = !group.expanded;
      render();
      return;
    }

    const command = event.target.closest("[data-command-page]");
    if (command) {
      navigate(command.dataset.commandPage, command.dataset.commandConfig || null);
      return;
    }

    const profileTarget = event.target.closest("[data-profile]");
    if (profileTarget) {
      switchProfile(profileTarget.dataset.profile);
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

    const accentTarget = event.target.closest(".color-swatch[data-accent]");
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
        showToast("核心重启完成", `sing-box 已使用“${activeProfile().name}”恢复运行。`);
      }, 900);
    } else if (action === "refresh-dashboard") {
      showToast("状态已刷新", "流量、连接与核心状态均已更新。", "info");
    } else if (action === "test-proxy-group") {
      const group = state.proxyGroups.find((item) => item.id === actionTarget.dataset.id);
      if (!group) return;
      refreshProxyLatencies([group]);
      render();
      showToast("延迟测试完成", `${group.name} 的 ${group.members.length} 个候选节点已更新。`, "success");
    } else if (action === "test-all-proxies") {
      if (state.proxyTesting) return;
      state.proxyTesting = true;
      render();
      window.setTimeout(() => {
        refreshProxyLatencies(state.proxyGroups);
        state.proxyTesting = false;
        render();
        showToast("全部测速完成", "所有策略组的节点延迟已更新。", "success");
      }, 700);
    } else if (action === "save-config") {
      const name = String(state.configFields.profileName || "").trim();
      if (!name) {
        showToast("配置名称不能为空", "请在通用配置中填写配置名称。", "info");
        document.querySelector('[data-config-field="profileName"]')?.focus();
        return;
      }
      const duplicate = state.profiles.find((profile) => profile.id !== state.activeProfileId && profile.name.toLowerCase() === name.toLowerCase());
      if (duplicate) {
        showToast("配置名称已存在", `“${duplicate.name}”已在配置列表中，请换一个名称。`, "info");
        document.querySelector('[data-config-field="profileName"]')?.focus();
        return;
      }
      state.dirty = false;
      state.lastSaved = new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
      const profile = activeProfile();
      profile.name = name;
      profile.lastSaved = state.lastSaved;
      profile.version = Number(profile.version || 0) + 1;
      profile.rawConfig = currentProfileConfigObject();
      saveActiveProfileDraft();
      persistProfiles();
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
      const text = JSON.stringify(currentProfileConfigObject(), null, 2);
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
    } else if (action === "format-profile-json") {
      const textarea = document.querySelector("[data-profile-json]");
      if (!textarea) return;
      const result = updateProfileJsonValidation(textarea);
      if (!result.valid) {
        showToast("暂时无法格式化", "请先修正完整配置 JSON。", "info");
        return;
      }
      textarea.value = JSON.stringify(result.config, null, 2);
      updateProfileJsonValidation(textarea);
      textarea.focus();
    } else if (action === "format-subscription-json") {
      const textarea = document.querySelector("[data-subscription-json]");
      if (!textarea) return;
      const result = updateSubscriptionValidation(textarea);
      if (!result.valid) {
        showToast("暂时无法格式化", "请先修正订阅 JSON 的语法或节点字段。", "info");
        return;
      }
      textarea.value = JSON.stringify(result.config, null, 2);
      updateSubscriptionValidation(textarea);
      textarea.focus();
    } else if (action === "reset-subscription-json") {
      const textarea = document.querySelector("[data-subscription-json]");
      if (!textarea) return;
      textarea.value = subscriptionJsonExample;
      updateSubscriptionValidation(textarea);
      textarea.focus();
    } else if (action === "format-manual-ruleset") {
      const textarea = document.querySelector("[data-manual-ruleset-json]");
      if (!textarea) return;
      const result = updateManualRulesetValidation(textarea);
      if (!result.valid) {
        showToast("暂时无法格式化", "请先修正手工规则 JSON。", "info");
        return;
      }
      textarea.value = JSON.stringify(result.config, null, 2);
      updateManualRulesetValidation(textarea);
      textarea.focus();
    } else if (action === "reset-manual-ruleset") {
      const textarea = document.querySelector("[data-manual-ruleset-json]");
      if (!textarea) return;
      textarea.value = manualRulesetExample;
      updateManualRulesetValidation(textarea);
      textarea.focus();
    } else if (action === "copy-subscription-json") {
      const item = state.subscriptions.find((subscription) => subscription.id === actionTarget.dataset.id);
      if (!item) return;
      copyText(subscriptionJsonFor(item));
      showToast("已复制订阅 JSON", `已复制 ${subscriptionNodeCount(item)} 个节点的完整 JSON。`, "success");
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
    } else if (action === "view-subscription-nodes") {
      openModal("subscription-nodes", actionTarget.dataset.id);
    } else if (action === "edit-subscription") {
      openModal("subscription", actionTarget.dataset.id);
    } else if (action === "refresh-ruleset-hub") {
      state.rulesetHubUpdated = "刚刚";
      updateRulesetHubResults();
      showToast("规则集中心已刷新", `已加载 ${rulesetHubItems.length} 个可用模板。`, "success");
    } else if (action === "import-hub-ruleset") {
      const hubItem = rulesetHubItems.find((item) => item.id === actionTarget.dataset.id);
      const format = actionTarget.dataset.format === "source" ? "source" : "binary";
      if (!hubItem || isRulesetHubItemAdded(hubItem.id, format)) return;
      state.rulesets.unshift({
        id: `hub-ruleset-${hubItem.id}-${format}-${Date.now()}`,
        hubId: hubItem.id,
        name: `${hubItem.name}-${format}`,
        source: hubItem.source,
        type: "remote",
        format,
        count: hubItem.count,
        size: estimateRulesetSize(hubItem.count, format),
        updated: "刚刚",
        url: rulesetHubUrl(hubItem, format),
        updateInterval: "1d",
        nextUpdate: rulesetNextUpdateLabel("1d"),
        disabled: false,
        previewContent: format === "source" ? rulesetHubPreviewContent(hubItem) : "",
      });
      render();
      if (document.querySelector(".ruleset-hub-preview-body")) openModal("ruleset-hub-preview", hubItem.id);
      else updateRulesetHubResults();
      showToast("规则集已导入", `${hubItem.name}（${format}）已添加，并启用每天自动更新。`, "success");
    } else if (action === "update-rulesets") {
      const remoteRulesets = state.rulesets.filter((item) => item.type === "remote");
      remoteRulesets.forEach((item) => {
        item.updated = "刚刚";
        item.nextUpdate = rulesetNextUpdateLabel(item.updateInterval);
      });
      render();
      showToast("网络规则更新完成", `${remoteRulesets.length} 个网络规则集已更新并重新加载。`, "success");
    } else if (action === "refresh-ruleset") {
      const item = state.rulesets.find((ruleset) => String(ruleset.id) === String(actionTarget.dataset.id));
      if (item?.type === "remote") {
        item.updated = "刚刚";
        item.nextUpdate = rulesetNextUpdateLabel(item.updateInterval);
      }
      render();
      showToast("规则集已更新", `${item?.name || "规则集"} 已加载最新版本。`);
    } else if (action === "clear-ruleset-filter") {
      state.rulesetSearch = "";
      state.rulesetType = "all";
      render();
    } else if (action === "confirm-delete-profile") {
      const profileId = actionTarget.dataset.id;
      const profile = state.profiles.find((item) => item.id === profileId);
      if (!profile || state.profiles.length === 1) {
        showToast("无法删除配置", "至少需要保留一个配置。", "info");
        return;
      }
      saveActiveProfileDraft();
      const deletingActive = profile.id === state.activeProfileId;
      state.profiles = state.profiles.filter((item) => item.id !== profile.id);
      if (deletingActive) {
        state.activeProfileId = state.profiles[0].id;
        loadProfileDraft(state.profiles[0]);
      }
      persistProfiles();
      closeModal();
      navigate("config", state.activeConfig);
      showToast("配置已删除", `“${profile.name}”及其草稿已移除。`, "success");
    } else if (action === "check-update") {
      showToast("已是最新版本", "当前 sing-box Stable 1.11.11 无需更新。", "info");
    } else if (action === "change-password") {
      showToast("原型操作", "正式接入后此处将打开密码修改流程。", "info");
    } else if (action === "project-info") {
      showToast("SingBox Control", "当前为交互原型，后续可对接 sing-box 与 Clash API。", "info");
    } else if (["edit-entry", "duplicate-entry", "remove-entry"].includes(action)) {
      const messages = {
        "edit-entry": "编辑面板将在正式版本中读取完整配置字段。",
        "duplicate-entry": "已复制一个配置项草稿。",
        "remove-entry": "原型不会删除现有配置项。",
      };
      showToast("原型交互", messages[action], "info");
    }
  });

  document.addEventListener("input", (event) => {
    if (event.target.matches("[data-profile-json]")) {
      updateProfileJsonValidation(event.target);
    }
    if (event.target.matches("[data-custom-inbound-json]")) {
      updateCustomInboundValidation(event.target);
    }
    if (event.target.matches("[data-subscription-json]")) {
      updateSubscriptionValidation(event.target);
    }
    if (event.target.matches("[data-manual-ruleset-json]")) {
      const fields = event.target.closest("[data-resource-ruleset-type-fields]");
      if (fields) fields.dataset.manualContent = event.target.value;
      updateManualRulesetValidation(event.target);
    }
    if (event.target.matches("[data-ruleset-hub-search]")) {
      state.rulesetHubSearch = event.target.value;
      updateRulesetHubResults();
    }
    if (event.target.matches("[data-config-field], [data-settings-input]")) {
      if (event.target.matches("[data-config-field]")) {
        state.configFields[event.target.dataset.configField] = event.target.value;
      }
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
    if (event.target.matches('input[name="profile-create-mode"]')) {
      const mode = event.target.value;
      const section = document.querySelector("[data-profile-json-section]");
      const note = document.querySelector("[data-profile-mode-note]");
      if (section) section.hidden = mode !== "json";
      if (note) note.innerHTML = profileModeNote(mode);
      const submit = document.querySelector("#profile-create-submit");
      if (submit) {
        const textarea = document.querySelector("[data-profile-json]");
        submit.disabled = mode === "json" && textarea ? !updateProfileJsonValidation(textarea).valid : false;
      }
      return;
    }
    if (event.target.matches("[data-config-field]")) {
      state.configFields[event.target.dataset.configField] = event.target.value;
      state.dirty = true;
      return;
    }
    if (event.target.matches("[data-ruleset-hub-category]")) {
      state.rulesetHubCategory = event.target.value;
      updateRulesetHubResults();
      return;
    }
    if (event.target.matches('[name="resource-ruleset-type"]')) {
      const form = event.target.closest("#resource-ruleset-form");
      const fields = form?.querySelector("[data-resource-ruleset-type-fields]");
      const editor = fields?.querySelector("[data-manual-ruleset-json]");
      if (editor && fields) fields.dataset.manualContent = editor.value;
      if (fields) fields.innerHTML = resourceRulesetTypeFields(event.target.value, { content: fields.dataset.manualContent || manualRulesetExample });
      const manualEditor = fields?.querySelector("[data-manual-ruleset-json]");
      if (manualEditor) updateManualRulesetValidation(manualEditor);
      else {
        const submit = document.querySelector("#resource-ruleset-submit");
        if (submit) submit.disabled = false;
      }
      return;
    }
    if (event.target.matches("[data-route-common]")) {
      state[event.target.name] = event.target.value;
      state.dirty = true;
      const button = document.querySelector("#save-config-button");
      if (button) button.innerHTML = `${icon("save")}保存更改`;
      return;
    }
    if (event.target.matches("[data-route-rule-enabled]")) {
      const rule = state.routeRules.find((item) => item.id === event.target.dataset.id);
      if (rule) rule.enable = event.target.checked;
      event.target.closest(".route-rule-row")?.classList.toggle("is-disabled", !event.target.checked);
      state.dirty = true;
      const enabledRules = state.routeRules.filter((item) => item.type !== "insertion" && item.enable).length;
      const badge = document.querySelector("#route-rules-section .route-section-badge");
      if (badge) badge.textContent = `已启用 ${enabledRules}/${routeRuleCount()}`;
      return;
    }
    if (event.target.matches("[data-route-rule-type]")) {
      const form = event.target.closest("#route-rule-form");
      const fields = form?.querySelector("[data-route-rule-payload-fields]");
      if (fields) fields.innerHTML = renderRouteRulePayloadFields(event.target.value, { type: event.target.value, payload: "" });
      return;
    }
    if (event.target.matches('[name="route-rule-action"]')) {
      const form = event.target.closest("#route-rule-form");
      const fields = form?.querySelector("[data-route-rule-action-fields]");
      if (fields) fields.innerHTML = renderRouteRuleActionFields(event.target.value, { action: event.target.value });
      return;
    }
    if (event.target.matches('[name="route-rule-ruleset"]')) {
      const form = event.target.closest("#route-rule-form");
      const count = form?.querySelectorAll('[name="route-rule-ruleset"]:checked').length || 0;
      const label = form?.querySelector("[data-route-rule-set-count]");
      if (label) label.textContent = `已选 ${count}`;
      return;
    }
    if (event.target.matches('[name="route-ruleset-type"]')) {
      const form = event.target.closest("#route-ruleset-form");
      const fields = form?.querySelector("[data-route-ruleset-type-fields]");
      if (fields) fields.innerHTML = renderRouteRulesetTypeFields(event.target.value, { type: event.target.value });
      return;
    }
    if (event.target.matches("[data-route-resource]")) {
      const form = event.target.closest("#route-ruleset-form");
      const tag = form?.querySelector('[name="route-ruleset-tag"]');
      if (tag) tag.value = event.target.dataset.resourceName || tag.value;
      return;
    }
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
    if (type === "profile-create") {
      const data = new FormData(form);
      const name = String(data.get("profile-name") || "").trim();
      const mode = String(data.get("profile-create-mode") || "blank");
      if (!name) {
        showToast("配置名称不能为空", "请填写一个便于识别的配置名称。", "info");
        form.querySelector('[name="profile-name"]')?.focus();
        return;
      }
      const duplicate = state.profiles.find((profile) => profile.name.toLowerCase() === name.toLowerCase());
      if (duplicate) {
        showToast("配置名称已存在", `“${duplicate.name}”已在配置列表中。`, "info");
        form.querySelector('[name="profile-name"]')?.focus();
        return;
      }
      saveActiveProfileDraft();
      const sourceProfile = activeProfile();
      let draft;
      let source;
      let rawConfig = null;
      if (mode === "copy") {
        draft = cloneData(sourceProfile.draft);
        draft.configFields.profileName = name;
        draft.dirty = true;
        draft.lastSaved = "尚未保存";
        source = `复制自 ${sourceProfile.name}`;
        rawConfig = sourceProfile.rawConfig ? cloneData(sourceProfile.rawConfig) : null;
      } else if (mode === "json") {
        const textarea = form.querySelector("[data-profile-json]");
        const result = updateProfileJsonValidation(textarea);
        if (!result.valid) return;
        rawConfig = cloneData(result.config);
        draft = profileDraftFromJson(result.config, name);
        source = "完整 JSON 导入";
      } else {
        draft = createBlankProfileDraft(name);
        source = "Web 空白配置";
      }
      const profile = {
        id: `config-${Date.now()}`,
        name,
        source,
        lastSaved: "尚未保存",
        version: 1,
        rawConfig,
        draft,
      };
      state.profiles.push(profile);
      state.activeProfileId = profile.id;
      loadProfileDraft(profile);
      persistProfiles();
      closeModal();
      navigate("config", "general");
      showToast("配置已创建", `“${name}”已加入配置列表并设为当前配置。`, "success");
    } else if (type === "profile-manage") {
      const profile = state.profiles.find((item) => item.id === form.dataset.id);
      const name = String(new FormData(form).get("profile-name") || "").trim();
      if (!profile || !name) {
        showToast("配置名称不能为空", "请填写一个便于识别的配置名称。", "info");
        form.querySelector('[name="profile-name"]')?.focus();
        return;
      }
      const duplicate = state.profiles.find((item) => item.id !== profile.id && item.name.toLowerCase() === name.toLowerCase());
      if (duplicate) {
        showToast("配置名称已存在", `“${duplicate.name}”已在配置列表中。`, "info");
        form.querySelector('[name="profile-name"]')?.focus();
        return;
      }
      const oldName = profile.name;
      profile.name = name;
      if (profile.id === state.activeProfileId) {
        state.configFields.profileName = name;
        state.dirty = true;
        saveActiveProfileDraft();
      } else {
        profile.draft.configFields.profileName = name;
        profile.draft.dirty = true;
      }
      persistProfiles();
      closeModal();
      render();
      showToast("配置已重命名", `“${oldName}”已更名为“${name}”。`, "success");
    } else if (type === "inbound") {
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
    } else if (type === "route-rule") {
      const rule = buildRouteRuleConfig(form, state.editingRouteRuleId);
      if (!rule.payload) {
        showToast("缺少匹配内容", rule.type === "rule_set" ? "请至少选择一个规则集。" : "请填写该规则类型的匹配内容。", "info");
        return;
      }
      if (rule.action === "route" && !rule.outbound) {
        showToast("缺少目标出站", "路由动作需要选择一个出站标签。", "info");
        return;
      }
      if ([rule.type === "inline" ? rule.payload : "", rule.action === "route-options" ? rule.outbound : ""].filter(Boolean).some((json) => {
        try { JSON.parse(json); return false; } catch { return true; }
      })) {
        showToast("JSON 格式错误", "请检查内联规则或路由选项的 JSON 语法。", "info");
        return;
      }
      const existingIndex = state.routeRules.findIndex((item) => item.id === state.editingRouteRuleId);
      if (existingIndex >= 0) state.routeRules.splice(existingIndex, 1, rule);
      else {
        const insertionIndex = state.routeRules.findIndex((item) => item.type === "insertion");
        state.routeRules.splice(insertionIndex >= 0 ? insertionIndex + 1 : 0, 0, rule);
      }
      state.dirty = true;
      closeModal();
      render();
      showToast(existingIndex >= 0 ? "路由规则已更新" : "路由规则已添加", `${rule.type} · ${routeRuleActionLabel(rule)}`, "success");
    } else if (type === "route-ruleset") {
      const ruleset = buildRouteRulesetConfig(form, state.editingRouteRulesetId);
      if (ruleset.type === "local" && !ruleset.path) {
        showToast("请选择本地资源", "本地规则集必须关联一个已下载的规则资源。", "info");
        return;
      }
      if (ruleset.type === "remote" && !ruleset.url) {
        showToast("缺少远程链接", "远程规则集需要填写下载 URL。", "info");
        return;
      }
      if (ruleset.type === "inline") {
        try { JSON.parse(ruleset.rules); } catch {
          showToast("JSON 格式错误", "请检查内联规则集的 JSON 语法。", "info");
          return;
        }
      }
      const existingIndex = state.routeRuleSets.findIndex((item) => item.id === state.editingRouteRulesetId);
      if (existingIndex >= 0) state.routeRuleSets.splice(existingIndex, 1, ruleset);
      else state.routeRuleSets.unshift(ruleset);
      state.dirty = true;
      closeModal();
      render();
      showToast(existingIndex >= 0 ? "规则集已更新" : "规则集已添加", `${ruleset.tag}（${ruleset.type}）已加入当前配置草稿。`, "success");
    } else if (type === "resource-ruleset") {
      const data = new FormData(form);
      const rulesetType = String(data.get("resource-ruleset-type") || "remote");
      const name = String(data.get("resource-ruleset-name") || "").trim();
      if (!name) {
        showToast("缺少规则集名称", "请为规则集填写一个便于识别的名称。", "info");
        form.querySelector('[name="resource-ruleset-name"]')?.focus();
        return;
      }
      const existingIndex = state.rulesets.findIndex((item) => String(item.id) === String(state.editingResourceRulesetId));
      const existing = existingIndex >= 0 ? state.rulesets[existingIndex] : null;
      let resource;
      if (rulesetType === "manual") {
        const textarea = form.querySelector("[data-manual-ruleset-json]");
        const result = updateManualRulesetValidation(textarea);
        if (!result.valid) return;
        const content = JSON.stringify(result.config, null, 2);
        resource = {
          id: existing?.id || `manual-ruleset-${Date.now()}`,
          name,
          source: "手工规则 · JSON",
          type: "manual",
          format: "source",
          count: result.count,
          size: formatRulesetTextSize(content),
          updated: "刚刚",
          url: "",
          updateInterval: "manual",
          nextUpdate: "手工维护",
          disabled: existing?.disabled || false,
          content,
        };
      } else {
        const url = String(data.get("resource-ruleset-url") || "").trim();
        if (!/^https?:\/\//i.test(url)) {
          showToast("网络地址无效", "网络规则集必须填写以 http:// 或 https:// 开头的地址。", "info");
          form.querySelector('[name="resource-ruleset-url"]')?.focus();
          return;
        }
        const format = String(data.get("resource-ruleset-format") || "binary");
        const updateInterval = String(data.get("resource-ruleset-interval") || "manual");
        let source = url;
        try { source = new URL(url).hostname; } catch { /* 保留用户输入的地址作为来源。 */ }
        resource = {
          id: existing?.id || `remote-ruleset-${Date.now()}`,
          hubId: existing?.hubId,
          name,
          source,
          type: "remote",
          format,
          count: existing ? rulesetCountValue(existing) : 0,
          size: existing?.size || "尚未下载",
          updated: existing?.updated || "尚未更新",
          url,
          updateInterval,
          nextUpdate: rulesetNextUpdateLabel(updateInterval),
          disabled: existing?.disabled || false,
          previewContent: existing?.previewContent || "",
        };
      }
      if (existingIndex >= 0) state.rulesets.splice(existingIndex, 1, resource);
      else state.rulesets.unshift(resource);
      closeModal();
      render();
      showToast(existing ? "规则集已保存" : "规则集已添加", resource.type === "remote" ? `${name} 将按“${rulesetIntervalLabel(resource.updateInterval)}”更新。` : `${name} 已保存 ${formatRulesetCount(resource.count)} 条手工规则信息。`, "success");
    } else if (type === "subscription") {
      const textarea = form.querySelector("[data-subscription-json]");
      const result = updateSubscriptionValidation(textarea);
      if (!result.valid) return;
      const existing = state.subscriptions.find((subscription) => subscription.id === state.editingSubscriptionId);
      const name = result.name || existing?.name || `JSON 订阅 ${state.subscriptions.length + 1}`;
      const nodeList = result.nodes.map((node) => ({ ...node, type: node.type.trim(), tag: node.tag.trim() }));
      const subscription = {
        id: existing?.id || `sub-${Date.now()}`,
        name,
        logo: outboundMemberInitial(name),
        tone: existing?.tone || (state.subscriptions.length % 2 ? "blue" : "cyan"),
        nodes: nodeList.length,
        nodeList,
        sourceJson: JSON.stringify(result.config, null, 2),
        updated: "刚刚",
      };
      if (existing) Object.assign(existing, subscription);
      else state.subscriptions.push(subscription);
      closeModal();
      render();
      showToast(existing ? "JSON 订阅已更新" : "JSON 订阅已导入", `${name} 的 ${nodeList.length} 个节点已全部保存。`, "success");
    } else {
      closeModal();
      state.dirty = true;
      render();
      showToast("配置项已添加", "新项目已加入当前配置草稿，请保存后应用。", "success");
    }
  });

  document.addEventListener("dblclick", (event) => {
    const card = event.target.closest("[data-subscription-card]");
    if (!card || event.target.closest("button, a, input, select, textarea")) return;
    openModal("subscription-nodes", card.dataset.id);
  });

  document.addEventListener("keydown", (event) => {
    if (event.target.matches("[data-profile-json]") && event.key === "Tab") {
      event.preventDefault();
      const textarea = event.target;
      textarea.setRangeText("  ", textarea.selectionStart, textarea.selectionEnd, "end");
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
      return;
    }
    if (event.target.matches("[data-profile-json]") && (event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      const submit = document.querySelector("#profile-create-submit");
      if (!submit?.disabled) document.querySelector("#profile-create-form")?.requestSubmit();
      return;
    }
    if (event.target.matches("[data-subscription-card]") && ["Enter", " "].includes(event.key)) {
      event.preventDefault();
      openModal("subscription-nodes", event.target.dataset.id);
      return;
    }
    if (event.target.matches("[data-tun-list-input]") && event.key === "Enter") {
      event.preventDefault();
      event.target.closest("[data-tun-list-field]")?.querySelector('[data-action="add-tun-list-item"]')?.click();
      return;
    }
    if (event.target.matches("[data-subscription-json]") && event.key === "Tab") {
      event.preventDefault();
      const textarea = event.target;
      textarea.setRangeText("  ", textarea.selectionStart, textarea.selectionEnd, "end");
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
      return;
    }
    if (event.target.matches("[data-subscription-json]") && (event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      const submit = document.querySelector("#subscription-submit");
      if (!submit?.disabled) document.querySelector("#subscription-form")?.requestSubmit();
      return;
    }
    if (event.target.matches("[data-manual-ruleset-json]") && event.key === "Tab") {
      event.preventDefault();
      const textarea = event.target;
      textarea.setRangeText("  ", textarea.selectionStart, textarea.selectionEnd, "end");
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
      return;
    }
    if (event.target.matches("[data-manual-ruleset-json]") && (event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      const submit = document.querySelector("#resource-ruleset-submit");
      if (!submit?.disabled) document.querySelector("#resource-ruleset-form")?.requestSubmit();
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
    if (state.page === "config") {
      const routeProfileId = route[2] ? route[1] : null;
      const routeTab = route[2] || route[1];
      const profile = state.profiles.find((item) => item.id === routeProfileId);
      if (profile && profile.id !== state.activeProfileId) {
        saveActiveProfileDraft();
        state.activeProfileId = profile.id;
        loadProfileDraft(profile);
      }
      if (configTabs.some((tab) => tab.id === routeTab)) state.activeConfig = routeTab;
    }
    closeModal();
    render();
  });

  initializeProfiles();
  applyTheme();
  setAccent(state.accent);
  render();
})();
