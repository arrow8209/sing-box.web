export type ConfigModule =
  | "log" | "dns" | "inbounds" | "outbounds" | "route" | "services"
  | "ntp" | "certificate" | "certificate_providers" | "http_clients" | "network_namespaces" | "experimental";

export const configTabs = [
  { id: "general", label: "常规设置", modules: ["log"] },
  { id: "inbounds", label: "入站", modules: ["inbounds"] },
  { id: "outbounds", label: "出站", modules: ["outbounds"] },
  { id: "route", label: "路由", modules: ["route"] },
  { id: "dns", label: "DNS", modules: ["dns"] },
  { id: "services", label: "服务", modules: ["services"] },
  { id: "advanced", label: "高级", modules: ["ntp", "certificate", "certificate_providers", "http_clients", "network_namespaces", "experimental"] }
] as const;

export type ConfigTabId = (typeof configTabs)[number]["id"];

export interface ConfigSection {
  key: ConfigModule;
  label: string;
  type: "array" | "object";
  group: ConfigTabId;
  icon: string;
  description: string;
}

// These are the twelve top-level modules managed by the product. `$schema`
// may be retained in an imported draft, but it is deliberately not presented
// as an editable or validated module.
export const configSections: readonly ConfigSection[] = [
  { key: "log", label: "日志", type: "object", group: "general", icon: "terminal", description: "日志等级与输出" },
  { key: "dns", label: "DNS", type: "object", group: "dns", icon: "dns", description: "服务器、规则与缓存" },
  { key: "ntp", label: "NTP", type: "object", group: "advanced", icon: "clock", description: "时间同步" },
  { key: "certificate", label: "证书", type: "object", group: "advanced", icon: "lock", description: "系统与自定义证书" },
  { key: "certificate_providers", label: "证书提供器", type: "array", group: "advanced", icon: "shield", description: "ACME、Tailscale 等" },
  { key: "http_clients", label: "HTTP 客户端", type: "array", group: "advanced", icon: "globe", description: "可复用下载客户端" },
  { key: "network_namespaces", label: "网络命名空间", type: "array", group: "advanced", icon: "folder", description: "default 与 unshare" },
  { key: "inbounds", label: "入站", type: "array", group: "inbounds", icon: "inbound", description: "流量入口" },
  { key: "outbounds", label: "出站", type: "array", group: "outbounds", icon: "outbound", description: "出口与策略组" },
  { key: "route", label: "路由", type: "object", group: "route", icon: "route", description: "规则与规则集" },
  { key: "services", label: "服务", type: "array", group: "services", icon: "server", description: "API 与后台服务" },
  { key: "experimental", label: "实验性", type: "object", group: "advanced", icon: "sparkles", description: "Clash API、缓存等" }
];

export const advancedConfigSections = configSections.filter((section) => section.group === "advanced");

export function configSectionFor(module: ConfigModule): ConfigSection {
  const section = configSections.find((candidate) => candidate.key === module);
  if (!section) throw new Error(`未知配置模块：${module}`);
  return section;
}

export const moduleLabels: Record<ConfigModule, string> = {
  log: "日志", dns: "DNS", inbounds: "入站", outbounds: "出站", route: "路由", services: "服务",
  ntp: "NTP", certificate: "证书", certificate_providers: "证书提供器", http_clients: "HTTP 客户端", network_namespaces: "网络命名空间", experimental: "实验性"
};
