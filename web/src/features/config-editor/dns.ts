import { findNodeAtLocation, parse, parseTree, type Node, type ParseError } from "jsonc-parser";
import type { RuleBinding } from "@/features/profiles/types";
import { setJsonPath } from "./json";

export type DnsObject = Record<string, unknown>;
export type DnsServer = DnsObject;
export type DnsRule = DnsObject;

export const dnsServerTypeOptions = [
  { value: "local", label: "本地", encrypted: false },
  { value: "hosts", label: "Hosts", encrypted: false },
  { value: "udp", label: "UDP", encrypted: false },
  { value: "tcp", label: "TCP", encrypted: false },
  { value: "tls", label: "TLS", encrypted: true },
  { value: "https", label: "HTTPS", encrypted: true },
  { value: "h3", label: "HTTP/3", encrypted: true },
  { value: "quic", label: "QUIC", encrypted: true },
  { value: "dhcp", label: "DHCP", encrypted: false },
  { value: "fakeip", label: "Fake-IP", encrypted: false },
  { value: "tailscale", label: "Tailscale", encrypted: false },
  { value: "resolved", label: "systemd-resolved", encrypted: false }
] as const;

export const dnsRuleTypeOptions = [
  { value: "inbound", label: "入站 (inbound)" },
  { value: "network", label: "网络 (network)" },
  { value: "protocol", label: "协议 (protocol)" },
  { value: "query_type", label: "查询类型 (query_type)" },
  { value: "domain", label: "域名 (domain)" },
  { value: "domain_suffix", label: "域名后缀 (domain_suffix)" },
  { value: "domain_keyword", label: "域名关键词 (domain_keyword)" },
  { value: "domain_regex", label: "域名正则 (domain_regex)" },
  { value: "source_ip_cidr", label: "源 IP 地址段 (source_ip_cidr)" },
  { value: "ip_cidr", label: "响应 IP 地址段 (ip_cidr)" },
  { value: "source_port", label: "源端口 (source_port)" },
  { value: "source_port_range", label: "源端口范围 (source_port_range)" },
  { value: "port", label: "目标端口 (port)" },
  { value: "port_range", label: "目标端口范围 (port_range)" },
  { value: "process_name", label: "进程名称 (process_name)" },
  { value: "process_path", label: "进程路径 (process_path)" },
  { value: "process_path_regex", label: "进程路径正则 (process_path_regex)" },
  { value: "package_name", label: "包名 (package_name)" },
  { value: "clash_mode", label: "Clash 模式 (clash_mode)" },
  { value: "rule_set", label: "规则集 (rule_set)" },
  { value: "inline", label: "完整 JSON (inline)" }
] as const;

export type DnsRuleType = typeof dnsRuleTypeOptions[number]["value"];

export const dnsRuleActionOptions = [
  { value: "route", label: "路由到服务器", shortLabel: "路由", icon: "route" },
  { value: "route-options", label: "设置解析选项", shortLabel: "解析选项", icon: "config" },
  { value: "reject", label: "拒绝 DNS 查询", shortLabel: "拒绝", icon: "shield" },
  { value: "predefined", label: "返回预定义响应", shortLabel: "预定义", icon: "dns" }
] as const;

export const defaultFakeIpDomainSuffixes = [
  ".lan",
  ".localdomain",
  ".example",
  ".invalid",
  ".localhost",
  ".test",
  ".local",
  ".home.arpa",
  ".msftconnecttest.com",
  ".msftncsi.com"
] as const;

export interface FakeIpDnsRuleParts {
  suffixes: string[];
  queryTypes: string[];
}

export interface DisabledDnsRuleState {
  id: string;
  position: number;
  text: string;
}

export interface DnsEditorState {
  version: 1;
  insertionPosition: number;
  disabledRules: DisabledDnsRuleState[];
}

export interface DnsPanelSave {
  text: string;
  bindings: RuleBinding[];
  editorState: DnsEditorState;
  renamed?: { from: string; to: string };
}

export interface DnsServerEditorResult {
  server: DnsServer;
  text: string;
}

