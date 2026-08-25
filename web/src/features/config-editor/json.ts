import { applyEdits, findNodeAtLocation, modify, parse, parseTree, type ParseError } from "jsonc-parser";
import type { Draft } from "@/features/profiles/types";

export interface BufferError { code: "invalid_json_buffer" | "invalid_module_shape"; message: string; fieldPath: string; }
export interface BufferResult { draft: Draft; error?: BufferError; }

export function setJsonPath(text: string, path: readonly (string | number)[], value: unknown): string {
  const edits = modify(text, [...path], value, { formattingOptions: { insertSpaces: true, tabSize: 2, eol: "\n" } });
  const output = applyEdits(text, edits);
  // jsonc-parser replaces an existing value token without changing surrounding
  // whitespace. Normalize only the edited property's colon, not the document.
  const tree = parseTree(output);
  if (!tree) return output;
  const node = findNodeAtLocation(tree, [...path]);
  const property = node?.parent;
  if (!node || !property || property.type !== "property") return output;
  const prefix = output.slice(property.offset, node.offset);
  const normalized = prefix.replace(/(\"(?:[^\"\\]|\\.)*\")\s*:\s*$/, "$1: ");
  return normalized === prefix ? output : output.slice(0, property.offset) + normalized + output.slice(node.offset);
}

export function applyModuleBuffer(draft: Draft, module: string, text: string): BufferResult {
  // Drafts held by the profile store are deep Vue proxies, which the browser
  // structured-clone algorithm rejects. This edit only replaces one module,
  // so copy the envelope and its maps without round-tripping untouched JSON.
  const next: Draft = {
    ...draft,
    config: { ...draft.config },
    buffers: { ...draft.buffers },
    bindings: draft.bindings.map((binding) => ({ ...binding }))
  };
  next.buffers[module] = text;
  if (text.trim() === "") {
    delete next.config[module];
    return { draft: next };
  }
  const errors: ParseError[] = [];
  const value = parse(text, errors, { allowTrailingComma: false, disallowComments: true });
  if (errors.length !== 0) return { draft: next, error: { code: "invalid_json_buffer", message: `JSON 语法错误（offset ${errors[0]?.offset ?? 0}）`, fieldPath: module } };
  next.config[module] = value;
  return { draft: next };
}

export function validateModuleShape(text: string, module: string, expected: "array" | "object"): BufferError | undefined {
  if (text.trim() === "") return undefined;
  const tree = parseTree(text, [], { allowTrailingComma: false, disallowComments: true });
  if (!tree) return { code: "invalid_json_buffer", message: "JSON 语法错误", fieldPath: module };
  if (tree.type !== expected) return { code: "invalid_module_shape", message: `${module} 必须是${expected === "array" ? "数组" : "对象"}`, fieldPath: module };
  return undefined;
}
