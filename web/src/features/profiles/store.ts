import { computed, ref, toRaw, type Ref } from "vue";
import { isApiError, profileApi, type ProfileApi } from "./api";
import type { Draft, DraftConflict, ProfileSummary, SaveState } from "./types";

const SAVE_DELAY = 600;

export interface ProfileStore {
  profiles: Ref<ProfileSummary[]>;
  activeProfileId: Ref<string | null>;
  draft: Ref<Draft | null>;
  etag: Ref<string | null>;
  saveState: Ref<SaveState>;
  conflict: Ref<DraftConflict | null>;
  hasPendingChanges: Ref<boolean>;
  refresh(): Promise<void>;
  open(id: string): Promise<boolean>;
  mutate(change: (draft: Draft) => void): void;
  flush(): Promise<void>;
  loadServerVersion(): Promise<void>;
  overwriteServerVersion(): Promise<void>;
  create(name: string): Promise<ProfileSummary>;
  rename(id: string, name: string): Promise<ProfileSummary>;
  remove(id: string, confirmName: string, stopCurrent: boolean): Promise<void>;
  dispose(): void;
}

function copy<T>(value: T): T {
  return structuredClone(toRaw(value));
}

export function createProfileStore(api: ProfileApi = profileApi): ProfileStore {
  const profiles = ref<ProfileSummary[]>([]);
  const activeProfileId = ref<string | null>(null);
  const draft = ref<Draft | null>(null);
  const etag = ref<string | null>(null);
  const serverRevision = ref(0);
  const saveState = ref<SaveState>("idle");
  const conflict = ref<DraftConflict | null>(null);
  let pendingGeneration = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let saveInFlight: Promise<void> | undefined;

  const hasPendingChanges = computed(() => ["dirty", "saving", "offline", "conflict", "error"].includes(saveState.value));

  function cancelTimer(): void {
    if (timer !== undefined) clearTimeout(timer);
    timer = undefined;
  }

  function scheduleSave(generation: number, delay = SAVE_DELAY): void {
    cancelTimer();
    timer = setTimeout(() => {
      timer = undefined;
      if (generation === pendingGeneration) void flush();
    }, delay);
  }

  function classifySaveFailure(error: unknown): void {
    if (isApiError(error) && error.status === 409 && error.body.code === "draft_revision_conflict") {
      conflict.value = { latestRevision: error.body.latestRevision };
      saveState.value = "conflict";
      return;
    }
    if (error instanceof TypeError || (typeof navigator !== "undefined" && !navigator.onLine)) {
      saveState.value = "offline";
      return;
    }
    saveState.value = "error";
  }

  async function refresh(): Promise<void> {
    profiles.value = await api.list();
  }

  async function open(id: string): Promise<boolean> {
    if (id === activeProfileId.value) return true;
    await flush();
    if (hasPendingChanges.value) return false;
    const profile = profiles.value.find((item) => item.id === id);
    if (profile?.quarantinedError) return false;
    const response = await api.getDraft(id);
    activeProfileId.value = id;
    draft.value = response.data;
    etag.value = response.etag;
    serverRevision.value = response.data.revision;
    conflict.value = null;
    saveState.value = "saved";
    return true;
  }

  function mutate(change: (value: Draft) => void): void {
    if (!draft.value || saveState.value === "conflict") return;
    change(draft.value);
    draft.value.revision = serverRevision.value;
    saveState.value = "dirty";
    pendingGeneration += 1;
    scheduleSave(pendingGeneration);
  }

  async function save(): Promise<void> {
    if (!draft.value || !etag.value || !activeProfileId.value || !["dirty", "offline", "error"].includes(saveState.value)) return;
    const generation = pendingGeneration;
    const payload = copy(draft.value);
    const id = activeProfileId.value;
    saveState.value = "saving";
    try {
      const response = await api.saveDraft(id, etag.value, payload);
      etag.value = response.etag;
      serverRevision.value = response.data.revision;
      if (generation === pendingGeneration) {
        draft.value = response.data;
        saveState.value = "saved";
        await refresh();
      } else {
        saveState.value = "dirty";
        scheduleSave(pendingGeneration);
      }
    } catch (error) {
      classifySaveFailure(error);
    }
  }

  async function flush(): Promise<void> {
    cancelTimer();
    if (saveInFlight) await saveInFlight;
    if (!["dirty", "offline", "error"].includes(saveState.value)) return;
    saveInFlight = save();
    try {
      await saveInFlight;
    } finally {
      saveInFlight = undefined;
    }
  }

  async function loadServerVersion(): Promise<void> {
    if (!activeProfileId.value) return;
    const response = await api.getDraft(activeProfileId.value);
    draft.value = response.data;
    etag.value = response.etag;
    serverRevision.value = response.data.revision;
    conflict.value = null;
    saveState.value = "saved";
  }

  async function overwriteServerVersion(): Promise<void> {
    if (!activeProfileId.value || !draft.value) return;
    const local = copy(draft.value);
    const latest = await api.getDraft(activeProfileId.value);
    etag.value = latest.etag;
    serverRevision.value = latest.data.revision;
    draft.value = local;
    conflict.value = null;
    saveState.value = "dirty";
    pendingGeneration += 1;
    await flush();
  }

  async function create(name: string): Promise<ProfileSummary> {
    const created = await api.create(name);
    await refresh();
    await open(created.id);
    return created;
  }

  async function rename(id: string, name: string): Promise<ProfileSummary> {
    const renamed = await api.rename(id, name);
    profiles.value = profiles.value.map((item) => item.id === id ? renamed : item);
    return renamed;
  }

  async function remove(id: string, confirmName: string, stopCurrent: boolean): Promise<void> {
    await api.remove(id, confirmName, stopCurrent);
    if (id === activeProfileId.value) {
      activeProfileId.value = null;
      draft.value = null;
      etag.value = null;
      saveState.value = "idle";
    }
    await refresh();
  }

  const onOnline = () => { if (saveState.value === "offline") void flush(); };
  const onBeforeUnload = (event: BeforeUnloadEvent) => {
    if (hasPendingChanges.value) {
      event.preventDefault();
      event.returnValue = "";
    }
  };
  window.addEventListener("online", onOnline);
  window.addEventListener("beforeunload", onBeforeUnload);

  return { profiles, activeProfileId, draft, etag, saveState, conflict, hasPendingChanges, refresh, open, mutate, flush, loadServerVersion, overwriteServerVersion, create, rename, remove, dispose: () => { cancelTimer(); window.removeEventListener("online", onOnline); window.removeEventListener("beforeunload", onBeforeUnload); } };
}

const defaultStore = createProfileStore();
export function useProfileStore(): ProfileStore { return defaultStore; }
