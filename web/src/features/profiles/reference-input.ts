import type { Draft, PersistedReference, PersistedReferences } from "./types";

interface LegacyGroup {
  id: string; tag: string; type: string; include?: string; exclude?: string;
  references?: Array<PersistedReference & { resolvedTags?: string[] }>;
}
interface LegacyOutboundState { version?: number; groups?: LegacyGroup[]; }
interface LegacyInboundState { version?: number; resourceBindings?: Array<{ tag: string; resourceId: string; sourceTag: string }> }

// Adapt only explicit saved IDs. This never matches a source by name or JSON.
export function persistedReferences(draft: Draft): PersistedReferences {
  if (draft.schemaVersion === 2 && draft.references) return JSON.parse(JSON.stringify(draft.references)) as PersistedReferences;
  const outbounds = draft.editorState?.outbounds as LegacyOutboundState | undefined;
  const inbounds = draft.editorState?.inbounds as LegacyInboundState | undefined;
  const outboundItems = Array.isArray(draft.config.outbounds) ? draft.config.outbounds : [];
  const inboundItems = Array.isArray(draft.config.inbounds) ? draft.config.inbounds : [];
  return {
    version: 2,
    outbounds: outbounds?.version === 1 ? (outbounds.groups ?? []).filter((group) => outboundItems.some((item) => Boolean(item && typeof item === "object" && !Array.isArray(item) && (item as Record<string, unknown>).tag === group.tag))).map((group) => ({
      id: group.id, tag: group.tag, type: group.type, include: group.include ?? "", exclude: group.exclude ?? "",
      references: (group.references ?? []).map(({ resolvedTags: _resolvedTags, ...reference }) => ({ ...reference }))
    })) : [],
    inbounds: inbounds?.version === 1 ? (inbounds.resourceBindings ?? []).flatMap((binding) => {
      const position = inboundItems.findIndex((item) => Boolean(item && typeof item === "object" && !Array.isArray(item) && (item as Record<string, unknown>).tag === binding.tag));
      return position < 0 ? [] : [{ ...binding, position }];
    }) : []
  };
}

export function presentationEditorState(draft: Draft): Record<string, unknown> {
  const state = { ...(draft.editorState ?? {}) };
  delete state.inbounds;
  const outbound = state.outbounds as { version?: number; groups?: Array<{ id: string; tag: string; hidden?: boolean; icon?: string }> } | undefined;
  if (outbound?.version === 1) state.outbounds = { version: 2, groups: (outbound.groups ?? []).map((group) => ({
    id: group.id, tag: group.tag, hidden: group.hidden ?? false, icon: group.icon ?? ""
  })) };
  return state;
}
