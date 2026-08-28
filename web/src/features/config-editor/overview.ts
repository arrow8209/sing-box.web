import type { Draft } from "@/features/profiles/types";
import { setJsonPath, validateModuleShape } from "./json";
import type { ConfigModule } from "./module-map";
import { saveModuleBuffer, type ModuleSaveResult } from "./paths";

export interface OverviewEdit {
  path: readonly (string | number)[];
  value: unknown;
}

export function isJsonObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function applyOverviewEdits(
  draft: Draft,
  module: ConfigModule,
  edits: readonly OverviewEdit[],
  emptyRoot: Record<string, never> | unknown[] = {}
): ModuleSaveResult {
  const configured = draft.config[module];
  let source = draft.buffers[module];
  if (!source?.trim()) source = JSON.stringify(configured ?? emptyRoot, null, 2);

  const expected = Array.isArray(emptyRoot) ? "array" : "object";
  const shapeError = validateModuleShape(source, module, expected);
  if (shapeError) return { draft, error: shapeError };

  try {
    for (const edit of edits) source = setJsonPath(source, edit.path, edit.value);
  } catch {
    return {
      draft,
      error: {
        code: "invalid_module_shape",
        message: `${module} 当前结构无法在概览中编辑，请改用完整 JSON`,
        fieldPath: module
      }
    };
  }
  return saveModuleBuffer(draft, module, source);
}

export function mixedInboundIndex(config: Record<string, unknown>): number {
  const inbounds = config.inbounds;
  return Array.isArray(inbounds)
    ? inbounds.findIndex((item) => isJsonObject(item) && item.type === "mixed")
    : -1;
}

export function externalControllerPort(controller: unknown): string {
  if (typeof controller !== "string") return "";
  return controller.match(/:(\d+)$/)?.[1] ?? "";
}

export function replaceExternalControllerPort(controller: unknown, port: number): string {
  const current = typeof controller === "string" ? controller.trim() : "";
  const host = current.match(/^(.*):\d+$/)?.[1] || "127.0.0.1";
  return `${host}:${port}`;
}