export interface DnsRuleEditorResult {
  rule: DnsRule;
  text: string;
  resourceIds: string[];
}

const listRuleTypes = new Set<DnsRuleType>([
  "network", "protocol", "query_type", "domain", "domain_suffix", "domain_keyword",
  "domain_regex", "source_ip_cidr", "ip_cidr", "source_port_range", "port_range",
  "process_name", "process_path", "process_path_regex", "package_name", "rule_set"
]);
let fallbackStateId = 0;

export function isDnsObject(value: unknown): value is DnsObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function parseDnsObjectText(text: string): DnsObject | null {
  const errors: ParseError[] = [];
  const value = parse(text, errors, { allowTrailingComma: false, disallowComments: true });
  return errors.length === 0 && isDnsObject(value) ? value : null;
}

export function createDefaultFakeIpDnsRule(
  suffixes: readonly string[] = defaultFakeIpDomainSuffixes,
  queryTypes: readonly string[] = ["A", "AAAA"],
  server = "Fake-IP"
): DnsRule {
  return {
    action: "route",
    type: "logical",
    mode: "and",
    rules: [
      { domain_suffix: [...suffixes], invert: true },
      { query_type: [...queryTypes] }
    ],
    server
  };
}

export function fakeIpDnsRuleParts(rule: unknown): FakeIpDnsRuleParts | null {
  if (!isDnsObject(rule) || rule.action !== "route" || rule.type !== "logical" || rule.mode !== "and" || !Array.isArray(rule.rules) || rule.rules.length !== 2) return null;
  const [domainRule, queryRule] = rule.rules;
  if (!isDnsObject(domainRule) || domainRule.invert !== true || !Array.isArray(domainRule.domain_suffix)) return null;
  if (!isDnsObject(queryRule) || !Array.isArray(queryRule.query_type)) return null;
  return {
    suffixes: domainRule.domain_suffix.map(String),
    queryTypes: queryRule.query_type.map(String)
  };
}

export function isDefaultFakeIpDnsRule(rule: unknown): boolean {
  return fakeIpDnsRuleParts(rule) !== null;
}

export function parseFakeIpDomainSuffixes(value: unknown): string[] {
  return [...new Set(String(value ?? "")
    .split(/[\n,]+/)
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
    .map((item) => item.startsWith(".") ? item : `.${item}`))];
}

function disabledDnsRuleTexts(value: unknown): string[] {
  if (!isDnsObject(value) || value.version !== 1 || !Array.isArray(value.disabledRules)) return [];
  return value.disabledRules.flatMap((item) => isDnsObject(item) && typeof item.text === "string" ? [item.text] : []);
}

export function ensureDefaultFakeIpDnsRuleText(text: string, state?: unknown): string {
  const source = sourceText(text);
  const dns = parseDnsObjectText(source);
  if (!dns || (dns.rules !== undefined && !Array.isArray(dns.rules))) return source;
  if (dnsRules(dns).some(isDefaultFakeIpDnsRule)) return source;
  if (disabledDnsRuleTexts(state).some((item) => isDefaultFakeIpDnsRule(parseDnsObjectText(item)))) return source;
  const rules = dnsRules(dns);
  const remoteRuleIndex = rules.findIndex((rule) => isDnsObject(rule)
    && Array.isArray(rule.rule_set)
    && rule.rule_set.includes("GeoLocation-!CN"));
  const insertion = remoteRuleIndex >= 0 ? remoteRuleIndex : Math.min(3, rules.length);
  return insertDnsRuleText(source, insertion, JSON.stringify(createDefaultFakeIpDnsRule(), null, 2));
}

export function dnsServers(dns: unknown): unknown[] {
  return isDnsObject(dns) && Array.isArray(dns.servers) ? dns.servers : [];
}

export function dnsRules(dns: unknown): unknown[] {
  return isDnsObject(dns) && Array.isArray(dns.rules) ? dns.rules : [];
}

export function dnsServerTags(dns: unknown): string[] {
  return [...new Set(dnsServers(dns).flatMap((server) => isDnsObject(server) && typeof server.tag === "string" && server.tag.trim() ? [server.tag.trim()] : []))];
}

