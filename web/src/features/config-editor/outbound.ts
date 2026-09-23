import { applyEdits, modify, parseTree, type Node, type ParseError } from "jsonc-parser";
import type { SubscriptionDetail } from "@/features/subscriptions/types";

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

export interface OutboundBufferResult {
  text: string;
  state: OutboundEditorState;
  groups: OutboundEditorGroup[];
  generatedCount: number;
}

export interface OutboundPanelSave {
  text: string;
  editorState: OutboundEditorState;
  renamed?: { from: string; to: string };
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
const structuredKeys = [
  "type", "tag", "hidden", "include", "exclude", "icon",
  "interrupt_exist_connections", "outbounds", "url", "interval", "tolerance"
] as const;
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

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function booleanValue(value: unknown): boolean {
  return value === true;
}

function uniqueStrings(values: Iterable<string>): string[] {
  return [...new Set([...values].filter(Boolean))];
}

function sameStrings(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function sameValue(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left)
      && Array.isArray(right)
      && left.length === right.length
      && left.every((item, index) => sameValue(item, right[index]));
  }
  if (!isObject(left) || !isObject(right)) return false;
  const leftKeys = Object.keys(left).sort();
  const rightKeys = Object.keys(right).sort();
  return leftKeys.length === rightKeys.length
    && leftKeys.every((key, index) => key === rightKeys[index] && sameValue(left[key], right[key]));
}

export function sameOutboundIgnoringTag(left: OutboundConfig, right: OutboundConfig): boolean {
  const { tag: _leftTag, ...leftFields } = left;
  const { tag: _rightTag, ...rightFields } = right;
  return sameValue(leftFields, rightFields);
}

