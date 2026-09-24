import type { PersistedReferences } from "@/features/profiles/types";
import { isRouteObject, type RouteEditorState, type RouteRule } from "./route";

export interface ManagedInboundSniffRule {
  tag: string;
  rule: RouteRule;
}

export interface InboundResourceBinding {
  tag: string;
  resourceId: string;
  sourceTag: string;
  position?: number;
}

// Keep the exact generated rule outside sing-box JSON. A later disable or
// rename only removes this snapshot, never a similar user-authored rule.
export interface InboundEditorState {
  version: 1;
  managedSniffRules: ManagedInboundSniffRule[];
  resourceBindings?: InboundResourceBinding[];
}

export interface InboundPanelSave {
  text: string;
  editorState?: InboundEditorState;
  routeText?: string;
  routeEditorState?: RouteEditorState;
  references?: PersistedReferences;
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
  const linked = new Set<string>();
  const resourceBindings = (Array.isArray(value.resourceBindings) ? value.resourceBindings : []).flatMap((item): InboundResourceBinding[] => {
    if (!isRouteObject(item) || typeof item.tag !== "string" || !item.tag.trim()
      || typeof item.resourceId !== "string" || !/^[a-f0-9]{32}$/.test(item.resourceId)
      || typeof item.sourceTag !== "string" || linked.has(item.tag)) return [];
    linked.add(item.tag);
    return [{ tag: item.tag, resourceId: item.resourceId, sourceTag: item.sourceTag }];
  });
  return { version: 1, managedSniffRules, ...(resourceBindings.length ? { resourceBindings } : {}) };
}