export function dnsServerType(server: unknown): string {
  return isDnsObject(server) && typeof server.type === "string" && server.type ? server.type : "legacy";
}

function portText(server: DnsServer): string {
  return typeof server.server_port === "number" || typeof server.server_port === "string" ? String(server.server_port) : "";
}

function endpointHost(server: DnsServer): string {
  const value = typeof server.server === "string" ? server.server : "";
  return value.includes(":") && !value.startsWith("[") ? `[${value}]` : value;
}

export function dnsServerAddress(server: unknown): string {
  if (!isDnsObject(server)) return "无效的服务器配置";
  const type = dnsServerType(server);
  if (type === "fakeip") {
    const ranges = [server.inet4_range, server.inet6_range].flatMap((value) => typeof value === "string" && value ? [value] : []);
    return `fake-ip://${ranges.join(",") || "未设置地址段"}`;
  }
  if (type === "local") return `local://${typeof server.predefined === "string" ? server.predefined : "system"}`;
  if (type === "hosts") return "hosts://配置内映射";
  if (type === "dhcp") return `dhcp://${typeof server.interface === "string" && server.interface ? server.interface : "auto"}`;
  if (type === "resolved") return "resolved://system";
  if (type === "tailscale") return `tailscale://${typeof server.endpoint === "string" && server.endpoint ? server.endpoint : "default"}`;
  const host = endpointHost(server) || "未设置地址";
  const port = portText(server);
  const path = (type === "https" || type === "h3") && typeof server.path === "string" ? server.path : "";
  return `${type}://${host}${port ? `:${port}` : ""}${path}`;
}

export function dnsServerDetour(server: unknown): string {
  return isDnsObject(server) && typeof server.detour === "string" ? server.detour : "";
}

export function dnsRuleType(rule: unknown): DnsRuleType {
  if (!isDnsObject(rule)) return "inline";
  return dnsRuleTypeOptions.find((option) => option.value !== "inline" && Object.prototype.hasOwnProperty.call(rule, option.value))?.value ?? "inline";
}

export function dnsRulePayloadText(rule: unknown, type = dnsRuleType(rule)): string {
  if (!isDnsObject(rule)) return "";
  if (type === "inline") return "完整 JSON 规则";
  const value = rule[type];
  if (Array.isArray(value)) return value.map((item) => typeof item === "object" && item !== null ? JSON.stringify(item) : String(item)).join(", ");
  if (typeof value === "object" && value !== null) return JSON.stringify(value);
  return String(value ?? "");
}

export function dnsRulePayloadValue(type: DnsRuleType, text: string, selectedTags: string[], original: unknown): unknown {
  if (type === "rule_set") return [...new Set(selectedTags)];
  if (type === "source_port" || type === "port") {
    const values = text.split(",").map((item) => Number(item.trim())).filter((item) => Number.isInteger(item));
    return Array.isArray(original) || values.length > 1 ? values : values[0];
  }
  if (type === "inbound" || type === "clash_mode") {
    const values = text.split(",").map((item) => item.trim()).filter(Boolean);
    return Array.isArray(original) ? values : values[0];
  }
  if (listRuleTypes.has(type)) {
    const values = text.split(",").map((item) => item.trim()).filter(Boolean);
    return Array.isArray(original) || original === undefined || values.length > 1 ? values : values[0];
  }
  return text.trim();
}

export function dnsRuleAction(rule: unknown): string {
  return isDnsObject(rule) && typeof rule.action === "string" && rule.action ? rule.action : "route";
}

export function dnsRuleServer(rule: unknown): string {
  return isDnsObject(rule) && typeof rule.server === "string" ? rule.server : "";
}

function sourceText(text: string): string {
  return text.trim() ? text : "{}";
}

function arrayNode(text: string, key: "servers" | "rules"): Node | undefined {
  const tree = parseTree(text, [], { allowTrailingComma: false, disallowComments: true });
  const node = tree ? findNodeAtLocation(tree, [key]) : undefined;
  return node?.type === "array" ? node : undefined;
}

