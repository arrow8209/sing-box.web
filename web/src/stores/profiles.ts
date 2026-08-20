import { defineStore } from "pinia";
import { apiClient } from "../api/client";
import { useDraftStore } from "./draft";
import type { Profile } from "../api/types";

interface ProfilesState {
  items: Profile[];
  activeId: string | null;
  loading: boolean;
}

const UNSAVED_DRAFT_STATUSES = new Set(["dirty", "saving", "offline", "conflict"]);

async function flushActiveDraft(activeId: string | null): Promise<void> {
  const draft = useDraftStore();
  const needsFlush =
    activeId !== null &&
    draft.draft?.profileId === activeId &&
    UNSAVED_DRAFT_STATUSES.has(draft.status);
  if (needsFlush && !(await draft.save())) {
    throw new Error(draft.lastError?.message || "草稿尚未保存，无法切换档案");
  }
}

export const useProfilesStore = defineStore("profiles", {
  state: (): ProfilesState => ({
    items: [],
    activeId: null,
    loading: false,
  }),
  getters: {
    active(state): Profile | null {
      return state.items.find((item) => item.id === state.activeId) ?? null;
    },
  },
  actions: {
    async load(): Promise<void> {
      this.loading = true;
      try {
        const { data } = await apiClient.request<Profile[]>("/profiles");
        this.items = data;
      } finally {
        this.loading = false;
      }
    },
    async create(
      input: { name: string; slug: string },
      router?: { push: (path: string) => unknown },
    ): Promise<Profile> {
      await flushActiveDraft(this.activeId);
      const { data } = await apiClient.request<Profile>("/profiles", {
        method: "POST",
        body: JSON.stringify(input),
      });
      this.items = [...this.items, data];
      await this.select(data.id);
      await router?.push(`/config/${data.id}/general`);
      return data;
    },
    async rename(id: string, name: string): Promise<void> {
      const { data } = await apiClient.request<Profile>(`/profiles/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ name }),
      });
      const index = this.items.findIndex((item) => item.id === id);
      if (index !== -1) {
        this.items[index] = { ...this.items[index], ...data };
      }
    },
    async select(id: string): Promise<void> {
      const draft = useDraftStore();
      if (this.activeId === id && draft.draft?.profileId === id) {
        return;
      }

      const currentId = this.activeId;
      await flushActiveDraft(currentId);

      const previousStatus = draft.status;
      const previousError = draft.lastError;
      const previousConflict = draft.conflict;
      try {
        await draft.load(id);
        this.activeId = id;
      } catch (error) {
        draft.status = previousStatus;
        draft.lastError = previousError;
        draft.conflict = previousConflict;
        throw error;
      }
    },
    async remove(id: string): Promise<void> {
      this.items = this.items.filter((item) => item.id !== id);
      if (this.activeId === id) {
        this.activeId = this.items[0]?.id ?? null;
      }
    },
  },
});
