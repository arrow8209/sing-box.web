import { parseTree } from "jsonc-parser";
import type { Draft } from "@/features/profiles/types";
import { isInboundConfig } from "@/features/inbounds/model";
import type { InboundConfig } from "@/features/inbounds/types";

// Preserve untouched independent objects as raw tokens while changing the
// surrounding array. In particular, their large integers never visit number.
export function serializeInboundInput(draft: Draft, items: InboundConfig[]): string {
  const source = draft.buffers.inbounds ?? "[]";
  const tree = parseTree(source);
  const original = new Map<string, { raw: string; parsed: unknown }>();
  if (tree?.type === "array") {
    for (const child of tree.children ?? []) {
      const raw = source.slice(child.offset, child.offset + child.length);
      try {
        const parsed: unknown = JSON.parse(raw);
        if (isInboundConfig(parsed)) original.set(parsed.tag, { raw, parsed });
      } catch { /* Invalid buffers are diagnosed by the normal JSON editor. */ }
    }
  }
  const entries = items.map((item) => {
    const saved = original.get(item.tag);
    return saved && JSON.stringify(saved.parsed) === JSON.stringify(item) ? saved.raw : JSON.stringify(item);
  });
  return entries.length ? "[\n" + entries.join(",\n") + "\n]" : "[]";
}
