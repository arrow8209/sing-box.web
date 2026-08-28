import type { Draft } from "@/features/profiles/types";
import { setJsonPath, validateModuleShape } from "./json";
import { saveModuleBuffer, type ModuleSaveResult } from "./paths";

export interface GeneralSettingEdit {
  path: readonly (string | number)[];
  value: unknown;
}

export type GeneralSettingsModule = "log" | "experimental";

export interface GeneralSettingsSave {
  draft: Draft;
  module: GeneralSettingsModule;
  error?: string;
}

export function applyGeneralSettingEdits(
  draft: Draft,
  module: GeneralSettingsModule,
  edits: readonly GeneralSettingEdit[]
): ModuleSaveResult {
  let source = draft.buffers[module];
  if (!source?.trim()) source = JSON.stringify(draft.config[module] ?? {}, null, 2);

  const shapeError = validateModuleShape(source, module, "object");
  if (shapeError) return { draft, error: shapeError };

  try {
    for (const edit of edits) source = setJsonPath(source, edit.path, edit.value);
  } catch {
    return {
      draft,
      error: {
        code: "invalid_module_shape",
        message: `${module} 当前结构无法在常规设置中编辑，请改用完整 JSON`,
        fieldPath: module
      }
    };
  }
  return saveModuleBuffer(draft, module, source);
}
