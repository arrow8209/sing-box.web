import type { Draft } from "@/features/profiles/types";
import { applyModuleBuffer, setJsonPath, type BufferError } from "./json";

export interface ModuleSaveResult { draft: Draft; error?: BufferError; }

export function saveModuleBuffer(draft: Draft, module: string, text: string): ModuleSaveResult {
  return applyModuleBuffer(draft, module, text);
}

export function updateStructuredPath(draft: Draft, module: string, path: readonly (string | number)[], value: unknown): ModuleSaveResult {
  const source = draft.buffers[module] ?? JSON.stringify(draft.config[module] ?? {}, null, 2);
  return applyModuleBuffer(draft, module, setJsonPath(source, path, value));
}
