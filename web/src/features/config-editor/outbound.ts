import { applyEdits, modify, parseTree, type ParseError } from "jsonc-parser";

export type OutboundEditorType = "direct" | "block" | "selector" | "urltest" | "custom";

export type OutboundConfig = Record<string, unknown> & {
  type: string;
  tag: string;
};

export interface OutboundReferenceSelection {
  key: string;
  tag: string;
  config?: OutboundConfig;
}

export interface OutboundEditorResult {
  config: OutboundConfig;
  references: OutboundReferenceSelection[];
  editorType?: OutboundEditorType;
}

export interface OutboundJsonValidationSuccess {
  valid: true;
  config: OutboundConfig;
  message: string;
}

export interface OutboundJsonValidationFailure {
  valid: false;
  message: string;
}

export type OutboundJsonValidation = OutboundJsonValidationSuccess | OutboundJsonValidationFailure;

export const outboundTypes: ReadonlyArray<{
  type: OutboundEditorType;
  label: string;
  icon: string;
}> = [
  { type: "direct", label: "直连", icon: "lightning" },
  { type: "block", label: "阻断", icon: "shield" },
  { type: "selector", label: "手动选择", icon: "globe" },
  { type: "urltest", label: "自动选择", icon: "gauge" },
  { type: "custom", label: "完整 JSON", icon: "code" }
];

const structuredTypes = new Set<OutboundEditorType>(["direct", "block", "selector", "urltest"]);

export function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

export function cloneValue<T>(value: T): T {
  if (Array.isArray(value)) return value.map((item) => cloneValue(item)) as T;
  if (!isObject(value)) return value;
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, cloneValue(item)])) as T;
}

export function isOutboundConfig(value: unknown): value is OutboundConfig {
  return isObject(value)
    && typeof value.type === "string"
    && Boolean(value.type.trim())
    && typeof value.tag === "string"
    && Boolean(value.tag.trim());
}

export function outboundEditorType(config: OutboundConfig): OutboundEditorType {
  return structuredTypes.has(config.type as OutboundEditorType) ? config.type as OutboundEditorType : "custom";
}

export function isPolicyOutbound(config: Pick<OutboundConfig, "type">): boolean {
  return config.type === "selector" || config.type === "urltest";
}

export function outboundReferenceTags(config: OutboundConfig): string[] {
  if (!Array.isArray(config.outbounds)) return [];
  return config.outbounds.filter((tag): tag is string => typeof tag === "string" && Boolean(tag.trim()));
}

export function outboundMemberInitial(name: string): string {
  const cleanName = name.replace(/^\p{Extended_Pictographic}+\s*/u, "").trim();
  if (/^[a-z]/i.test(cleanName)) return cleanName.slice(0, 2).toUpperCase();
  return Array.from(cleanName).slice(0, 2).join("") || "--";
}

export function outboundDescription(config: OutboundConfig): string {
  if (config.type === "selector") return "手动选择引用的出站或订阅节点";
  if (config.type === "urltest") return `每 ${typeof config.interval === "string" && config.interval ? config.interval : "5m"} 检测延迟`;
  if (config.type === "direct") return "不经过代理服务器";
  if (config.type === "block") return "直接阻断匹配流量";
  if (typeof config.server === "string" && config.server) return config.server;
  return "自定义出站";
}

export function outboundIcon(config: OutboundConfig): string {
  if (config.type === "selector") return "globe";
  if (config.type === "urltest") return "gauge";
  if (config.type === "direct") return "lightning";
  if (config.type === "block") return "shield";
  return "outbound";
}

export function uniqueOutboundTag(seed: string, used: Iterable<string>): string {
  const tags = new Set(used);
  const base = seed.trim() || "outbound";
  if (!tags.has(base)) return base;
  let suffix = 2;
  while (tags.has(`${base}-${suffix}`)) suffix++;
  return `${base}-${suffix}`;
}

export function validateOutboundJson(source: string, existingTags: Iterable<string> = [], originalTag = ""): OutboundJsonValidation {
  let parsed: unknown;
  try {
    parsed = JSON.parse(source) as unknown;
  } catch (cause) {
    return { valid: false, message: `JSON 语法错误：${cause instanceof Error ? cause.message : "无法解析 JSON"}` };
  }
  if (!isObject(parsed)) return { valid: false, message: "请输入单个出站 JSON 对象，不能使用数组。" };
  if (typeof parsed.type !== "string" || !parsed.type.trim()) return { valid: false, message: "出站配置必须包含非空的 type 字段。" };
  if (typeof parsed.tag !== "string" || !parsed.tag.trim()) return { valid: false, message: "出站配置必须包含非空的 tag 字段。" };
  const type = parsed.type.trim();
  const tag = parsed.tag.trim();
  if (Array.from(type).length > 64) return { valid: false, message: "出站 type 不能超过 64 个字符。" };
  if (Array.from(tag).length > 128) return { valid: false, message: "出站名称不能超过 128 个字符。" };
  if (tag !== originalTag && new Set(existingTags).has(tag)) return { valid: false, message: `出站名称已存在：${tag}` };
  return { valid: true, config: { ...parsed, type, tag } as OutboundConfig, message: `JSON 有效 · 标签 ${tag} · 类型 ${type}` };
}

function referencedTag(value: unknown, mapping: Map<string, string>): unknown {
  return typeof value === "string" && mapping.has(value) ? mapping.get(value) : value;
}

