import { applyEdits, modify, parseTree, type Node, type ParseError } from "jsonc-parser";
import type { Draft, PersistedReferences } from "@/features/profiles/types";
import { persistedReferences } from "@/features/profiles/reference-input";

export type OutboundEditorType = "direct" | "block" | "selector" | "urltest";

export type OutboundConfig = Record<string, unknown> & {
  type: string;
  tag: string;
};

interface ResolvedReference {
  resolvedTags?: string[];
}

export type OutboundReference = (
  | { kind: "builtin"; tag: "direct" | "block" }
  | { kind: "outbound"; outboundId: string; tag: string }
  | { kind: "subscription"; subscriptionId: string; tag: string }
  | { kind: "subscription-node"; subscriptionId: string; nodeTag: string }
  | { kind: "raw"; tag: string }
  | { kind: "missing"; tag: string }
) & ResolvedReference;

export interface OutboundGroupState {
  id: string;
  tag: string;
  type: OutboundEditorType;
  hidden: boolean;
  include: string;
  exclude: string;
  icon: string;
  references: OutboundReference[];
  runtimeTags: string[];
}

export interface OutboundEditorState {
  version: 1;
  groups: OutboundGroupState[];
  generatedTags: string[];
}

export interface OutboundEditorGroup extends OutboundGroupState {
  config: OutboundConfig;
}

export interface OutboundEditorResult {
  group: OutboundEditorGroup;
}

export interface OutboundPanelSave {
  text: string;
  uiState: { version: 2; groups: Array<{ id: string; tag: string; hidden: boolean; icon: string }> };
  references: PersistedReferences;
  renamed?: { from: string; to: string };
}

// Edit only the known form fields in raw JSON. Source membership is now a
// separate business input; unknown fields and number tokens stay untouched.
export function persistOutboundInputs(draft: Draft, currentGroups: OutboundEditorGroup[], nextGroups: OutboundEditorGroup[], renamed?: { from: string; to: string }): OutboundPanelSave {
  const source = draft.buffers.outbounds ?? JSON.stringify(draft.config.outbounds ?? []);
  if (!validArrayBuffer(source)) throw new Error("出站 JSON 无效，请先修正输入。");
  const tree = parseTree(source)!;
  const items = Array.isArray(draft.config.outbounds) ? draft.config.outbounds : [];
  const indexByTag = new Map(items.flatMap((item, index) => isOutboundConfig(item) ? [[item.tag, index] as const] : []));
  const previous = new Map(currentGroups.map((group) => [group.id, group]));
  const nextById = new Map(nextGroups.map((group) => [group.id, group]));
  let text = source;
  for (const group of nextGroups) {
    const old = previous.get(group.id);
    const index = old ? indexByTag.get(old.tag) : undefined;
    if (index === undefined) continue;
    for (const key of ["type", "tag", "interrupt_exist_connections", "url", "interval", "tolerance", "hidden", "icon", "include", "exclude", "outbounds"] as const) {
      const value = ["hidden", "icon", "include", "exclude", "outbounds"].includes(key) ? undefined : group.config[key];
      text = editJson(text, [index, key], value);
    }
  }
  const removed = currentGroups.flatMap((group) => !nextById.has(group.id) && indexByTag.has(group.tag) ? [indexByTag.get(group.tag)!] : []).sort((a, b) => b - a);
  for (const index of removed) text = editJson(text, [index], undefined);
  let insertionIndex = tree.children!.length - removed.length;
  for (const group of nextGroups.filter((item) => !previous.has(item.id))) {
    const config = cloneValue(group.config);
    for (const key of ["hidden", "icon", "include", "exclude", "outbounds"]) delete config[key];
    text = editJson(text, [insertionIndex++], config);
  }
  const references = persistedReferences(draft);
  references.outbounds = nextGroups.map((group) => ({
    id: group.id, tag: group.tag, type: group.type, include: group.include, exclude: group.exclude,
    references: group.references.map(({ resolvedTags: _resolvedTags, ...reference }) => ({ ...reference }))
  }));
  return { text, references, uiState: { version: 2, groups: nextGroups.map((group) => ({ id: group.id, tag: group.tag, hidden: group.hidden, icon: group.icon })) }, renamed };
}

