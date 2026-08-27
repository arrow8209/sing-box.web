import { parse, type ParseError } from "jsonc-parser";
import type { Draft } from "@/features/profiles/types";
import {
  insertRouteRuleText,
  isRouteObject,
  removeRouteRule,
  restoreRouteEditorState,
  routeRules,
  upsertRouteRuleText,
  type RouteEditorState,
  type RouteRule
} from "./route";

export interface ManagedInboundSniffRule {
  tag: string;
  rule: RouteRule;
}

// Keep the exact generated rule outside sing-box JSON. A later disable or
// rename only removes this snapshot, never a similar user-authored rule.
export interface InboundEditorState {
  version: 1;
  managedSniffRules: ManagedInboundSniffRule[];
}

export interface InboundRouteIntent {
  previousTag?: string;
  tag: string;
  sniff: boolean;
  sniffTimeout?: string;
}

export interface InboundRouteUpdate {
  routeText?: string;
  editorState: InboundEditorState;
  routeEditorState?: RouteEditorState;
}

export interface InboundPanelSave extends InboundRouteUpdate {
  text: string;
}

function deepEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left) && Array.isArray(right)
      && left.length === right.length
      && left.every((item, index) => deepEqual(item, right[index]));
  }
  if (!isRouteObject(left) || !isRouteObject(right)) return false;
  const leftKeys = Object.keys(left).sort();
  const rightKeys = Object.keys(right).sort();
  return leftKeys.length === rightKeys.length
    && leftKeys.every((key, index) => key === rightKeys[index] && deepEqual(left[key], right[key]));
}

function isManagedRule(value: unknown, tag: string): value is RouteRule {
  return isRouteObject(value) && value.action === "sniff" && value.inbound === tag;
}

export function restoreInboundEditorState(value: unknown): InboundEditorState {
  if (!isRouteObject(value) || value.version !== 1 || !Array.isArray(value.managedSniffRules)) {
    return { version: 1, managedSniffRules: [] };
  }
  const seen = new Set<string>();
  const managedSniffRules = value.managedSniffRules.flatMap((item): ManagedInboundSniffRule[] => {
    if (!isRouteObject(item) || typeof item.tag !== "string") return [];
    const tag = item.tag.trim();
    if (!tag || seen.has(tag) || !isManagedRule(item.rule, tag)) return [];
    seen.add(tag);
    return [{ tag, rule: { ...item.rule } }];
  });
  return { version: 1, managedSniffRules };
}

function reconcileManagedRules(state: InboundEditorState, rules: unknown[]): InboundEditorState {
  return {
    version: 1,
    managedSniffRules: state.managedSniffRules.filter((managed) => rules.some((rule) => deepEqual(rule, managed.rule)))
  };
}

function routeSource(draft: Draft): string {
  const buffer = draft.buffers.route;
  if (buffer?.trim()) return buffer;
  return JSON.stringify(isRouteObject(draft.config.route) ? draft.config.route : {}, null, 2);
}

function parseRouteSource(text: string): RouteRule {
  const errors: ParseError[] = [];
  const parsed = parse(text, errors, { allowTrailingComma: false, disallowComments: true });
  if (errors.length || !isRouteObject(parsed)) throw new Error("route JSON 无效，无法写入结构化协议嗅探规则。");
  if (parsed.rules !== undefined && !Array.isArray(parsed.rules)) throw new Error("route.rules 必须是数组，无法写入结构化协议嗅探规则。");
  return parsed;
}

function desiredSniffRule(intent: InboundRouteIntent): RouteRule {
  return {
    inbound: intent.tag.trim(),
    action: "sniff",
    timeout: intent.sniffTimeout?.trim() || "300ms"
  };
}

function sniffAppliesToTag(rule: unknown, tag: string): boolean {
  if (!isRouteObject(rule) || rule.action !== "sniff") return false;
  if (rule.inbound === undefined) return true;
  if (rule.inbound === tag) return true;
  return Array.isArray(rule.inbound) && rule.inbound.includes(tag);
}

function combinedPositionForActiveIndex(state: RouteEditorState, activeRuleCount: number, activeIndex: number): number {
  const disabled = new Set(state.disabledRules.map((item) => item.position));
  const total = activeRuleCount + state.disabledRules.length;
  let seenActive = 0;
  for (let position = 0; position < total; position++) {
    if (disabled.has(position)) continue;
    if (seenActive === activeIndex) return position;
    seenActive++;
  }
  return total;
}

