import type { RuleBinding } from "@/features/profiles/types";
import type { RuleResource } from "@/features/rule-resources/types";
import { setJsonPath } from "./json";

export type RouteRule = Record<string, unknown>;

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
  resourceIds: string[];
}

export interface RoutePanelSave {
  text: string;
  bindings: RuleBinding[];
}

export function isRouteObject(value: unknown): value is RouteRule {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function cloneRouteValue<T>(value: T): T {
  if (value === null || typeof value !== "object") return value;
  if (Array.isArray(value)) return value.map((item) => cloneRouteValue(item)) as T;
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, cloneRouteValue(item)])) as T;
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

export function routeOutboundTags(config: Record<string, unknown>): string[] {
  if (!Array.isArray(config.outbounds)) return [];
  return [...new Set(config.outbounds.flatMap((item) => isRouteObject(item) && typeof item.tag === "string" && item.tag.trim() ? [item.tag.trim()] : []))];
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

export function upsertRouteRule(text: string, index: number, rule: RouteRule): string {
  return setJsonPath(text, ["rules", index], rule);
}

export function removeRouteRule(text: string, index: number): string {
  return setJsonPath(text, ["rules", index], undefined);
}

export function updateRouteFinal(text: string, tag: string): string {
  return setJsonPath(text, ["final"], tag || undefined);
}
