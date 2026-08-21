import { ref, type Ref } from "vue";
import { ruleResourceApi, type RuleResourceApi } from "./api";
import type { RuleResource, RuleResourceInput, RuleResourcePatch, RuleUpdateResult } from "./types";

export interface RuleResourceStore {
  resources: Ref<RuleResource[]>;
  updatingId: Ref<string | null>;
  refresh(): Promise<void>;
  create(input: RuleResourceInput): Promise<RuleResource>;
  patch(id: string, input: RuleResourcePatch): Promise<RuleResource>;
  update(id: string): Promise<RuleUpdateResult>;
  remove(id: string): Promise<void>;
}

export function createRuleResourceStore(api: RuleResourceApi = ruleResourceApi): RuleResourceStore {
  const resources = ref<RuleResource[]>([]);
  const updatingId = ref<string | null>(null);
  function replace(resource: RuleResource): RuleResource {
    const index = resources.value.findIndex((item) => item.id === resource.id);
    if (index < 0) resources.value = [...resources.value, resource];
    else resources.value = resources.value.map((item) => item.id === resource.id ? resource : item);
    return resource;
  }
  return {
    resources, updatingId,
    async refresh() { resources.value = await api.list(); },
    async create(input) { return replace(await api.create(input)); },
    async patch(id, input) { return replace(await api.patch(id, input)); },
    async update(id) {
      updatingId.value = id;
      try { const result = await api.update(id); await this.refresh(); return result; }
      finally { updatingId.value = null; }
    },
    async remove(id) { await api.remove(id); resources.value = resources.value.filter((item) => item.id !== id); }
  };
}

const defaultStore = createRuleResourceStore();
export function useRuleResourceStore(): RuleResourceStore { return defaultStore; }
