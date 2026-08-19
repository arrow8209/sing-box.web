import { defineStore } from "pinia";
import { apiClient } from "../api/client";
import { useAuthStore } from "./auth";
import type { CapabilitySnapshot } from "../api/types";

export type CapabilityKey = keyof CapabilitySnapshot["capabilities"];

interface CapabilitiesState {
  snapshot: CapabilitySnapshot | null;
  loading: boolean;
}

export const useCapabilitiesStore = defineStore("capabilities", {
  state: (): CapabilitiesState => ({
    snapshot: null,
    loading: false,
  }),
  actions: {
    async load(): Promise<void> {
      const auth = useAuthStore();
      if (!auth.session) {
        return;
      }
      this.loading = true;
      try {
        const { data } = await apiClient.request<CapabilitySnapshot>("/capabilities");
        this.snapshot = data;
      } finally {
        this.loading = false;
      }
    },
    can(key: CapabilityKey): boolean {
      return this.snapshot?.capabilities[key] ?? false;
    },
  },
});