export const outboundTypes: ReadonlyArray<{
  type: OutboundEditorType;
  label: string;
  icon: string;
}> = [
  { type: "direct", label: "直连", icon: "lightning" },
  { type: "block", label: "阻断", icon: "shield" },
  { type: "selector", label: "手动选择", icon: "globe" },
  { type: "urltest", label: "自动选择", icon: "gauge" }
];

const structuredTypes = new Set<OutboundEditorType>(["direct", "block", "selector", "urltest"]);
const formattingOptions = { insertSpaces: true, tabSize: 2, eol: "\n" } as const;
let fallbackId = 0;

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

export function isManagedOutbound(config: OutboundConfig): boolean {
  return structuredTypes.has(config.type as OutboundEditorType);
}

export function outboundEditorType(config: OutboundConfig): OutboundEditorType | null {
  return structuredTypes.has(config.type as OutboundEditorType) ? config.type as OutboundEditorType : null;
}

export function isPolicyOutbound(config: Pick<OutboundConfig, "type">): boolean {
  return config.type === "selector" || config.type === "urltest";
}

export function outboundReferenceTags(config: OutboundConfig): string[] {
  if (!Array.isArray(config.outbounds)) return [];
  return config.outbounds.filter((tag): tag is string => typeof tag === "string" && Boolean(tag.trim()));
}

