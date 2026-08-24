import { ref, type Ref } from "vue";
import { ruleResourceApi, type RuleResourceApi } from "./api";
import type {
  RuleBatchResult,
  RuleCatalog,
  RuleCatalogImportResult,
  RuleCatalogPreview,
  RuleResource,
  RuleResourceDetail,
  RuleResourceInput,
  RuleResourcePatch,
  RuleUpdateResult
} from "./types";

export interface RuleResourceStore {
  resources: Ref<RuleResource[]>;
  catalog: Ref<RuleCatalog | null>;
  updatingId: Ref<string | null>;
  updatingIds: Ref<string[]>;
  catalogLoading: Ref<boolean>;
  refresh(): Promise<void>;
  get(id: string): Promise<RuleResourceDetail>;
  create(input: RuleResourceInput): Promise<RuleResource>;
  patch(id: string, input: RuleResourcePatch): Promise<RuleResource>;
  update(id: string): Promise<RuleUpdateResult>;
  updateAll(items: RuleResource[], concurrency?: number): Promise<RuleBatchResult>;
  remove(id: string): Promise<void>;
  loadCatalog(force?: boolean): Promise<RuleCatalog>;
  previewCatalog(key: string): Promise<RuleCatalogPreview>;
  importCatalog(key: string, updateInterval?: string): Promise<RuleCatalogImportResult>;
}

export function createRuleResourceStore(api: RuleResourceApi = ruleResourceApi): RuleResourceStore {
  const resources = ref<RuleResource[]>([]);
  const catalog = ref<RuleCatalog | null>(null);
  const updatingId = ref<string | null>(null);
  const updatingIds = ref<string[]>([]);
  const catalogLoading = ref(false);

  function replace(resource: RuleResource): RuleResource {
    const index = resources.value.findIndex((item) => item.id === resource.id);
    if (index < 0) resources.value = [...resources.value, resource];
    else resources.value = resources.value.map((item) => item.id === resource.id ? resource : item);
    return resource;
  }
  function startUpdate(id: string): void {
    if (!updatingIds.value.includes(id)) updatingIds.value = [...updatingIds.value, id];
    updatingId.value = id;
  }
  function finishUpdate(id: string): void {
    updatingIds.value = updatingIds.value.filter((item) => item !== id);
    updatingId.value = updatingIds.value.at(-1) ?? null;
  }

  const store: RuleResourceStore = {
    resources, catalog, updatingId, updatingIds, catalogLoading,
    async refresh() { resources.value = await api.list(); },
    async get(id) { return await api.get(id); },
    async create(input) { return replace(await api.create(input)); },
    async patch(id, input) { return replace(await api.patch(id, input)); },
    async update(id) {
      startUpdate(id);
      try {
        const result = await api.update(id);
        await store.refresh();
        return result;
      } finally {
        finishUpdate(id);
      }
    },
    async updateAll(items, concurrency = 5) {
      const queue = items.filter((item) => item.sourceType === "remote" && !item.disabled);
      const result: RuleBatchResult = { total: queue.length, changed: 0, unchanged: 0, failed: 0 };
      let cursor = 0;
      async function worker(): Promise<void> {
        while (cursor < queue.length) {
          const item = queue[cursor++];
          if (!item) return;
          startUpdate(item.id);
          try {
            const updated = await api.update(item.id);
            if (updated.changed) result.changed++;
            else result.unchanged++;
          } catch {
            result.failed++;
          } finally {
            finishUpdate(item.id);
          }
        }
      }
      const workers = Math.min(Math.max(1, concurrency), queue.length);
      if (workers > 0) {
        await Promise.all(Array.from({ length: workers }, () => worker()));
        await store.refresh();
      }
      return result;
    },
    async remove(id) { await api.remove(id); resources.value = resources.value.filter((item) => item.id !== id); },
    async loadCatalog(force = false) {
      catalogLoading.value = true;
      try {
        let value = force ? await api.refreshCatalog() : await api.catalog();
        if (!force && !value.available) value = await api.refreshCatalog();
        catalog.value = value;
        return value;
      } finally {
        catalogLoading.value = false;
      }
    },
    async previewCatalog(key) { return await api.previewCatalog(key); },
    async importCatalog(key, updateInterval = "24h") {
      const result = await api.importCatalog(key, updateInterval);
      replace(result.resource);
      return result;
    }
  };
  return store;
}

const defaultStore = createRuleResourceStore();
export function useRuleResourceStore(): RuleResourceStore { return defaultStore; }
