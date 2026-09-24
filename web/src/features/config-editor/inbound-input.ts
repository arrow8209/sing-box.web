import { parseTree, type Node } from "jsonc-parser";
import type { Draft } from "@/features/profiles/types";
import { isInboundConfig } from "@/features/inbounds/model";
import type { InboundConfig } from "@/features/inbounds/types";

// Preserve untouched independent objects as raw tokens while changing the
// surrounding array. In particular, their large integers never visit number.
export function serializeInboundInput(draft: Draft, items: InboundConfig[]): string {
  const source = draft.buffers.inbounds ?? "[]";
  const tree = parseTree(source);
  const original = new Map<string, { raw: string; parsed: unknown; unsafe: boolean }>();
  const hasUnsafeInteger = (raw: string): boolean => {
    const visit = (node: Node): boolean => {
      if (node.type === "number") {
        const token = raw.slice(node.offset, node.offset + node.length);
        return /^-?\d+$/.test(token) && (BigInt(token) > BigInt(Number.MAX_SAFE_INTEGER) || BigInt(token) < BigInt(Number.MIN_SAFE_INTEGER));
      }
      return node.children?.some(visit) ?? false;
    };
    const tree = parseTree(raw);
    return tree ? visit(tree) : false;
  };
  const hasUnsafeNumberValue = (value: unknown): boolean => {
    if (typeof value === "number") return Number.isInteger(value) && !Number.isSafeInteger(value);
    if (Array.isArray(value)) return value.some(hasUnsafeNumberValue);
    if (value && typeof value === "object") return Object.values(value).some(hasUnsafeNumberValue);
    return false;
  };
  if (tree?.type === "array") {
    for (const child of tree.children ?? []) {
      const raw = source.slice(child.offset, child.offset + child.length);
      try {
        const parsed: unknown = JSON.parse(raw);
        if (isInboundConfig(parsed)) original.set(parsed.tag, { raw, parsed, unsafe: hasUnsafeInteger(raw) });
      } catch { /* Invalid buffers are diagnosed by the normal JSON editor. */ }
    }
  }
  const entries = items.map((item) => {
    const saved = original.get(item.tag);
    if (saved && JSON.stringify(saved.parsed) === JSON.stringify(item)) return saved.raw;
    const similarUnsafe = [...original.values()].some((entry) => {
      if (!entry.unsafe || !isInboundConfig(entry.parsed)) return false;
      const { tag: _oldTag, ...oldFields } = entry.parsed;
      const { tag: _newTag, ...newFields } = item;
      return JSON.stringify(oldFields) === JSON.stringify(newFields);
    });
    if (saved?.unsafe || similarUnsafe || hasUnsafeNumberValue(item)) throw new Error("此入站包含超出 JavaScript 安全范围的整数，请使用完整 JSON 编辑器保留原始值。");
    return JSON.stringify(item);
  });
  return entries.length ? "[\n" + entries.join(",\n") + "\n]" : "[]";
}