function lineIndent(text: string, offset: number): string {
  const lineStart = text.lastIndexOf("\n", Math.max(0, offset - 1)) + 1;
  return text.slice(lineStart, offset).match(/^\s*/)?.[0] ?? "";
}

function indentItemText(raw: string, indent: string): string {
  const lines = raw.trim().split(/\r?\n/);
  if (lines.length === 1) return indent + lines[0];
  const tailIndents = lines.slice(1).filter((line) => line.trim()).map((line) => line.match(/^\s*/)?.[0].length ?? 0);
  const trim = tailIndents.length ? Math.min(...tailIndents) : 0;
  return lines.map((line, index) => indent + (index ? line.slice(Math.min(trim, line.length)) : line)).join("\n");
}

function rawElements(text: string, node: Node): string[] {
  return (node.children ?? []).map((child) => text.slice(child.offset, child.offset + child.length));
}

function replaceArrayElements(text: string, key: "servers" | "rules", elements: string[]): string {
  let source = sourceText(text);
  let node = arrayNode(source, key);
  if (!node) {
    const tree = parseTree(source, [], { allowTrailingComma: false, disallowComments: true });
    const existing = tree ? findNodeAtLocation(tree, [key]) : undefined;
    if (existing && existing.type !== "array") throw new Error(`dns.${key} 必须是数组`);
    source = setJsonPath(source, [key], []);
    node = arrayNode(source, key);
  }
  if (!node) throw new Error(`dns.${key} 必须是数组`);
  const indent = lineIndent(source, node.parent?.offset ?? node.offset);
  const childIndent = `${indent}  `;
  const replacement = elements.length
    ? `[\n${elements.map((item) => indentItemText(item, childIndent)).join(",\n")}\n${indent}]`
    : "[]";
  return source.slice(0, node.offset) + replacement + source.slice(node.offset + node.length);
}

export function dnsArrayItemText(text: string, key: "servers" | "rules", index: number): string | null {
  const source = sourceText(text);
  const child = arrayNode(source, key)?.children?.[index];
  return child ? source.slice(child.offset, child.offset + child.length) : null;
}

export function upsertDnsArrayItemText(text: string, key: "servers" | "rules", index: number, itemText: string): string {
  if (!parseDnsObjectText(itemText)) throw new Error(`DNS ${key === "servers" ? "服务器" : "规则"}必须是 JSON 对象`);
  const source = sourceText(text);
  const node = arrayNode(source, key);
  const elements = node ? rawElements(source, node) : [];
  if (index < 0 || index > elements.length) throw new Error(`DNS ${key} 索引超出范围`);
  if (index === elements.length) elements.push(itemText);
  else elements[index] = itemText;
  return replaceArrayElements(source, key, elements);
}

export function insertDnsRuleText(text: string, index: number, ruleText: string): string {
  if (!parseDnsObjectText(ruleText)) throw new Error("DNS 规则必须是 JSON 对象");
  const source = sourceText(text);
  const node = arrayNode(source, "rules");
  const elements = node ? rawElements(source, node) : [];
  if (index < 0 || index > elements.length) throw new Error("DNS 规则索引超出范围");
  elements.splice(index, 0, ruleText);
  return replaceArrayElements(source, "rules", elements);
}

export function removeDnsArrayItem(text: string, key: "servers" | "rules", index: number): string {
  return setJsonPath(sourceText(text), [key, index], undefined);
}

export function updateDnsBoolean(text: string, key: "disable_cache" | "disable_expire" | "independent_cache", value: boolean): string {
  return setJsonPath(sourceText(text), [key], value);
}

export function updateDnsString(text: string, key: "final" | "strategy" | "client_subnet", value: string): string {
  return setJsonPath(sourceText(text), [key], value.trim() || undefined);
}

export function renameDnsServerInRuleText(text: string, from: string, to: string): string {
  const rule = parseDnsObjectText(text);
  return rule?.server === from ? setJsonPath(text, ["server"], to) : text;
}

