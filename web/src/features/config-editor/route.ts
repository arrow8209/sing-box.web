import { findNodeAtLocation, parse, parseTree, type Node, type ParseError } from "jsonc-parser";
import type { RuleBinding } from "@/features/profiles/types";
import type { RuleResource } from "@/features/rule-resources/types";
import { setJsonPath } from "./json";

export type RouteRule = Record<string, unknown>;

export const routeRuleTypeOptions = [
  { value: "inbound", label: "入站 (inbound)" },
  { value: "network", label: "网络 (network)" },
  { value: "protocol", label: "协议 (protocol)" },
  { value: "domain", label: "域名 (domain)" },
  { value: "domain_suffix", label: "域名后缀 (domain_suffix)" },
  { value: "domain_keyword", label: "域名关键词 (domain_keyword)" },
  { value: "domain_regex", label: "域名正则 (domain_regex)" },
  { value: "source_ip_cidr", label: "源 IP 地址段 (source_ip_cidr)" },
  { value: "ip_cidr", label: "IP 地址段 (ip_cidr)" },
  { value: "ip_is_private", label: "是否为私有 IP (ip_is_private)" },
  { value: "source_port", label: "源端口 (source_port)" },
  { value: "source_port_range", label: "源端口范围 (source_port_range)" },
  { value: "port", label: "端口 (port)" },
  { value: "port_range", label: "端口范围 (port_range)" },
  { value: "process_name", label: "进程名称 (process_name)" },
  { value: "process_path", label: "进程路径 (process_path)" },
  { value: "process_path_regex", label: "进程路径正则 (process_path_regex)" },
  { value: "clash_mode", label: "Clash 模式 (clash_mode)" },
  { value: "rule_set", label: "规则集 (rule_set)" },
  { value: "inline", label: "内联 (inline)" }
] as const;

export type RouteRuleType = typeof routeRuleTypeOptions[number]["value"];

export const routeRuleActionOptions = [
  { value: "route", label: "路由", shortLabel: "路由", icon: "route" },
  { value: "route-options", label: "路由设置选项", shortLabel: "路由选项", icon: "config" },
  { value: "reject", label: "拒绝连接", shortLabel: "拒绝", icon: "shield" },
  { value: "hijack-dns", label: "劫持 DNS 请求", shortLabel: "DNS 劫持", icon: "dns" },
  { value: "sniff", label: "协议嗅探", shortLabel: "嗅探", icon: "search" },
  { value: "resolve", label: "解析 DNS", shortLabel: "DNS 解析", icon: "globe" }
] as const;

export interface RouteRuleSetOption {
  id: string;
  tag: string;
  name: string;
  kind: "managed" | "native" | "missing";
  detail: string;
  resourceId?: string;
  disabled: boolean;
}

export interface RouteRuleEditorResult {
  rule: RouteRule;
  text: string;
  resourceIds: string[];
}

export interface DisabledRouteRuleState {
  id: string;
  position: number;
  text: string;
}

export interface RouteEditorState {
  version: 1;
  insertionPosition: number;
  disabledRules: DisabledRouteRuleState[];
}

export interface RoutePanelSave {
  text: string;
  bindings: RuleBinding[];
  editorState: RouteEditorState;
}

const listRuleTypes = new Set<RouteRuleType>([
  "network", "protocol", "domain", "domain_suffix", "domain_keyword", "domain_regex",
  "source_ip_cidr", "ip_cidr", "source_port_range", "port_range", "process_name",
  "process_path", "process_path_regex", "rule_set"
]);
let fallbackStateId = 0;

