export interface ModuleDescriptor {
  key: string;
  label: string;
  shortLabel: string;
  group: "common" | "network" | "advanced";
}

export const modules: ModuleDescriptor[] = [
  { key: "general", label: "配置概览", shortLabel: "概览", group: "common" },
  { key: "dns", label: "DNS", shortLabel: "DNS", group: "network" },
  { key: "inbounds", label: "入站", shortLabel: "入站", group: "network" },
  { key: "outbounds", label: "出站", shortLabel: "出站", group: "network" },
  { key: "endpoints", label: "端点", shortLabel: "端点", group: "network" },
  { key: "routing", label: "路由", shortLabel: "路由", group: "network" },
  { key: "services", label: "服务", shortLabel: "服务", group: "advanced" },
  { key: "advanced", label: "高级", shortLabel: "高级", group: "advanced" },
];

/** 模块键到顶层配置键的映射；未列出的模块与其键同名。 */
export const moduleTopLevelKeys: Record<string, string> = {
  general: "log",
  routing: "route",
};

export function topLevelKey(moduleKey: string): string {
  return moduleTopLevelKeys[moduleKey] ?? moduleKey;
}

/** `advanced` 模块所覆盖的顶层键。 */
export const advancedTopLevelKeys = [
  "$schema",
  "ntp",
  "certificate",
  "certificate_providers",
  "http_clients",
  "network_namespaces",
  "experimental",
] as const;

/** 敏感键（大小写不敏感精确匹配）。 */
export const sensitiveKeys = [
  "password",
  "passwd",
  "secret",
  "token",
  "access_token",
  "refresh_token",
  "authorization",
  "api_key",
  "client_secret",
  "uuid",
  "private_key",
  "pre_shared_key",
] as const;

/** 敏感模块（默认整体隐藏，需显式确认才可编辑）；同时接受描述符键与顶层键。 */
export const sensitiveModules = new Set(["advanced", "services", "experimental"]);

export function isSensitiveModule(moduleKey: string): boolean {
  return sensitiveModules.has(moduleKey);
}

export function isSensitiveKey(key: string): boolean {
  const lower = key.toLowerCase();
  return (sensitiveKeys as readonly string[]).includes(lower);
}

/** 把 JSON Pointer 首段转成模块路由参数；未知首段返回 null。 */
export function jsonPointerToModule(pointer: string): string | null {
  const match = /^\/([^/]+)/.exec(pointer);
  if (match === null) {
    return null;
  }
  const segment = decodeURIComponent(match[1].replace(/~1/g, "/").replace(/~0/g, "~"));
  for (const descriptor of modules) {
    if (topLevelKey(descriptor.key) === segment) {
      return descriptor.key;
    }
  }
  if ((advancedTopLevelKeys as readonly string[]).includes(segment)) {
    return "advanced";
  }
  return null;
}
