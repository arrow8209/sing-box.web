import { computed, ref, type Ref } from "vue";
import { isApiError } from "@/features/profiles/api";
import { useProfileStore, type ProfileStore } from "@/features/profiles/store";
import { applicationApi, type ApplicationApi } from "./api";
import { ReferenceSyncError, sameJsonValue, synchronizeReferences, type ReferenceSources } from "./references";
import { cloneValue } from "@/features/config-editor/outbound";
import { useCapabilitiesStore } from "@/features/capabilities/store";
import type { ApplicationError, ApplyResult, PreviewResult, ValidationResult } from "./types";

export interface ApplicationStore {
  previewResult: Ref<PreviewResult | null>; validation: Ref<ValidationResult | null>; error: Ref<ApplicationError | null>;
  applying: Ref<boolean>;
  preparing: Ref<boolean>;
  requesting: Ref<boolean>;
  preview(): Promise<void>; fetchCandidate(): Promise<void>; validate(): Promise<void>; apply(): Promise<ApplyResult | null>; clear(): void; dispose(): void;
}

function errorFrom(cause: unknown): ApplicationError {
  if (isApiError(cause)) {
    const output = [cause.body.details?.stdout, cause.body.details?.stderr].filter((value): value is string => typeof value === "string" && value.length > 0).join("\n");
    return { code: cause.body.code, message: cause.body.message, stage: cause.body.stage, fieldPath: cause.body.fieldPath, output };
  }
  return { code: "network_error", message: cause instanceof Error ? cause.message : "请求失败" };
}

export function createApplicationStore(profiles: ProfileStore = useProfileStore(), api: ApplicationApi = applicationApi, sources?: ReferenceSources, serverResolution: Readonly<Ref<boolean>> = computed(() => useCapabilitiesStore().capabilities.value?.serverReferenceResolution === true)): ApplicationStore {
  const previewResult = ref<PreviewResult | null>(null);
  const validation = ref<ValidationResult | null>(null);
  const error = ref<ApplicationError | null>(null);
  const applying = ref(false);
  const preparing = ref(false);
  const requesting = ref(false);
  let preparation: Promise<{ id: string; revision: number } | null> | null = null;
  const active = () => ({ id: profiles.activeProfileId.value, revision: profiles.draft.value?.revision });
  async function prepare(): Promise<{ id: string; revision: number } | null> {
    if (preparation) return preparation;
    preparing.value = true;
    preparation = (async () => {
      try {
        if (serverResolution.value) {
          if (profiles.saveState.value === "saving") await profiles.waitForSave();
          const saved = active();
          if (!saved.id || !saved.revision || profiles.saveState.value !== "saved") {
            throw new ReferenceSyncError("请先等待用户编辑保存完成，并处理保存错误或冲突。", "");
          }
          return { id: saved.id, revision: saved.revision };
        }
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
        error.value = { ...errorFrom(cause), stage: serverResolution.value ? "resolve_references" : "sync_references", ...(cause instanceof ReferenceSyncError ? { code: "reference_sync_failed", fieldPath: cause.fieldPath || undefined } : {}) };
        return null;
      }
    })().finally(() => { preparation = null; preparing.value = false; });
    return preparation;
  }
  const isCurrent = (value: { id: string; revision: number }) => profiles.activeProfileId.value === value.id
    && profiles.draft.value?.revision === value.revision && profiles.saveState.value === "saved";
  async function preview(): Promise<void> {
    if (requesting.value) return;
    error.value = null; previewResult.value = null; requesting.value = true;
    try {
      const value = await prepare(); if (!value) return;
      const result = await api.preview(value.id, value.revision);
      if (isCurrent(value)) previewResult.value = result;
    } catch (cause) { error.value = errorFrom(cause); }
    finally { requesting.value = false; }
  }
  async function fetchCandidate(): Promise<void> { await preview(); }
  async function validate(): Promise<void> {
    if (requesting.value) return;
    error.value = null; validation.value = null; requesting.value = true;
    try {
      const value = await prepare(); if (!value) return;
      const result = await api.validate(value.id, value.revision);
      if (isCurrent(value)) validation.value = result;
      await profiles.refresh();
    } catch (cause) { error.value = errorFrom(cause); }
    finally { requesting.value = false; }
  }
  async function apply(): Promise<ApplyResult | null> {
    if (requesting.value) return null;
    error.value = null; validation.value = null;
    requesting.value = true; applying.value = true;
    try {
      const value = await prepare();
      if (!value) return null;
      const result = await api.apply(value.id, { revision: value.revision });
      if (isCurrent(value)) validation.value = result.validation;
      await profiles.refresh();
      return isCurrent(value) ? result : null;
    } catch (cause) {
      error.value = errorFrom(cause);
      return null;
    } finally {
      applying.value = false; requesting.value = false;
    }
  }
  return {
    previewResult, validation, error, applying, preparing, requesting, preview, fetchCandidate, validate, apply,
    clear: () => { error.value = null; validation.value = null; previewResult.value = null; },
    dispose: () => undefined
  };
}

const defaultStore = createApplicationStore();
export function useApplicationStore(): ApplicationStore { return defaultStore; }
