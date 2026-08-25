import type { InboundConfig, InboundEditorType, InboundJsonValidation, InboundMode } from "./types";

export const inboundTypes: ReadonlyArray<{ type: InboundEditorType; label: string; icon: string; description: string }> = [
  { type: "mixed", label: "Mixed", icon: "inbound", description: "HTTP 与 SOCKS 复合代理" },
  { type: "http", label: "HTTP", icon: "globe", description: "HTTP 与 CONNECT 代理" },
  { type: "socks", label: "SOCKS", icon: "route", description: "SOCKS4 / SOCKS5 代理" },
  { type: "tun", label: "TUN", icon: "shield", description: "虚拟网卡与自动路由" },
  { type: "direct", label: "Direct", icon: "lightning", description: "透明流量直连入口" },
  { type: "custom", label: "完整 JSON", icon: "code", description: "任意 Schema 入站类型" }
];

const structured = new Set<InboundEditorType>(["mixed", "http", "socks", "tun", "direct"]);

export function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

export function isInboundConfig(value: unknown): value is InboundConfig {
  return isObject(value) && typeof value.type === "string" && Boolean(value.type.trim()) && typeof value.tag === "string" && Boolean(value.tag.trim());
}

export function editorType(config: InboundConfig, mode?: InboundMode): InboundEditorType {
  if (mode === "custom") return "custom";
  return structured.has(config.type as InboundEditorType) ? config.type as InboundEditorType : "custom";
}

export function resourceMode(config: InboundConfig): InboundMode {
  return structured.has(config.type as InboundEditorType) ? "structured" : "custom";
}

export function uniqueInboundTag(seed: string, used: Iterable<string>): string {
  const tags = new Set(used);
  const base = seed.trim() || "inbound";
  if (!tags.has(base)) return base;
  let suffix = 2;
  while (tags.has(`${base}-${suffix}`)) suffix++;
  return `${base}-${suffix}`;
}

export function validateInboundJson(source: string, existingTags: Iterable<string> = [], originalTag = ""): InboundJsonValidation {
  let parsed: unknown;
  try {
    parsed = JSON.parse(source) as unknown;
  } catch (cause) {
    return { valid: false, message: `JSON 语法错误：${cause instanceof Error ? cause.message : "无法解析 JSON"}` };
  }
  if (!isObject(parsed)) return { valid: false, message: "请输入单个入站 JSON 对象，不能使用数组。" };
  if (typeof parsed.type !== "string" || !parsed.type.trim()) return { valid: false, message: "入站配置必须包含非空的 type 字段。" };
  if (typeof parsed.tag !== "string" || !parsed.tag.trim()) return { valid: false, message: "入站配置必须包含非空的 tag 字段。" };
  const type = parsed.type.trim();
  const tag = parsed.tag.trim();
  if (Array.from(type).length > 64) return { valid: false, message: "入站 type 不能超过 64 个字符。" };
  if (Array.from(tag).length > 128) return { valid: false, message: "入站 tag 不能超过 128 个字符。" };
  if (tag !== originalTag && new Set(existingTags).has(tag)) return { valid: false, message: `入站名称已存在：${tag}` };
  return { valid: true, config: { ...parsed, type, tag } as InboundConfig, message: `JSON 有效 · 标签 ${tag} · 类型 ${type}` };
}

export function inboundEndpoint(config: InboundConfig): string {
  if (config.type === "tun") {
    const name = typeof config.interface_name === "string" && config.interface_name.trim() ? config.interface_name.trim() : "自动命名";
    const address = Array.isArray(config.address) && typeof config.address[0] === "string" ? config.address[0] : "";
    return address ? `${name} · ${address}` : name;
  }
  const listen = typeof config.listen === "string" && config.listen.trim() ? config.listen.trim() : "由完整 JSON 定义";
  return typeof config.listen_port === "number" ? `${listen}:${config.listen_port}` : listen;
}

export function updatedLabel(value: string): string {
  const stamp = new Date(value).getTime();
  if (!Number.isFinite(stamp)) return "未知";
  const seconds = Math.max(0, Math.floor((Date.now() - stamp) / 1000));
  if (seconds < 60) return "刚刚";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`;
  if (seconds < 604800) return `${Math.floor(seconds / 86400)} 天前`;
  return new Date(value).toLocaleDateString("zh-CN");
}