export function isRouteObject(value: unknown): value is RouteRule {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function parseRouteRuleText(text: string): RouteRule | null {
  const errors: ParseError[] = [];
  const value = parse(text, errors, { allowTrailingComma: false, disallowComments: true });
  return errors.length === 0 && isRouteObject(value) ? value : null;
}

export function createRouteStateId(): string {
  const bytes = new Uint8Array(8);
  globalThis.crypto?.getRandomValues?.(bytes);
  const random = [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
  return random && !/^0+$/.test(random) ? `route-rule-${random}` : `route-rule-local-${++fallbackStateId}`;
}

export function restoreRouteEditorState(value: unknown, activeRuleCount: number): RouteEditorState {
  const source = isRouteObject(value) && value.version === 1 ? value : {};
  const seenIds = new Set<string>();
  const candidates = Array.isArray(source.disabledRules) ? source.disabledRules.flatMap((item): DisabledRouteRuleState[] => {
    if (!isRouteObject(item) || typeof item.id !== "string" || !item.id || !Number.isInteger(item.position) || typeof item.text !== "string" || !parseRouteRuleText(item.text)) return [];
    if (seenIds.has(item.id)) return [];
    seenIds.add(item.id);
    return [{ id: item.id, position: Number(item.position), text: item.text }];
  }) : [];
  const slots: Array<{ disabled?: DisabledRouteRuleState }> = Array.from({ length: activeRuleCount }, () => ({}));
  for (const item of candidates.sort((left, right) => left.position - right.position)) {
    slots.splice(Math.max(0, Math.min(item.position, slots.length)), 0, { disabled: item });
  }
  const disabledRules = slots.flatMap((slot, position) => slot.disabled ? [{ ...slot.disabled, position }] : []);
  const total = activeRuleCount + disabledRules.length;
  const fallbackInsertion = Math.min(4, total);
  const requestedInsertion = Number.isInteger(source.insertionPosition) ? Number(source.insertionPosition) : fallbackInsertion;
  return {
    version: 1,
    insertionPosition: Math.max(0, Math.min(requestedInsertion, total)),
    disabledRules
  };
}

export function routeRules(route: unknown): unknown[] {
  return isRouteObject(route) && Array.isArray(route.rules) ? route.rules : [];
}

export function routeRuleSetTags(rule: unknown): string[] {
  if (!isRouteObject(rule)) return [];
  if (typeof rule.rule_set === "string") return rule.rule_set.trim() ? [rule.rule_set.trim()] : [];
  if (!Array.isArray(rule.rule_set)) return [];
  return [...new Set(rule.rule_set.flatMap((tag) => typeof tag === "string" && tag.trim() ? [tag.trim()] : []))];
}

export function routeRuleType(rule: unknown): RouteRuleType {
  if (!isRouteObject(rule)) return "inline";
  return routeRuleTypeOptions.find((option) => option.value !== "inline" && Object.prototype.hasOwnProperty.call(rule, option.value))?.value ?? "inline";
}

export function routeRuleAction(rule: unknown): string {
  return isRouteObject(rule) && typeof rule.action === "string" && rule.action ? rule.action : "route";
}

export function routeRulePayloadText(rule: unknown, type = routeRuleType(rule)): string {
  if (!isRouteObject(rule)) return "";
  if (type === "inline") return JSON.stringify(rule, null, 2);
  const value = rule[type];
  if (Array.isArray(value)) return value.map((item) => typeof item === "object" && item !== null ? JSON.stringify(item) : String(item)).join(",");
  if (typeof value === "object" && value !== null) return JSON.stringify(value);
  return String(value ?? "");
}

export function routeRulePayloadValue(type: RouteRuleType, text: string, selectedTags: string[], original: unknown): unknown {
  if (type === "rule_set") return [...new Set(selectedTags)];
  if (type === "ip_is_private") return text !== "false";
  if (type === "inbound" || type === "clash_mode") {
    const values = text.split(",").map((item) => item.trim()).filter(Boolean);
    return Array.isArray(original) ? values : values[0];
  }
  if (type === "source_port" || type === "port") {
    const values = text.split(",").map((item) => Number(item.trim())).filter((item) => Number.isInteger(item));
    return Array.isArray(original) || values.length > 1 ? values : values[0];
  }
  if (listRuleTypes.has(type)) {
    const values = text.split(",").map((item) => item.trim()).filter(Boolean);
    return Array.isArray(original) || original === undefined || values.length > 1 ? values : values[0];
  }
  return text.trim();
}

function nativeRuleSets(route: unknown): RouteRuleSetOption[] {
  if (!isRouteObject(route) || !Array.isArray(route.rule_set)) return [];
  return route.rule_set.flatMap((item, index) => {
    if (!isRouteObject(item) || typeof item.tag !== "string" || !item.tag.trim()) return [];
    const type = typeof item.type === "string" ? item.type : "local";
    return [{
      id: `native:${index}:${item.tag}`,
      tag: item.tag.trim(),
      name: item.tag.trim(),
      kind: "native" as const,
      detail: `${type} · 配置内规则集`,
      disabled: type === "remote"
    }];
  });
}

function uniqueTag(preferred: string, reserved: Set<string>): string {
  const base = preferred.trim() || "规则集";
  if (!reserved.has(base)) return base;
  let suffix = 2;
  while (reserved.has(`${base}-${suffix}`)) suffix++;
  return `${base}-${suffix}`;
}

export function buildRouteRuleSetOptions(
  route: unknown,
  bindings: RuleBinding[],
  resources: RuleResource[]
): RouteRuleSetOption[] {
  const native = nativeRuleSets(route);
  const reserved = new Set([
    ...native.map((option) => option.tag),
    ...bindings.flatMap((binding) => binding.tag.trim() ? [binding.tag.trim()] : [])
  ]);
  const bindingByResource = new Map(bindings.map((binding) => [binding.resourceId, binding]));
  const managed = resources.map((resource): RouteRuleSetOption => {
    const binding = bindingByResource.get(resource.id);
    const tag = binding?.tag.trim() || uniqueTag(resource.name, reserved);
    reserved.add(tag);
    return {
      id: `managed:${resource.id}`,
      tag,
      name: resource.name,
      kind: "managed",
      detail: `${resource.sourceType === "remote" ? "网络" : "手工"} · ${resource.format === "binary" ? "Binary" : "Source"}`,
      resourceId: resource.id,
      disabled: resource.disabled
    };
  });
  const knownTags = new Set([...native, ...managed].map((option) => option.tag));
  const missing = routeRules(route).flatMap(routeRuleSetTags).flatMap((tag): RouteRuleSetOption[] => {
    if (knownTags.has(tag)) return [];
    knownTags.add(tag);
    return [{ id: `missing:${tag}`, tag, name: tag, kind: "missing", detail: "当前 JSON 引用", disabled: false }];
  });
  return [...native, ...managed, ...missing];
}

function taggedItems(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((item) => isRouteObject(item) && typeof item.tag === "string" && item.tag.trim() ? [item.tag.trim()] : []);
}

export function routeOutboundTags(config: Record<string, unknown>): string[] {
  return [...new Set(taggedItems(config.outbounds))];
}

export function routeInboundTags(config: Record<string, unknown>): string[] {
  return [...new Set(taggedItems(config.inbounds))];
}

export function routeDnsServerTags(config: Record<string, unknown>): string[] {
  const dns = isRouteObject(config.dns) ? config.dns : {};
  return [...new Set(taggedItems(dns.servers))];
}

export function routeDefaultDomainResolverTag(route: unknown): string {
  if (!isRouteObject(route)) return "";
  if (typeof route.default_domain_resolver === "string") return route.default_domain_resolver;
  return isRouteObject(route.default_domain_resolver) && typeof route.default_domain_resolver.server === "string"
    ? route.default_domain_resolver.server
    : "";
}

export function mergeSelectedRuleBindings(
  bindings: RuleBinding[],
  options: RouteRuleSetOption[],
  resourceIds: string[]
): RuleBinding[] {
  const selected = new Set(resourceIds);
  const next = bindings.map((binding) => ({ ...binding }));
  for (const option of options) {
    if (!option.resourceId || !selected.has(option.resourceId)) continue;
    const index = next.findIndex((binding) => binding.resourceId === option.resourceId);
    if (index >= 0) next[index] = { ...next[index]!, tag: option.tag, enabled: true };
    else next.push({ resourceId: option.resourceId, tag: option.tag, enabled: true });
  }
  return next;
}

function sourceText(text: string): string {
  return text.trim() ? text : "{}";
}

function routeRulesNode(text: string): Node | undefined {
  const tree = parseTree(text, [], { allowTrailingComma: false, disallowComments: true });
  const node = tree ? findNodeAtLocation(tree, ["rules"]) : undefined;
  return node?.type === "array" ? node : undefined;
}

function lineIndent(text: string, offset: number): string {
  const lineStart = text.lastIndexOf("\n", Math.max(0, offset - 1)) + 1;
  return text.slice(lineStart, offset).match(/^\s*/)?.[0] ?? "";
}

function indentRuleText(raw: string, indent: string): string {
  const lines = raw.trim().split(/\r?\n/);
  if (lines.length === 1) return indent + lines[0];
  const tailIndents = lines.slice(1).filter((line) => line.trim()).map((line) => line.match(/^\s*/)?.[0].length ?? 0);
  const trim = tailIndents.length ? Math.min(...tailIndents) : 0;
  return lines.map((line, index) => indent + (index ? line.slice(Math.min(trim, line.length)) : line)).join("\n");
}

function rawRuleElements(text: string, node: Node): string[] {
  return (node.children ?? []).map((child) => text.slice(child.offset, child.offset + child.length));
}

function replaceRawRuleElements(text: string, elements: string[]): string {
  let source = sourceText(text);
  let node = routeRulesNode(source);
  if (!node) {
    const tree = parseTree(source, [], { allowTrailingComma: false, disallowComments: true });
    const existing = tree ? findNodeAtLocation(tree, ["rules"]) : undefined;
    if (existing && existing.type !== "array") throw new Error("route.rules 必须是数组");
    source = setJsonPath(source, ["rules"], []);
    node = routeRulesNode(source);
  }
  if (!node) throw new Error("route.rules 必须是数组");
  const indent = lineIndent(source, node.parent?.offset ?? node.offset);
  const childIndent = `${indent}  `;
  const replacement = elements.length
    ? `[\n${elements.map((item) => indentRuleText(item, childIndent)).join(",\n")}\n${indent}]`
    : "[]";
  return source.slice(0, node.offset) + replacement + source.slice(node.offset + node.length);
}

export function routeRuleText(text: string, index: number): string | null {
  const source = sourceText(text);
  const node = routeRulesNode(source);
  const child = node?.children?.[index];
  return child ? source.slice(child.offset, child.offset + child.length) : null;
}

export function upsertRouteRuleText(text: string, index: number, ruleText: string): string {
  if (!parseRouteRuleText(ruleText)) throw new Error("路由规则必须是 JSON 对象");
  const source = sourceText(text);
  const node = routeRulesNode(source);
  const elements = node ? rawRuleElements(source, node) : [];
  if (index < 0 || index > elements.length) throw new Error("路由规则索引超出范围");
  if (index === elements.length) elements.push(ruleText);
  else elements[index] = ruleText;
  return replaceRawRuleElements(source, elements);
}

export function insertRouteRuleText(text: string, index: number, ruleText: string): string {
  if (!parseRouteRuleText(ruleText)) throw new Error("路由规则必须是 JSON 对象");
  const source = sourceText(text);
  const node = routeRulesNode(source);
  const elements = node ? rawRuleElements(source, node) : [];
  if (index < 0 || index > elements.length) throw new Error("路由规则索引超出范围");
  elements.splice(index, 0, ruleText);
  return replaceRawRuleElements(source, elements);
}

export function upsertRouteRule(text: string, index: number, rule: RouteRule): string {
  return upsertRouteRuleText(text, index, JSON.stringify(rule, null, 2));
}

export function removeRouteRule(text: string, index: number): string {
  return setJsonPath(sourceText(text), ["rules", index], undefined);
}

export function reorderRouteRules(text: string, orderedIndexes: number[]): string {
  const source = sourceText(text);
  const node = routeRulesNode(source);
  if (!node) return source;
  const elements = rawRuleElements(source, node);
  const valid = orderedIndexes.length === elements.length
    && new Set(orderedIndexes).size === elements.length
    && orderedIndexes.every((index) => Number.isInteger(index) && index >= 0 && index < elements.length);
  if (!valid) throw new Error("路由规则排序必须包含全部规则");
  if (orderedIndexes.every((value, index) => value === index)) return source;
  return replaceRawRuleElements(source, orderedIndexes.map((index) => elements[index]!));
}

export function updateRouteFinal(text: string, tag: string): string {
  return setJsonPath(sourceText(text), ["final"], tag || undefined);
}

export function updateRouteBoolean(text: string, key: "find_process" | "auto_detect_interface", value: boolean): string {
  return setJsonPath(sourceText(text), [key], key === "find_process" && !value ? undefined : value);
}

export function updateRouteString(text: string, key: "default_interface", value: string): string {
  return setJsonPath(sourceText(text), [key], value.trim() || undefined);
}

export function updateRouteDefaultDomainResolver(text: string, route: unknown, tag: string): string {
  const source = sourceText(text);
  if (isRouteObject(route) && typeof route.default_domain_resolver === "string") {
    return setJsonPath(source, ["default_domain_resolver"], tag || undefined);
  }
  if (isRouteObject(route) && isRouteObject(route.default_domain_resolver)) {
    if (!tag && Object.keys(route.default_domain_resolver).every((key) => key === "server")) {
      return setJsonPath(source, ["default_domain_resolver"], undefined);
    }
    return setJsonPath(source, ["default_domain_resolver", "server"], tag || undefined);
  }
  return setJsonPath(source, ["default_domain_resolver"], tag ? { server: tag } : undefined);
}