function remapImportedReferences(config: OutboundConfig, mapping: Map<string, string>): void {
  config.detour = referencedTag(config.detour, mapping);
  config.default = referencedTag(config.default, mapping);
  if (Array.isArray(config.outbounds)) config.outbounds = config.outbounds.map((tag) => referencedTag(tag, mapping));
}

/**
 * Applies one structured editor result to an outbounds array. Subscription
 * references carry complete node objects; those objects become independent
 * profile copies and only their resolved tags enter the selector/urltest.
 */
export function mergeOutboundEdit(
  source: unknown[],
  editingIndex: number | null,
  result: OutboundEditorResult
): { items: unknown[]; config: OutboundConfig; imports: OutboundConfig[]; importedCount: number; referencedTags: string[] } {
  const items = cloneValue(source);
  const config = cloneValue(result.config);
  const used = new Set<string>();

  items.forEach((item, index) => {
    if (!isObject(item) || typeof item.tag !== "string" || !item.tag.trim() || index === editingIndex) return;
    const tag = item.tag.trim();
    used.add(tag);
  });
  used.add(config.tag);

  const allocated = new Map<string, string>();
  const imports: OutboundConfig[] = [];
  const references: string[] = [];

  for (const reference of result.references) {
    const requestedTag = reference.tag.trim();
    if (!requestedTag) continue;
    if (!reference.config) {
      if (requestedTag !== config.tag && !references.includes(requestedTag)) references.push(requestedTag);
      continue;
    }

    const allocationKey = JSON.stringify(reference.config);
    let resolvedTag = allocated.get(allocationKey);
    if (!resolvedTag) {
      const imported = cloneValue(reference.config);
      resolvedTag = uniqueOutboundTag(requestedTag, used);
      imported.type = imported.type.trim();
      imported.tag = resolvedTag;
      used.add(resolvedTag);
      imports.push(imported);
      allocated.set(allocationKey, resolvedTag);
      if (!allocated.has(requestedTag)) allocated.set(requestedTag, resolvedTag);
    }
    if (resolvedTag !== config.tag && !references.includes(resolvedTag)) references.push(resolvedTag);
  }

  for (const imported of imports) remapImportedReferences(imported, allocated);
  if (isPolicyOutbound(config) && (result.editorType !== "custom" || references.length > 0)) config.outbounds = references;

  if (editingIndex === null) items.unshift(config);
  else if (editingIndex >= 0 && editingIndex < items.length) items[editingIndex] = config;
  else items.push(config);
  items.push(...imports);
  return { items, config, imports, importedCount: imports.length, referencedTags: references };
}

const formattingOptions = { insertSpaces: true, tabSize: 2, eol: "\n" } as const;
const structuredKeys = [
  "type", "tag", "hidden", "include", "exclude", "icon",
  "interrupt_exist_connections", "outbounds", "url", "interval", "tolerance"
] as const;

function validArrayBuffer(source: string): boolean {
  const errors: ParseError[] = [];
  const tree = parseTree(source, errors, { allowTrailingComma: false, disallowComments: true });
  return errors.length === 0 && tree?.type === "array";
}

function editJson(source: string, path: (string | number)[], value: unknown, isArrayInsertion = false): string {
  return applyEdits(source, modify(source, path, value, { formattingOptions, isArrayInsertion }));
}

/** Preserves untouched array items and their original numeric/string tokens. */
export function applyOutboundBufferEdit(
  sourceText: string,
  sourceItems: unknown[],
  editingIndex: number | null,
  result: OutboundEditorResult
): ReturnType<typeof mergeOutboundEdit> & { text: string } {
  const merged = mergeOutboundEdit(sourceItems, editingIndex, result);
  if (!validArrayBuffer(sourceText)) return { ...merged, text: JSON.stringify(merged.items, null, 2) };

  let text = sourceText;
  let length = sourceItems.length;
  if (editingIndex === null || editingIndex < 0 || editingIndex >= sourceItems.length) {
    text = editJson(text, [0], merged.config, true);
    length++;
  } else if (result.editorType === "custom") {
    text = editJson(text, [editingIndex], merged.config);
  } else {
    for (const key of structuredKeys) {
      const value = Object.prototype.hasOwnProperty.call(merged.config, key) ? merged.config[key] : undefined;
      text = editJson(text, [editingIndex, key], value);
    }
  }
  for (const imported of merged.imports) {
    text = editJson(text, [length], imported);
    length++;
  }
  return { ...merged, text };
}

export function removeOutboundFromBuffer(sourceText: string, sourceItems: unknown[], index: number): string {
  if (!validArrayBuffer(sourceText) || index < 0 || index >= sourceItems.length) {
    const items = cloneValue(sourceItems);
    if (index >= 0 && index < items.length) items.splice(index, 1);
    return JSON.stringify(items, null, 2);
  }
  return editJson(sourceText, [index], undefined);
}

export function updateOutboundFieldInBuffer(
  sourceText: string,
  sourceItems: unknown[],
  index: number,
  key: string,
  value: unknown
): string {
  if (!validArrayBuffer(sourceText) || index < 0 || index >= sourceItems.length) {
    const items = cloneValue(sourceItems);
    const config = items[index];
    if (isObject(config)) config[key] = value;
    return JSON.stringify(items, null, 2);
  }
  return editJson(sourceText, [index, key], value);
}
