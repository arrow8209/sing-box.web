import type { Draft } from "@/features/profiles/types";
import { setJsonPath, validateModuleShape } from "./json";
import { saveModuleBuffer, type ModuleSaveResult } from "./paths";

export interface GeneralSettingEdit {
  path: readonly (string | number)[];
  value: unknown;
}

export function applyGeneralSettingEdits(
  draft: Draft,
  edits: readonly GeneralSettingEdit[]
): ModuleSaveResult {
  let source = draft.buffers.log;
  if (!source?.trim()) source = JSON.stringify(draft.config.log ?? {}, null, 2);

  const shapeError = validateModuleShape(source, "log", "object");
  if (shapeError) return { draft, error: shapeError };

  try {
    for (const edit of edits) source = setJsonPath(source, edit.path, edit.value);
  } catch {
    return {
      draft,
      error: {
        code: "invalid_module_shape",
        message: "log 当前结构无法在通用设置中编辑，请改用完整 JSON",
        fieldPath: "log"
      }
    };
  }
  return saveModuleBuffer(draft, "log", source);
}
