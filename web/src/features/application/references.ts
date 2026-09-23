import { parse, type ParseError } from "jsonc-parser";
import { inboundApi } from "@/features/inbounds/api";
import { isInboundConfig, withoutLegacyInboundFields } from "@/features/inbounds/model";
import type { InboundDetail } from "@/features/inbounds/types";
import { subscriptionApi } from "@/features/subscriptions/api";
import type { SubscriptionDetail } from "@/features/subscriptions/types";
import type { Draft } from "@/features/profiles/types";
import { applyInboundRouteIntents, restoreInboundEditorState, type InboundRouteIntent } from "@/features/config-editor/inbound";
import { applyModuleBuffer, setJsonPath } from "@/features/config-editor/json";
import {
  applyOutboundEditorState, cloneValue, isObject, isOutboundConfig, outboundEditorGroups,
  outboundReferenceTags, parseOutboundEditorState
} from "@/features/config-editor/outbound";

export interface ReferenceSources {
  subscription(id: string): Promise<SubscriptionDetail>;
  inbound(id: string): Promise<InboundDetail>;
}

const defaultSources: ReferenceSources = {
  subscription: (id) => subscriptionApi.get(id),
  inbound: (id) => inboundApi.get(id)
};

export class ReferenceSyncError extends Error {
  constructor(message: string, readonly fieldPath: string) { super(message); }
}

export function sameJsonValue(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left) && Array.isArray(right) && left.length === right.length
      && left.every((item, index) => sameJsonValue(item, right[index]));
  }
  if (!isObject(left) || !isObject(right)) return false;
  const keys = Object.keys(left);
  return keys.length === Object.keys(right).length
    && keys.every((key) => Object.prototype.hasOwnProperty.call(right, key) && sameJsonValue(left[key], right[key]));
}

function arrayModule(draft: Draft, module: "inbounds" | "outbounds"): { text: string; items: unknown[] } {
  const text = draft.buffers[module] ?? JSON.stringify(draft.config[module] ?? [], null, 2);
  if (!text.trim()) return { text, items: [] };
  const errors: ParseError[] = [];
  const items: unknown = parse(text, errors, { allowTrailingComma: false, disallowComments: true });
  if (errors.length || !Array.isArray(items)) throw new ReferenceSyncError(`${module} JSON 无效，请先修正后再同步引用。`, module);
  const tags = new Set<string>();
  for (const item of items) {
    if (!isObject(item) || typeof item.tag !== "string" || !item.tag) continue;
    if (tags.has(item.tag)) throw new ReferenceSyncError(`${module === "inbounds" ? "入站" : "出站"}名称重复：“${item.tag}”，请先修正后再同步引用。`, module);
    tags.add(item.tag);
  }
  return { text, items };
}

function replaceModule(draft: Draft, module: string, text: string): Draft {
  const result = applyModuleBuffer(draft, module, text);
  if (result.error) throw new ReferenceSyncError(result.error.message, module);
  return result.draft;
}

// Work on a snapshot. The caller persists it once all source reads succeed and
// confirms the user has not edited or switched the active draft in the meantime.
export async function synchronizeReferences(draft: Draft, sources: ReferenceSources = defaultSources): Promise<Draft> {
  let next = cloneValue(draft);
  const storedOutbounds = parseOutboundEditorState(next.editorState?.outbounds);
  if (storedOutbounds?.groups.some((group) => group.references.some((item) => item.kind === "subscription" || item.kind === "subscription-node"))) {
    const source = arrayModule(next, "outbounds");
    const state = storedOutbounds;
    const groups = outboundEditorGroups(source.items, state);
    for (const group of state.groups) {
      const config = source.items.find((item) => isOutboundConfig(item) && item.tag === group.tag);
      if (isOutboundConfig(config) && (config.type !== group.type || !sameJsonValue(outboundReferenceTags(config), group.runtimeTags))) {
        throw new ReferenceSyncError(`出站“${group.tag}”的 JSON 与引用信息不一致，请在出站编辑器中确认并保存引用后重试。`, "outbounds");
      }
    }
    const references = groups.flatMap((group) => group.references).filter((item) => item.kind === "subscription" || item.kind === "subscription-node");
    const subscriptions = await Promise.all([...new Set(references.map((item) => item.subscriptionId))].map(async (id) => {
      try { return await sources.subscription(id); }
      catch (cause) {
        const reference = references.find((item) => item.subscriptionId === id)!;
        const name = reference.kind === "subscription" ? reference.tag : reference.nodeTag;
        throw new ReferenceSyncError(`无法同步订阅引用“${name}”：${cause instanceof Error ? cause.message : "读取源资源失败"}`, "outbounds");
      }
    }));
    for (const reference of references) {
      if (reference.kind === "subscription-node" && !subscriptions.find((item) => item.id === reference.subscriptionId)?.outbounds.some((node) => node.tag === reference.nodeTag)) {
        throw new ReferenceSyncError(`订阅节点“${reference.nodeTag}”已不存在，请重新选择该引用。`, "outbounds");
      }
    }
    if (references.length) {
      const result = applyOutboundEditorState(source.text, source.items, state, groups, subscriptions);
      const prepared = replaceModule(next, "outbounds", result.text);
      if (!sameJsonValue(source.items, prepared.config.outbounds)) next = prepared;
      next.editorState = { ...next.editorState, outbounds: result.state };
    }
  }

  const inboundState = restoreInboundEditorState(next.editorState?.inbounds);
  if (inboundState.resourceBindings?.length) {
    const source = arrayModule(next, "inbounds");
    const bindings = inboundState.resourceBindings.filter((binding) => source.items.some((item) => isInboundConfig(item) && item.tag === binding.tag));
    const details = await Promise.all([...new Set(bindings.map((item) => item.resourceId))].map(async (id) => {
      try { return await sources.inbound(id); }
      catch (cause) {
        const binding = bindings.find((item) => item.resourceId === id)!;
        throw new ReferenceSyncError(`无法同步入站“${binding.tag}”关联的源资源：${cause instanceof Error ? cause.message : "读取失败"}。请重新关联或解除关联。`, "inbounds");
      }
    }));
    let text = source.text;
    const intents: InboundRouteIntent[] = [];
    for (const binding of bindings) {
      const detail = details.find((item) => item.id === binding.resourceId)!;
      const config = detail.mode === "structured" ? withoutLegacyInboundFields(cloneValue(detail.config)) : cloneValue(detail.config);
      config.tag = binding.tag;
      binding.sourceTag = detail.config.tag;
      const index = source.items.findIndex((item) => isInboundConfig(item) && item.tag === binding.tag);
      if (!sameJsonValue(source.items[index], config)) text = setJsonPath(text, [index], config);
      intents.push({ tag: binding.tag, sniff: detail.mode === "structured" && detail.config.sniff === true, sniffTimeout: typeof detail.config.sniff_timeout === "string" ? detail.config.sniff_timeout : undefined });
    }
    if (text !== source.text) next = replaceModule(next, "inbounds", text);
    const route = applyInboundRouteIntents(next, intents);
    if (route.routeText !== undefined) next = replaceModule(next, "route", route.routeText);
    const state = { ...route.editorState };
    if (bindings.length) state.resourceBindings = bindings;
    else delete state.resourceBindings;
    next.editorState = { ...next.editorState, inbounds: state, ...(route.routeEditorState ? { route: route.routeEditorState } : {}) };
  }
  return next;
}
