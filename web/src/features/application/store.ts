import { computed, ref, type Ref } from "vue";
import { isApiError } from "@/features/profiles/api";
import { useProfileStore, type ProfileStore } from "@/features/profiles/store";
import { applicationApi, type ApplicationApi } from "./api";
import type { ApplicationError, CoreStatus, PreviewResult, ValidationResult } from "./types";

export interface ApplicationStore {
  core: Ref<CoreStatus | null>; previewResult: Ref<PreviewResult | null>; validation: Ref<ValidationResult | null>;
  error: Ref<ApplicationError | null>; driftDigest: Ref<string | null>; applying: Ref<boolean>; reconnecting: Ref<boolean>;
  canApply: Ref<boolean>; refreshCore(): Promise<void>; preview(): Promise<void>; validate(): Promise<void>;
  apply(confirmedConfigDigest?: string): Promise<void>; clear(): void; dispose(): void;
}

function errorFrom(cause: unknown): ApplicationError {
  if (isApiError(cause)) {
    const output = [cause.body.details?.stdout, cause.body.details?.stderr].filter((value): value is string => typeof value === "string").join("\n");
    return { code: cause.body.code, message: cause.body.message, stage: cause.body.stage, fieldPath: cause.body.fieldPath, output };
  }
  return { code: "network_error", message: cause instanceof Error ? cause.message : "请求失败" };
}

export function createApplicationStore(profiles: ProfileStore = useProfileStore(), api: ApplicationApi = applicationApi): ApplicationStore {
  const core = ref<CoreStatus | null>(null);
  const previewResult = ref<PreviewResult | null>(null);
  const validation = ref<ValidationResult | null>(null);
  const error = ref<ApplicationError | null>(null);
  const driftDigest = ref<string | null>(null);
  const applying = ref(false);
  const reconnecting = ref(false);
  let pollTimer: ReturnType<typeof setInterval> | undefined;
  const canApply = computed(() => Boolean(profiles.activeProfileId.value && profiles.draft.value && profiles.saveState.value === "saved" && !core.value?.operation.active && core.value?.state !== "not_installed" && !applying.value));
  const active = () => ({ id: profiles.activeProfileId.value, revision: profiles.draft.value?.revision });
  async function refreshCore(): Promise<void> { core.value = await api.core(); }
  async function prepare(): Promise<{ id: string; revision: number } | null> {
    await profiles.flush(); const value = active();
    return value.id && value.revision && profiles.saveState.value === "saved" ? { id: value.id, revision: value.revision } : null;
  }
  async function preview(): Promise<void> { error.value = null; const value = await prepare(); if (!value) return; try { previewResult.value = await api.preview(value.id, value.revision); } catch (cause) { error.value = errorFrom(cause); } }
  async function validate(): Promise<void> { error.value = null; const value = await prepare(); if (!value) return; try { validation.value = await api.validate(value.id, value.revision); await profiles.refresh(); } catch (cause) { error.value = errorFrom(cause); } }
  function startPolling(): void { if (pollTimer) return; pollTimer = setInterval(() => { void refreshCore().catch(() => undefined); }, 300); }
  function stopPolling(): void { if (pollTimer) clearInterval(pollTimer); pollTimer = undefined; }
  async function reloadActualState(): Promise<void> {
    reconnecting.value = true;
    try { await refreshCore(); await profiles.refresh(); if (profiles.activeProfileId.value) await profiles.loadServerVersion(); }
    finally { reconnecting.value = false; }
  }
  async function apply(confirmedConfigDigest?: string): Promise<void> {
    error.value = null; driftDigest.value = null; const value = await prepare();
    if (!value || !canApply.value) return;
    applying.value = true; startPolling();
    try { await api.apply(value.id, { revision: value.revision, confirmedConfigDigest }); await reloadActualState(); }
    catch (cause) {
      if (isApiError(cause) && cause.status === 409 && cause.body.code === "config_external_drift") {
        const digest = cause.body.details?.confirmationDigest; driftDigest.value = typeof digest === "string" ? digest : null;
      } else if (cause instanceof TypeError) { await reloadActualState(); }
      else error.value = errorFrom(cause);
    } finally {
      applying.value = false; stopPolling();
      // The server owns transaction recovery. Always re-read it instead of
      // inferring success from a lost response or a locally caught error.
      try { await refreshCore(); await profiles.refresh(); if (profiles.activeProfileId.value) await profiles.loadServerVersion(); } catch { /* preserve the original actionable error */ }
    }
  }
  return { core, previewResult, validation, error, driftDigest, applying, reconnecting, canApply, refreshCore, preview, validate, apply, clear: () => { error.value = null; validation.value = null; previewResult.value = null; }, dispose: stopPolling };
}

const defaultStore = createApplicationStore();
export function useApplicationStore(): ApplicationStore { return defaultStore; }
