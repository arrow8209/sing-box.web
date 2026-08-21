import { request } from "@/api/client";
import type {
  ProfileReference,
  RuleCatalog,
  RuleCatalogImportResult,
  RuleCatalogPreview,
  RuleFormat,
  RuleResource,
  RuleResourceDetail,
  RuleResourceInput,
  RuleResourcePatch,
  RuleUpdateResult
} from "./types";

export interface RuleResourceApi {
  list(): Promise<RuleResource[]>;
  get(id: string): Promise<RuleResourceDetail>;
  create(input: RuleResourceInput): Promise<RuleResource>;
  patch(id: string, input: RuleResourcePatch): Promise<RuleResource>;
  update(id: string): Promise<RuleUpdateResult>;
  remove(id: string): Promise<void>;
  references(id: string): Promise<ProfileReference[]>;
  catalog(): Promise<RuleCatalog>;
  refreshCatalog(): Promise<RuleCatalog>;
  previewCatalog(key: string): Promise<RuleCatalogPreview>;
  importCatalog(key: string, format: RuleFormat, updateInterval: string): Promise<RuleCatalogImportResult>;
}

export const ruleResourceApi: RuleResourceApi = {
  async list() { return (await request<RuleResource[]>("/rule-resources")).data; },
  async get(id) { return (await request<RuleResourceDetail>(`/rule-resources/${id}`)).data; },
  async create(input) { return (await request<RuleResource>("/rule-resources", { method: "POST", body: input })).data; },
  async patch(id, input) { return (await request<RuleResource>(`/rule-resources/${id}`, { method: "PATCH", body: input })).data; },
  async update(id) { return (await request<RuleUpdateResult>(`/rule-resources/${id}/update`, { method: "POST" })).data; },
  async remove(id) { await request<void>(`/rule-resources/${id}`, { method: "DELETE" }); },
  async references(id) { return (await request<{ profiles: ProfileReference[] }>(`/rule-resources/${id}/references`)).data.profiles; },
  async catalog() { return (await request<RuleCatalog>("/rule-resources/catalog")).data; },
  async refreshCatalog() { return (await request<RuleCatalog>("/rule-resources/catalog/refresh", { method: "POST" })).data; },
  async previewCatalog(key) { return (await request<RuleCatalogPreview>(`/rule-resources/catalog/preview?key=${encodeURIComponent(key)}`)).data; },
  async importCatalog(key, format, updateInterval) { return (await request<RuleCatalogImportResult>("/rule-resources/catalog/import", { method: "POST", body: { key, format, updateInterval } })).data; }
};
