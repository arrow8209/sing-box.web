import { ref, type Ref } from "vue";
import { isApiError } from "@/features/profiles/api";
import { useProfileStore, type ProfileStore } from "@/features/profiles/store";
import { applicationApi, type ApplicationApi } from "./api";
import { ReferenceSyncError, sameJsonValue, synchronizeReferences, type ReferenceSources } from "./references";
import { cloneValue } from "@/features/config-editor/outbound";
import type { ApplicationError, ApplyResult, PreviewResult, ValidationResult } from "./types";

export interface ApplicationStore {
  previewResult: Ref<PreviewResult | null>; validation: Ref<ValidationResult | null>; error: Ref<ApplicationError | null>;
  applying: Ref<boolean>;
  preparing: Ref<boolean>;
  preview(): Promise<void>; validate(): Promise<void>; apply(): Promise<ApplyResult | null>; clear(): void; dispose(): void;
}

function errorFrom(cause: unknown): ApplicationError {
  if (isApiError(cause)) {
    const output = [cause.body.details?.stdout, cause.body.details?.stderr].filter((value): value is string => typeof value === "string" && value.length > 0).join("\n");
    return { code: cause.body.code, message: cause.body.message, stage: cause.body.stage, fieldPath: cause.body.fieldPath, output };
  }
  return { code: "network_error", message: cause instanceof Error ? cause.message : "请求失败" };
}

export function createApplicationStore(profiles: ProfileStore = useProfileStore(), api: ApplicationApi = applicationApi, sources?: ReferenceSources): ApplicationStore {
  const previewResult = ref<PreviewResult | null>(null);
  const validation = ref<ValidationResult | null>(null);
  const error = ref<ApplicationError | null>(null);
  const applying = ref(false);
  const preparing = ref(false);
  let preparation: Promise<{ id: string; revision: number } | null> | null = null;
  const active = () => ({ id: profiles.activeProfileId.value, revision: profiles.draft.value?.revision });
  async function prepare(): Promise<{ id: string; revision: number } | null> {
    if (preparation) return preparation;
    preparing.value = true;
    preparation = (async () => {
      try {
        await profiles.flush();
        const value = active();
        if (!value.id || !value.revision || !profiles.draft.value || profiles.saveState.value !== "saved") return null;
        const snapshot = cloneValue(profiles.draft.value);
        const synchronized = await synchronizeReferences(snapshot, sources);
        if (profiles.activeProfileId.value !== value.id || profiles.saveState.value !== "saved"
          || !sameJsonValue(profiles.draft.value, snapshot)) {
          throw new ReferenceSyncError("同步期间草稿已变更或切换，请等待保存完成后重试。", "");
        }
        if (!sameJsonValue(synchronized.buffers, snapshot.buffers) || !sameJsonValue(synchronized.editorState, snapshot.editorState)) {
          profiles.mutate((current) => {
            current.buffers = synchronized.buffers;
            current.config = synchronized.config;
            current.editorState = synchronized.editorState;
          });
          await profiles.flush();
          if (profiles.saveState.value !== "saved") throw new ReferenceSyncError("同步后的草稿尚未保存成功，请先处理保存错误或版本冲突。", "");
        }
        const prepared = active();
        if (prepared.id !== value.id || !prepared.revision) return null;
        return { id: prepared.id, revision: prepared.revision };
      } catch (cause) {
        error.value = { ...errorFrom(cause), stage: "sync_references", ...(cause instanceof ReferenceSyncError ? { code: "reference_sync_failed", fieldPath: cause.fieldPath || undefined } : {}) };
        return null;
      }
    })().finally(() => { preparation = null; preparing.value = false; });
    return preparation;
  }
  async function preview(): Promise<void> { error.value = null; previewResult.value = null; const value = await prepare(); if (!value) return; try { previewResult.value = await api.preview(value.id, value.revision); } catch (cause) { error.value = errorFrom(cause); } }
  async function validate(): Promise<void> { error.value = null; validation.value = null; const value = await prepare(); if (!value) return; try { validation.value = await api.validate(value.id, value.revision); await profiles.refresh(); } catch (cause) { error.value = errorFrom(cause); } }
  async function apply(): Promise<ApplyResult | null> {
    error.value = null; validation.value = null;
    const value = await prepare();
    if (!value) return null;
    applying.value = true;
    try {
      const result = await api.apply(value.id, { revision: value.revision });
      validation.value = result.validation;
      await profiles.refresh();
      return result;
    } catch (cause) {
      error.value = errorFrom(cause);
      return null;
    } finally {
      applying.value = false;
    }
  }
  return {
    previewResult, validation, error, applying, preparing, preview, validate, apply,
    clear: () => { error.value = null; validation.value = null; previewResult.value = null; },
    dispose: () => undefined
  };
}

const defaultStore = createApplicationStore();
export function useApplicationStore(): ApplicationStore { return defaultStore; }
