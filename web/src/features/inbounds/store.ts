import { ref, type Ref } from "vue";
import { inboundApi, type InboundApi } from "./api";
import type { InboundDetail, InboundInput, InboundSummary } from "./types";

export interface InboundStore {
  resources: Ref<InboundSummary[]>;
  loading: Ref<boolean>;
  refresh(): Promise<void>;
  get(id: string): Promise<InboundDetail>;
  create(input: InboundInput): Promise<InboundSummary>;
  replace(id: string, input: InboundInput): Promise<InboundSummary>;
  remove(id: string): Promise<void>;
}

export function createInboundStore(api: InboundApi = inboundApi): InboundStore {
  const resources = ref<InboundSummary[]>([]);
  const loading = ref(false);

  function upsert(resource: InboundSummary): InboundSummary {
    const exists = resources.value.some((item) => item.id === resource.id);
    resources.value = exists
      ? resources.value.map((item) => item.id === resource.id ? resource : item)
      : [resource, ...resources.value];
    return resource;
  }

  return {
    resources,
    loading,
    async refresh() {
      loading.value = true;
      try { resources.value = await api.list(); } finally { loading.value = false; }
    },
    async get(id) { return await api.get(id); },
    async create(input) { return upsert(await api.create(input)); },
    async replace(id, input) { return upsert(await api.replace(id, input)); },
    async remove(id) {
      await api.remove(id);
      resources.value = resources.value.filter((item) => item.id !== id);
    }
  };
}

const defaultStore = createInboundStore();
export function useInboundStore(): InboundStore { return defaultStore; }
