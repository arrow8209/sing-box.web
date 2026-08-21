export type ConfigModule =
  | "log" | "dns" | "inbounds" | "outbounds" | "endpoints" | "route" | "services"
  | "ntp" | "certificate" | "certificate_providers" | "http_clients" | "network_namespaces" | "experimental";

export const configTabs = [
  { id: "overview", label: "概览", modules: ["log"] },
  { id: "dns", label: "DNS", modules: ["dns"] },
  { id: "inbounds", label: "入站", modules: ["inbounds"] },
  { id: "outbounds", label: "出站", modules: ["outbounds"] },
  { id: "endpoints", label: "端点", modules: ["endpoints"] },
  { id: "route", label: "路由", modules: ["route"] },
  { id: "services", label: "服务", modules: ["services"] },
  { id: "advanced", label: "高级", modules: ["ntp", "certificate", "certificate_providers", "http_clients", "network_namespaces", "experimental"] }
] as const;

export type ConfigTabId = (typeof configTabs)[number]["id"];
export const moduleLabels: Record<ConfigModule, string> = {
  log: "日志", dns: "DNS", inbounds: "入站", outbounds: "出站", endpoints: "端点", route: "路由", services: "服务",
  ntp: "NTP", certificate: "证书", certificate_providers: "证书提供器", http_clients: "HTTP 客户端", network_namespaces: "网络命名空间", experimental: "实验功能"
};
