import { readonly, ref, type Ref } from "vue";
import { request } from "@/api/client";
import type { Capabilities } from "./types";

export interface CapabilitiesApi {
  get(): Promise<Capabilities>;
}

const capabilitiesApi: CapabilitiesApi = {
  async get() { return (await request<Capabilities>("/capabilities")).data; }
};

export interface CapabilitiesStore {
  capabilities: Readonly<Ref<Capabilities | null>>;
  refresh(): Promise<void>;
}

export function createCapabilitiesStore(api: CapabilitiesApi = capabilitiesApi): CapabilitiesStore {
  const capabilities = ref<Capabilities | null>(null);
  return {
    capabilities: readonly(capabilities),
    async refresh() { capabilities.value = await api.get(); }
  };
}

const defaultStore = createCapabilitiesStore();

export function useCapabilitiesStore(): CapabilitiesStore { return defaultStore; }
