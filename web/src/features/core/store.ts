import { computed, ref, type Ref } from "vue";
import { coreApi, type CoreApi } from "./api";
import type { CoreStatus, CoreUpdateStatus, LatestRelease } from "./types";

export interface CoreStore {
  status: Ref<CoreStatus | null>;
  latest: Ref<LatestRelease | null>;
  update: Ref<CoreUpdateStatus | null>;
  error: Ref<string | null>;
  updateAvailable: Ref<boolean>;
  refresh(): Promise<void>;
  refreshLatest(): Promise<void>;
  refreshUpdate(): Promise<void>;
  startUpdate(version: string): Promise<void>;
  startPolling(): void;
  stopPolling(): void;
  dispose(): void;
}

export function createCoreStore(api: CoreApi = coreApi): CoreStore {
  const status = ref<CoreStatus | null>(null);
  const latest = ref<LatestRelease | null>(null);
  const update = ref<CoreUpdateStatus | null>(null);
  const error = ref<string | null>(null);
  let coreTimer: ReturnType<typeof setInterval> | undefined;
  let updateTimer: ReturnType<typeof setInterval> | undefined;
  let active = false;
  const updateAvailable = computed(() => Boolean(latest.value && (latest.value.updateAvailable ?? (latest.value.version !== status.value?.installedVersion))));
  const updateInProgress = (value: CoreUpdateStatus | null): boolean => Boolean(value?.stage && value.stage !== "idle" && !value.finishedAt && !["completed", "failed"].includes(value.stage));

  const capture = async (action: () => Promise<void>): Promise<void> => {
    try { await action(); error.value = null; } catch (cause) { error.value = cause instanceof Error ? cause.message : "无法读取核心状态"; }
  };
  async function refresh(): Promise<void> { await capture(async () => { status.value = await api.status(); }); }
  async function refreshLatest(): Promise<void> { await capture(async () => { latest.value = await api.latest(); }); }
  async function refreshUpdate(): Promise<void> {
    let succeeded = false;
    await capture(async () => { update.value = await api.updateStatus(); succeeded = true; });
    if (!succeeded) return;
    const finished = update.value?.finishedAt || ["completed", "failed"].includes(update.value?.stage ?? "");
    if (finished) {
      stopUpdatePolling();
      await Promise.all([refresh(), refreshLatest()]);
    } else if (active && updateInProgress(update.value)) {
      startUpdatePolling();
    } else {
      stopUpdatePolling();
    }
  }
  function tick(): void { if (active && !document.hidden) void refresh(); }
  function startUpdatePolling(): void { if (!updateTimer) updateTimer = setInterval(() => { if (active && !document.hidden) void refreshUpdate(); }, 500); }
  function stopUpdatePolling(): void { if (updateTimer) clearInterval(updateTimer); updateTimer = undefined; }
  function onVisibility(): void {
    if (!document.hidden && active) { void refresh(); if (updateInProgress(update.value)) void refreshUpdate(); }
  }
  function startPolling(): void {
    active = true;
    document.addEventListener("visibilitychange", onVisibility);
    if (!coreTimer) coreTimer = setInterval(tick, 2000);
    void refresh();
    void refreshUpdate();
  }
  function stopPolling(): void {
    active = false;
    if (coreTimer) clearInterval(coreTimer);
    coreTimer = undefined;
    stopUpdatePolling();
    document.removeEventListener("visibilitychange", onVisibility);
  }
  async function startUpdate(version: string): Promise<void> {
    await capture(async () => { await api.startUpdate({ confirmedVersion: version }); });
    if (!error.value) { await refreshUpdate(); startUpdatePolling(); }
  }
  return { status, latest, update, error, updateAvailable, refresh, refreshLatest, refreshUpdate, startUpdate, startPolling, stopPolling, dispose: stopPolling };
}

const defaultStore = createCoreStore();
export function useCoreStore(): CoreStore { return defaultStore; }