function insertActiveRuleState(value: unknown, activeRuleCount: number, activeIndex: number): RouteEditorState {
  const state = restoreRouteEditorState(value, activeRuleCount);
  const position = combinedPositionForActiveIndex(state, activeRuleCount, activeIndex);
  return restoreRouteEditorState({
    version: 1,
    insertionPosition: state.insertionPosition + (position <= state.insertionPosition ? 1 : 0),
    disabledRules: state.disabledRules.map((item) => ({
      ...item,
      position: item.position >= position ? item.position + 1 : item.position
    }))
  }, activeRuleCount + 1);
}

function removeActiveRuleState(value: unknown, activeRuleCount: number, activeIndex: number): RouteEditorState {
  const state = restoreRouteEditorState(value, activeRuleCount);
  const position = combinedPositionForActiveIndex(state, activeRuleCount, activeIndex);
  return restoreRouteEditorState({
    version: 1,
    insertionPosition: state.insertionPosition - (position < state.insertionPosition ? 1 : 0),
    disabledRules: state.disabledRules.map((item) => ({
      ...item,
      position: item.position > position ? item.position - 1 : item.position
    }))
  }, Math.max(0, activeRuleCount - 1));
}

export function managedInboundSniff(draft: Draft, tag: string): { enabled: boolean; timeout: string } {
  const rules = routeRules(draft.config.route);
  const state = reconcileManagedRules(restoreInboundEditorState(draft.editorState?.inbounds), rules);
  const managed = state.managedSniffRules.find((item) => item.tag === tag && rules.some((rule) => deepEqual(rule, item.rule)));
  return {
    enabled: Boolean(managed),
    timeout: managed && typeof managed.rule.timeout === "string" && managed.rule.timeout.trim() ? managed.rule.timeout : "300ms"
  };
}

export function applyInboundRouteIntents(draft: Draft, intents: InboundRouteIntent[]): InboundRouteUpdate {
  const restoredState = restoreInboundEditorState(draft.editorState?.inbounds);
  const needsRouteUpdate = intents.some((intent) => {
    const previousTag = intent.previousTag?.trim() || intent.tag.trim();
    return intent.sniff || restoredState.managedSniffRules.some((item) => item.tag === previousTag);
  });
  if (!needsRouteUpdate) return { editorState: restoredState };

  const originalText = routeSource(draft);
  let text = originalText;
  let parsed = parseRouteSource(text);
  let rules = routeRules(parsed);
  let state = reconcileManagedRules(restoredState, rules);
  let routeEditorState: RouteEditorState | undefined;
  let routeStateValue: unknown = draft.editorState?.route;

  for (const intent of intents) {
    const tag = intent.tag.trim();
    const previousTag = intent.previousTag?.trim() || tag;
    if (!tag) continue;
    const managed = state.managedSniffRules.find((item) => item.tag === previousTag);
    const managedRuleIndex = managed ? rules.findIndex((rule) => deepEqual(rule, managed.rule)) : -1;
    const desired = intent.sniff ? desiredSniffRule({ ...intent, tag }) : null;
    const conflictingRule = desired && rules.some((rule, index) => index !== managedRuleIndex && sniffAppliesToTag(rule, tag));

    if (managed && managedRuleIndex >= 0) {
      state = { ...state, managedSniffRules: state.managedSniffRules.filter((item) => item !== managed) };
      if (desired && !conflictingRule) {
        if (!deepEqual(managed.rule, desired)) text = upsertRouteRuleText(text, managedRuleIndex, JSON.stringify(desired, null, 2));
        state.managedSniffRules.push({ tag, rule: desired });
        rules[managedRuleIndex] = desired;
      } else {
        const nextRouteState = removeActiveRuleState(routeStateValue, rules.length, managedRuleIndex);
        text = removeRouteRule(text, managedRuleIndex);
        rules.splice(managedRuleIndex, 1);
        routeEditorState = nextRouteState;
        routeStateValue = nextRouteState;
      }
      continue;
    }

    if (!desired || conflictingRule) continue;
    // Sniff is a non-final action and must run before terminal route rules.
    const insertionIndex = 0;
    const nextRouteState = insertActiveRuleState(routeStateValue, rules.length, insertionIndex);
    text = insertRouteRuleText(text, insertionIndex, JSON.stringify(desired, null, 2));
    rules.splice(insertionIndex, 0, desired);
    state.managedSniffRules.push({ tag, rule: desired });
    routeEditorState = nextRouteState;
    routeStateValue = nextRouteState;
  }

  parsed = parseRouteSource(text);
  state = reconcileManagedRules(state, routeRules(parsed));
  return {
    routeText: text === originalText ? undefined : text,
    editorState: state,
    ...(routeEditorState ? { routeEditorState } : {})
  };
}