export function outboundDescription(config: OutboundConfig): string {
  if (config.type === "selector") return "手动选择引用的出站或订阅";
  if (config.type === "urltest") return `每 ${typeof config.interval === "string" && config.interval ? config.interval : "5m"} 检测延迟`;
  if (config.type === "direct") return "不经过代理服务器";
  if (config.type === "block") return "直接阻断匹配流量";
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

export function createOutboundGroupId(used: Iterable<string> = []): string {
  const existing = new Set(used);
  for (let attempt = 0; attempt < 8; attempt++) {
    const bytes = new Uint8Array(8);
    globalThis.crypto?.getRandomValues?.(bytes);
    const random = [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
    const id = random && !/^0+$/.test(random) ? `outbound-${random}` : `outbound-local-${++fallbackId}`;
    if (!existing.has(id)) return id;
  }
  return `outbound-local-${Date.now()}-${++fallbackId}`;
}

function validReference(value: unknown): value is OutboundReference {
  if (!isObject(value) || typeof value.kind !== "string") return false;
  if (value.resolvedTags !== undefined && (!Array.isArray(value.resolvedTags) || value.resolvedTags.some((tag) => typeof tag !== "string"))) return false;
  if (value.kind === "builtin") return value.tag === "direct" || value.tag === "block";
  if (value.kind === "outbound") return typeof value.outboundId === "string" && typeof value.tag === "string";
  if (value.kind === "subscription") return typeof value.subscriptionId === "string" && typeof value.tag === "string";
  if (value.kind === "subscription-node") return typeof value.subscriptionId === "string" && typeof value.nodeTag === "string";
  if (value.kind === "raw" || value.kind === "missing") return typeof value.tag === "string";
  return false;
}

function validGroupState(value: unknown): value is OutboundGroupState {
  return isObject(value)
    && typeof value.id === "string"
    && typeof value.tag === "string"
    && structuredTypes.has(value.type as OutboundEditorType)
    && typeof value.hidden === "boolean"
    && typeof value.include === "string"
    && typeof value.exclude === "string"
    && typeof value.icon === "string"
    && Array.isArray(value.references)
    && value.references.every(validReference)
    && Array.isArray(value.runtimeTags)
    && value.runtimeTags.every((tag) => typeof tag === "string");
}

export function parseOutboundEditorState(value: unknown): OutboundEditorState | null {
  if (!isObject(value) || value.version !== 1 || !Array.isArray(value.groups) || !value.groups.every(validGroupState)) return null;
  if (!Array.isArray(value.generatedTags) || value.generatedTags.some((tag) => typeof tag !== "string")) return null;
  return cloneValue(value as unknown as OutboundEditorState);
}

function validArrayBuffer(source: string): boolean {
  const errors: ParseError[] = [];
  const tree = parseTree(source, errors, { allowTrailingComma: false, disallowComments: true });
  return errors.length === 0 && tree?.type === "array";
}

function editJson(source: string, path: (string | number)[], value: unknown, isArrayInsertion = false): string {
  if (value === undefined && path.length === 1 && typeof path[0] === "number") {
    const tree = parseTree(source);
    const elements = tree?.type === "array" ? tree.children ?? [] : [];
    const index = path[0];
    const element = elements[index];
    if (!tree || !element) throw new Error("出站索引超出范围");
    // Token boundaries also work for compact arrays: jsonc-parser's final
    // element removal otherwise leaves a closing brace when no newline exists.
    const start = elements.length === 1 ? tree.offset + 1
      : index === elements.length - 1 ? elements[index - 1]!.offset + elements[index - 1]!.length : element.offset;
    const end = elements.length === 1 ? tree.offset + tree.length - 1
      : index === elements.length - 1 ? element.offset + element.length : elements[index + 1]!.offset;
    return source.slice(0, start) + source.slice(end);
  }
  return applyEdits(source, modify(source, path, value, { formattingOptions, isArrayInsertion }));
}

/** Updates known outbound-tag reference fields without reserializing JSON. */
export function renameOutboundTagInBuffer(source: string, module: string, from: string, to: string): string {
  if (!from || from === to) return source;
  const errors: ParseError[] = [];
  const tree = parseTree(source, errors, { allowTrailingComma: false, disallowComments: true });
  if (!tree || errors.length) return source;
  const paths: Array<Array<string | number>> = [];

  const visit = (node: Node, path: Array<string | number>) => {
    if (node.type === "array") {
      node.children?.forEach((child, index) => visit(child, [...path, index]));
      return;
    }
    if (node.type !== "object") return;
    for (const property of node.children ?? []) {
      const keyNode = property.children?.[0];
      const valueNode = property.children?.[1];
      if (!keyNode || !valueNode || typeof keyNode.value !== "string") continue;
      const key = keyNode.value;
      const valuePath = [...path, key];
      const scalarReference = key === "detour"
        || key === "download_detour"
        || key === "external_ui_download_detour"
        || (module === "route" && key === "outbound")
        || (module === "route" && path.length === 0 && key === "final")
        || (module === "outbounds" && key === "default");
      if (scalarReference && valueNode.type === "string" && valueNode.value === from) paths.push(valuePath);
      if (module === "outbounds" && key === "outbounds" && valueNode.type === "array") {
        valueNode.children?.forEach((child, index) => {
          if (child.type === "string" && child.value === from) paths.push([...valuePath, index]);
        });
      }
      visit(valueNode, valuePath);
    }
  };
  visit(tree, []);
  return paths.reduce((text, path) => editJson(text, path, to), source);
}

/**
 * Applies the UI model to sing-box outbounds. Existing unrelated items and
 * unknown tokens remain untouched; only managed groups and generated backing
 * nodes are changed.
 */
export function removeOutboundGroup(groups: OutboundEditorGroup[], id: string): OutboundEditorGroup[] {
  const removed = groups.find((group) => group.id === id);
  if (!removed) return groups.map((group) => cloneValue(group));
  return groups.flatMap((group) => {
    if (group.id === id) return [];
    const references = group.references.map((reference) => reference.kind === "outbound" && reference.outboundId === id
      ? { kind: "missing", tag: removed.tag, resolvedTags: [removed.tag] } as OutboundReference
      : cloneValue(reference));
    return [{ ...cloneValue(group), references }];
  });
}

export function updateOutboundGroup(groups: OutboundEditorGroup[], next: OutboundEditorGroup): OutboundEditorGroup[] {
  const index = groups.findIndex((group) => group.id === next.id);
  if (index < 0) return [cloneValue(next), ...groups.map((group) => cloneValue(group))];
  return groups.map((group, current) => current === index ? cloneValue(next) : cloneValue(group));
}