export function renameDnsServerTagInBuffer(source: string, module: string, from: string, to: string): string {
  if (!from || !to || from === to || (module !== "dns" && module !== "route")) return source;
  const value = parseDnsObjectText(source);
  if (!value) return source;
  let text = source;
  if (module === "dns") {
    if (value.final === from) text = setJsonPath(text, ["final"], to);
    dnsServers(value).forEach((server, index) => {
      if (!isDnsObject(server)) return;
      if (server.domain_resolver === from) text = setJsonPath(text, ["servers", index, "domain_resolver"], to);
      else if (isDnsObject(server.domain_resolver) && server.domain_resolver.server === from) text = setJsonPath(text, ["servers", index, "domain_resolver", "server"], to);
    });
    dnsRules(value).forEach((rule, index) => {
      if (isDnsObject(rule) && rule.server === from) text = setJsonPath(text, ["rules", index, "server"], to);
    });
    return text;
  }
  if (value.default_domain_resolver === from) text = setJsonPath(text, ["default_domain_resolver"], to);
  else if (isDnsObject(value.default_domain_resolver) && value.default_domain_resolver.server === from) text = setJsonPath(text, ["default_domain_resolver", "server"], to);
  if (Array.isArray(value.rules)) {
    value.rules.forEach((rule, index) => {
      if (isDnsObject(rule) && rule.server === from) text = setJsonPath(text, ["rules", index, "server"], to);
    });
  }
  return text;
}

export function createDnsStateId(): string {
  const bytes = new Uint8Array(8);
  globalThis.crypto?.getRandomValues?.(bytes);
  const random = [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
  return random && !/^0+$/.test(random) ? `dns-rule-${random}` : `dns-rule-local-${++fallbackStateId}`;
}

export function restoreDnsEditorState(value: unknown, activeRuleCount: number): DnsEditorState {
  const source = isDnsObject(value) && value.version === 1 ? value : {};
  const seenIds = new Set<string>();
  const candidates = Array.isArray(source.disabledRules) ? source.disabledRules.flatMap((item): DisabledDnsRuleState[] => {
    if (!isDnsObject(item) || typeof item.id !== "string" || !item.id || !Number.isInteger(item.position) || typeof item.text !== "string" || !parseDnsObjectText(item.text)) return [];
    if (seenIds.has(item.id)) return [];
    seenIds.add(item.id);
    return [{ id: item.id, position: Number(item.position), text: item.text }];
  }) : [];
  const slots: Array<{ disabled?: DisabledDnsRuleState }> = Array.from({ length: activeRuleCount }, () => ({}));
  for (const item of candidates.sort((left, right) => left.position - right.position)) {
    slots.splice(Math.max(0, Math.min(item.position, slots.length)), 0, { disabled: item });
  }
  const disabledRules = slots.flatMap((slot, position) => slot.disabled ? [{ ...slot.disabled, position }] : []);
  const total = activeRuleCount + disabledRules.length;
  const fallbackInsertion = Math.min(2, total);
  const requestedInsertion = Number.isInteger(source.insertionPosition) ? Number(source.insertionPosition) : fallbackInsertion;
  return {
    version: 1,
    insertionPosition: Math.max(0, Math.min(requestedInsertion, total)),
    disabledRules
  };
}

export function reconcileDnsEditorState(value: unknown, dns: unknown): DnsEditorState {
  const activeRules = dnsRules(dns);
  const restored = restoreDnsEditorState(value, activeRules.length);
  if (!activeRules.some(isDefaultFakeIpDnsRule)) return restored;
  const removedPositions = restored.disabledRules
    .filter((item) => isDefaultFakeIpDnsRule(parseDnsObjectText(item.text)))
    .map((item) => item.position);
  if (!removedPositions.length) return restored;
  const shift = (position: number) => removedPositions.filter((removed) => removed < position).length;
  const disabledRules = restored.disabledRules
    .filter((item) => !removedPositions.includes(item.position))
    .map((item) => ({ ...item, position: item.position - shift(item.position) }));
  return {
    version: 1,
    insertionPosition: Math.max(0, restored.insertionPosition - shift(restored.insertionPosition)),
    disabledRules
  };
}