/** Finds expanded subscription nodes that belong behind a selector/urltest card. */
export function referencedSubscriptionOutboundTags(source: unknown[], subscriptionNodes: unknown[]): string[] {
  const outbounds = source.filter(isOutboundConfig);
  const referenced = new Set(outbounds.flatMap((config) => isPolicyOutbound(config) ? outboundReferenceTags(config) : []));
  const candidates = subscriptionNodes.filter(isOutboundConfig);
  return outbounds.flatMap((config) => referenced.has(config.tag)
    && candidates.some((candidate) => candidate.tag === config.tag || sameOutboundIgnoringTag(config, candidate))
    ? [config.tag]
    : []);
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

function restoredOutboundGroupId(config: OutboundConfig, used: Set<string>): string {
  const seed = `${config.type}\u0000${config.tag}`;
  let hash = 0x811c9dc5;
  for (let index = 0; index < seed.length; index++) {
    hash ^= seed.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  const base = `outbound-restored-${(hash >>> 0).toString(16).padStart(8, "0")}`;
  if (!used.has(base)) return base;
  let suffix = 2;
  while (used.has(`${base}-${suffix}`)) suffix++;
  return `${base}-${suffix}`;
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

function isBuiltinConfig(config: OutboundConfig): boolean {
  return (config.type === "direct" && config.tag === "direct") || (config.type === "block" && config.tag === "block");
}

function subscriptionNodes(subscription: SubscriptionDetail): OutboundConfig[] {
  return subscription.outbounds.filter(isOutboundConfig).map((node) => cloneValue(node));
}

function findReferencedNodeTag(
  references: string[],
  claimed: Set<number>,
  byTag: Map<string, OutboundConfig>,
  node: OutboundConfig
): number {
  const exact = references.findIndex((tag, index) => !claimed.has(index) && tag === node.tag && byTag.has(tag));
  if (exact >= 0) return exact;
  return references.findIndex((tag, index) => {
    if (claimed.has(index)) return false;
    const config = byTag.get(tag);
    return Boolean(config && sameOutboundIgnoringTag(config, node));
  });
}

function restoreReferences(
  config: OutboundConfig,
  groups: OutboundGroupState[],
  rawConfigs: OutboundConfig[],
  subscriptions: SubscriptionDetail[]
): OutboundReference[] {
  const tags = outboundReferenceTags(config);
  const byTag = new Map(rawConfigs.map((item) => [item.tag, item]));
  const groupByTag = new Map(groups.map((group) => [group.tag, group]));
  const claimed = new Set<number>();
  const collapsed = new Map<number, { reference: OutboundReference; indexes: number[] }>();
  const matches = smartMatcher(stringValue(config.include), stringValue(config.exclude));

  for (const subscription of subscriptions) {
    const nodes = subscriptionNodes(subscription).filter((node) => matches(node.tag));
    if (!nodes.length) continue;
    const indexes: number[] = [];
    const locallyClaimed = new Set(claimed);
    for (const node of nodes) {
      const index = findReferencedNodeTag(tags, locallyClaimed, byTag, node);
      if (index < 0) {
        indexes.length = 0;
        break;
      }
      indexes.push(index);
      locallyClaimed.add(index);
    }
    if (!indexes.length) continue;
    indexes.forEach((index) => claimed.add(index));
    const ordered = [...indexes].sort((left, right) => left - right);
    collapsed.set(ordered[0]!, {
      reference: { kind: "subscription", subscriptionId: subscription.id, tag: subscription.name, resolvedTags: ordered.map((index) => tags[index]!) },
      indexes: ordered
    });
  }

  const result: OutboundReference[] = [];
  const consumed = new Set<number>();
  for (let index = 0; index < tags.length; index++) {
    const whole = collapsed.get(index);
    if (whole) {
      result.push(whole.reference);
      whole.indexes.forEach((item) => consumed.add(item));
      continue;
    }
    if (consumed.has(index)) continue;
    const tag = tags[index]!;
    if (tag === "direct" || tag === "block") {
      result.push({ kind: "builtin", tag, resolvedTags: [tag] });
      continue;
    }
    const target = groupByTag.get(tag);
    if (target) {
      result.push({ kind: "outbound", outboundId: target.id, tag: target.tag, resolvedTags: [tag] });
      continue;
    }
    let nodeReference: OutboundReference | null = null;
    const raw = byTag.get(tag);
    if (raw) {
      for (const subscription of subscriptions) {
        const node = subscriptionNodes(subscription).find((candidate) => candidate.tag === tag || sameOutboundIgnoringTag(raw, candidate));
        if (node) {
          nodeReference = { kind: "subscription-node", subscriptionId: subscription.id, nodeTag: node.tag, resolvedTags: [tag] };
          break;
        }
      }
    }
    result.push(nodeReference ?? (raw
      ? { kind: "raw", tag, resolvedTags: [tag] }
      : { kind: "missing", tag, resolvedTags: [tag] }));
  }
  return result;
}

function trustedStoredGroup(stored: OutboundGroupState | undefined, config: OutboundConfig): boolean {
  return Boolean(stored
    && stored.type === config.type
    && sameStrings(stored.runtimeTags, outboundReferenceTags(config)));
}

/** Restores the source project's editor model from runtime JSON when metadata is absent or stale. */
export function restoreOutboundEditorState(
  source: unknown[],
  subscriptions: SubscriptionDetail[],
  storedValue?: unknown
): OutboundEditorState {
  const configs = source.filter(isOutboundConfig);
  const stored = parseOutboundEditorState(storedValue);
  const subscriptionGenerated = new Set(referencedSubscriptionOutboundTags(
    source,
    subscriptions.flatMap((subscription) => subscription.outbounds)
  ));
  const storedGenerated = new Set(stored?.generatedTags ?? []);
  const generated = new Set(configs.flatMap((config) => isBuiltinConfig(config)
    || subscriptionGenerated.has(config.tag)
    || storedGenerated.has(config.tag)
    ? [config.tag]
    : []));
  const storedByTag = new Map((stored?.groups ?? []).map((group) => [group.tag, group]));
  const usedIds = new Set<string>();
  const groupConfigs = configs.filter((config) => isManagedOutbound(config) && !generated.has(config.tag));
  const groups = groupConfigs.map((config) => {
    const previous = storedByTag.get(config.tag);
    let id = previous?.id ?? restoredOutboundGroupId(config, usedIds);
    if (usedIds.has(id)) id = createOutboundGroupId(usedIds);
    usedIds.add(id);
    return {
      id,
      tag: config.tag,
      type: config.type as OutboundEditorType,
      hidden: previous?.hidden ?? booleanValue(config.hidden),
      include: previous?.include ?? stringValue(config.include),
      exclude: previous?.exclude ?? stringValue(config.exclude),
      icon: previous?.icon ?? stringValue(config.icon),
      references: [] as OutboundReference[],
      runtimeTags: outboundReferenceTags(config)
    };
  });

  groups.forEach((group, index) => {
    const config = groupConfigs[index]!;
    const previous = storedByTag.get(group.tag);
    group.references = trustedStoredGroup(previous, config)
      ? cloneValue(previous!.references)
      : restoreReferences(config, groups, configs, subscriptions);
  });
  return { version: 1, groups, generatedTags: [...generated] };
}

export function outboundEditorGroups(source: unknown[], state: OutboundEditorState): OutboundEditorGroup[] {
  const byTag = new Map(source.filter(isOutboundConfig).map((config) => [config.tag, config]));
  return state.groups.flatMap((group) => {
    const config = byTag.get(group.tag);
    return config && isManagedOutbound(config) ? [{ ...cloneValue(group), config: cloneValue(config) }] : [];
  });
}

function smartMatcher(include: string, exclude: string): (tag: string) => boolean {
  const makeRegex = (pattern: string): RegExp | null => {
    if (!pattern) return null;
    try {
      return new RegExp(pattern);
    } catch {
      return new RegExp(pattern.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
    }
  };
  const included = makeRegex(include);
  const excluded = makeRegex(exclude);
  return (tag) => (included ? included.test(tag) : true) && !(excluded?.test(tag) ?? false);
}

function referencedTag(value: unknown, mapping: Map<string, string>): unknown {
  return typeof value === "string" && mapping.has(value) ? mapping.get(value) : value;
}

function remapImportedReferences(config: OutboundConfig, mapping: Map<string, string>): void {
  config.detour = referencedTag(config.detour, mapping);
  config.default = referencedTag(config.default, mapping);
  if (Array.isArray(config.outbounds)) config.outbounds = config.outbounds.map((tag) => referencedTag(tag, mapping));
}

function fallbackTags(reference: OutboundReference): string[] {
  if (reference.resolvedTags?.length) return reference.resolvedTags;
  if (reference.kind === "builtin" || reference.kind === "raw" || reference.kind === "missing") return [reference.tag];
  if (reference.kind === "outbound" || reference.kind === "subscription") return [reference.tag];
  return [reference.nodeTag];
}

function cleanGroupConfig(group: OutboundEditorGroup, tags: string[]): OutboundConfig {
  const config = cloneValue(group.config) as Record<string, unknown>;
  config.type = group.type;
  config.tag = group.tag;
  delete config.hidden;
  delete config.include;
  delete config.exclude;
  delete config.icon;
  if (group.type === "direct" || group.type === "block") {
    for (const key of ["interrupt_exist_connections", "outbounds", "url", "interval", "tolerance"]) delete config[key];
    return config as OutboundConfig;
  }
  config.interrupt_exist_connections = typeof config.interrupt_exist_connections === "boolean" ? config.interrupt_exist_connections : true;
  config.outbounds = tags;
  if (group.type === "urltest") {
    config.url = typeof config.url === "string" && config.url ? config.url : "https://www.gstatic.com/generate_204";
    config.interval = typeof config.interval === "string" && config.interval ? config.interval : "3m";
    config.tolerance = typeof config.tolerance === "number" ? config.tolerance : 150;
  } else {
    delete config.url;
    delete config.interval;
    delete config.tolerance;
  }
  return config as OutboundConfig;
}

interface MaterializedOutbounds {
  configs: OutboundConfig[];
  groups: OutboundEditorGroup[];
  generated: OutboundConfig[];
  generatedTags: string[];
}

function materializeOutbounds(
  groups: OutboundEditorGroup[],
  subscriptions: SubscriptionDetail[],
  reservedTags: Iterable<string>,
  fallbackConfigs: Map<string, OutboundConfig>
): MaterializedOutbounds {
  const subscriptionById = new Map(subscriptions.map((subscription) => [subscription.id, subscription]));
  const groupById = new Map(groups.map((group) => [group.id, group]));
  const used = new Set([...reservedTags, ...groups.map((group) => group.tag)]);
  const generated: OutboundConfig[] = [];
  const generatedSubscriptions: Array<string | null> = [];
  const allocated = new Map<string, string>();
  const subscriptionMappings = new Map<string, Map<string, string>>();

  const addBuiltin = (tag: "direct" | "block") => {
    const key = `builtin:${tag}`;
    if (allocated.has(key) || used.has(tag)) return tag;
    generated.push({ type: tag, tag });
    generatedSubscriptions.push(null);
    allocated.set(key, tag);
    used.add(tag);
    return tag;
  };
  const addNode = (subscription: SubscriptionDetail, node: OutboundConfig, index: number) => {
    const key = `subscription-node:${subscription.id}:${index}`;
    const existing = allocated.get(key);
    if (existing) return existing;
    const resolved = uniqueOutboundTag(node.tag, used);
    const config = cloneValue(node);
    config.tag = resolved;
    generated.push(config);
    generatedSubscriptions.push(subscription.id);
    allocated.set(key, resolved);
    used.add(resolved);
    const mapping = subscriptionMappings.get(subscription.id) ?? new Map<string, string>();
    if (!mapping.has(node.tag)) mapping.set(node.tag, resolved);
    subscriptionMappings.set(subscription.id, mapping);
    return resolved;
  };
  const keepFallback = (tag: string) => {
    if (used.has(tag) || allocated.has(`fallback:${tag}`)) return tag;
    const config = fallbackConfigs.get(tag);
    if (config) {
      generated.push(cloneValue(config));
      generatedSubscriptions.push(null);
    }
    allocated.set(`fallback:${tag}`, tag);
    used.add(tag);
    return tag;
  };

  const nextGroups = groups.map((group) => {
    const matcher = smartMatcher(group.include, group.exclude);
    const runtimeTags: string[] = [];
    const references = group.references.map((reference) => {
      if (group.type !== "selector" && group.type !== "urltest") return cloneValue(reference);
      const nextReference = cloneValue(reference);
      let resolved: string[] = [];
      if (reference.kind === "builtin") {
        resolved = [addBuiltin(reference.tag)];
      } else if (reference.kind === "outbound") {
        const target = groupById.get(reference.outboundId);
        resolved = target && target.id !== group.id ? [target.tag] : fallbackTags(reference);
        if (target && nextReference.kind === "outbound") nextReference.tag = target.tag;
      } else if (reference.kind === "subscription") {
        const subscription = subscriptionById.get(reference.subscriptionId);
        if (subscription) {
          if (nextReference.kind === "subscription") nextReference.tag = subscription.name;
          resolved = subscriptionNodes(subscription).flatMap((node, index) => matcher(node.tag) ? [addNode(subscription, node, index)] : []);
        } else {
          resolved = fallbackTags(reference).map(keepFallback);
        }
      } else if (reference.kind === "subscription-node") {
        const subscription = subscriptionById.get(reference.subscriptionId);
        const nodes = subscription ? subscriptionNodes(subscription) : [];
        const index = nodes.findIndex((node) => node.tag === reference.nodeTag);
        if (subscription && index >= 0) {
          resolved = matcher(nodes[index]!.tag) ? [addNode(subscription, nodes[index]!, index)] : [];
        } else {
          resolved = fallbackTags(reference).map(keepFallback);
        }
      } else {
        resolved = fallbackTags(reference);
      }
      runtimeTags.push(...resolved);
      return { ...nextReference, resolvedTags: uniqueStrings(resolved) } as OutboundReference;
    });
    const tags = uniqueStrings(runtimeTags).filter((tag) => tag !== group.tag);
    const next = { ...cloneValue(group), references, runtimeTags: tags };
    next.config = cleanGroupConfig(next, tags);
    return next;
  });

  generated.forEach((config, index) => {
    const subscriptionId = generatedSubscriptions[index];
    const mapping = subscriptionId ? subscriptionMappings.get(subscriptionId) : undefined;
    if (mapping) remapImportedReferences(config, mapping);
  });
  return {
    configs: nextGroups.map((group) => group.config),
    groups: nextGroups,
    generated,
    generatedTags: generated.map((config) => config.tag)
  };
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
export function applyOutboundEditorState(
  sourceText: string,
  sourceItems: unknown[],
  currentState: OutboundEditorState,
  nextGroupsInput: OutboundEditorGroup[],
  subscriptions: SubscriptionDetail[]
): OutboundBufferResult {
  const currentGroupTags = new Set(currentState.groups.map((group) => group.tag));
  const currentGeneratedTags = new Set(currentState.generatedTags);
  const rawConfigs = sourceItems.filter(isOutboundConfig);
  const fallbackConfigs = new Map(rawConfigs.filter((config) => currentGeneratedTags.has(config.tag)).map((config) => [config.tag, config]));
  const reservedTags = rawConfigs.flatMap((config) => currentGroupTags.has(config.tag) || currentGeneratedTags.has(config.tag) ? [] : [config.tag]);
  const materialized = materializeOutbounds(nextGroupsInput, subscriptions, reservedTags, fallbackConfigs);
  const state: OutboundEditorState = {
    version: 1,
    groups: materialized.groups.map(({ config: _config, ...group }) => cloneValue(group)),
    generatedTags: materialized.generatedTags
  };

  if (!validArrayBuffer(sourceText)) {
    const preserved = sourceItems.filter((item) => !isOutboundConfig(item)
      || (!currentGroupTags.has(item.tag) && !currentGeneratedTags.has(item.tag)));
    const items = [...materialized.configs, ...preserved, ...materialized.generated];
    return { text: JSON.stringify(items, null, 2), state, groups: materialized.groups, generatedCount: materialized.generated.length };
  }

  const currentById = new Map(currentState.groups.map((group) => [group.id, group]));
  const nextById = new Map(materialized.groups.map((group) => [group.id, group]));
  const rawIndexByTag = new Map(sourceItems.flatMap((item, index) => isOutboundConfig(item) ? [[item.tag, index] as const] : []));
  let text = sourceText;

  for (const [id, current] of currentById) {
    const next = nextById.get(id);
    const index = rawIndexByTag.get(current.tag);
    if (!next || index === undefined) continue;
    for (const key of structuredKeys) {
      const value = Object.prototype.hasOwnProperty.call(next.config, key) ? next.config[key] : undefined;
      text = editJson(text, [index, key], value);
    }
  }

  const removedIndexes = sourceItems.flatMap((item, index) => {
    if (!isOutboundConfig(item)) return [];
    const currentGroup = currentState.groups.find((group) => group.tag === item.tag);
    if (currentGroup && !nextById.has(currentGroup.id)) return [index];
    return currentGeneratedTags.has(item.tag) ? [index] : [];
  }).sort((left, right) => right - left);
  for (const index of removedIndexes) text = editJson(text, [index], undefined);

  const newGroups = materialized.groups.filter((group) => !currentById.has(group.id));
  for (const group of [...newGroups].reverse()) text = editJson(text, [0], group.config, true);

  let length = sourceItems.length - removedIndexes.length + newGroups.length;
  for (const generated of materialized.generated) {
    text = editJson(text, [length], generated);
    length++;
  }
  return { text, state, groups: materialized.groups, generatedCount: materialized.generated.length };
}

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
