import { ref, type Ref } from "vue";
import { isApiError } from "@/features/profiles/api";
import { useProfileStore, type ProfileStore } from "@/features/profiles/store";
import { applicationApi, type ApplicationApi } from "./api";
import type { ApplicationError, ApplyResult, PreviewResult, ValidationResult } from "./types";

export interface ApplicationStore {
  previewResult: Ref<PreviewResult | null>; validation: Ref<ValidationResult | null>; error: Ref<ApplicationError | null>;
  applying: Ref<boolean>; driftDigest: Ref<string | null>;
  preview(): Promise<void>; validate(): Promise<void>; apply(confirmedConfigDigest?: string): Promise<ApplyResult | null>; clear(): void; dispose(): void;
}

function errorFrom(cause: unknown): ApplicationError {
  if (isApiError(cause)) {
    const output = [cause.body.details?.stdout, cause.body.details?.stderr].filter((value): value is string => typeof value === "string" && value.length > 0).join("\n");
    return { code: cause.body.code, message: cause.body.message, stage: cause.body.stage, fieldPath: cause.body.fieldPath, output };
  }
  return { code: "network_error", message: cause instanceof Error ? cause.message : "请求失败" };
}

export function createApplicationStore(profiles: ProfileStore = useProfileStore(), api: ApplicationApi = applicationApi): ApplicationStore {
  const previewResult = ref<PreviewResult | null>(null);
  const validation = ref<ValidationResult | null>(null);
  const error = ref<ApplicationError | null>(null);
  const applying = ref(false);
  const driftDigest = ref<string | null>(null);
  const active = () => ({ id: profiles.activeProfileId.value, revision: profiles.draft.value?.revision });
  async function prepare(): Promise<{ id: string; revision: number } | null> {
    await profiles.flush(); const value = active();
    return value.id && value.revision && profiles.saveState.value === "saved" ? { id: value.id, revision: value.revision } : null;
  }
  async function preview(): Promise<void> { error.value = null; const value = await prepare(); if (!value) return; try { previewResult.value = await api.preview(value.id, value.revision); } catch (cause) { error.value = errorFrom(cause); } }
  async function validate(): Promise<void> { error.value = null; validation.value = null; const value = await prepare(); if (!value) return; try { validation.value = await api.validate(value.id, value.revision); await profiles.refresh(); } catch (cause) { error.value = errorFrom(cause); } }
  async function apply(confirmedConfigDigest?: string): Promise<ApplyResult | null> {
    error.value = null; validation.value = null; driftDigest.value = null;
    const value = await prepare();
    if (!value) return null;
    applying.value = true;
    try {
      const result = await api.apply(value.id, { revision: value.revision, confirmedConfigDigest });
      validation.value = result.validation;
      await profiles.refresh();
      return result;
    } catch (cause) {
      if (isApiError(cause) && cause.status === 409 && cause.body.code === "config_external_drift") {
        const digest = cause.body.details?.confirmationDigest;
        driftDigest.value = typeof digest === "string" ? digest : null;
      } else {
        error.value = errorFrom(cause);
      }
      return null;
    } finally {
      applying.value = false;
    }
  }
  return {
    previewResult, validation, error, applying, driftDigest, preview, validate, apply,
    clear: () => { error.value = null; validation.value = null; previewResult.value = null; driftDigest.value = null; },
    dispose: () => undefined
  };
}

const defaultStore = createApplicationStore();
export function useApplicationStore(): ApplicationStore { return defaultStore; }
