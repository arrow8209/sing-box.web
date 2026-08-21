import { request } from "@/api/client";
import type { ProfileReference, RuleResource, RuleResourceInput, RuleResourcePatch, RuleUpdateResult } from "./types";

export interface RuleResourceApi {
  list(): Promise<RuleResource[]>;
  create(input: RuleResourceInput): Promise<RuleResource>;
  patch(id: string, input: RuleResourcePatch): Promise<RuleResource>;
  update(id: string): Promise<RuleUpdateResult>;
  remove(id: string): Promise<void>;
  references(id: string): Promise<ProfileReference[]>;
}

export const ruleResourceApi: RuleResourceApi = {
  async list() { return (await request<RuleResource[]>("/rule-resources")).data; },
  async create(input) { return (await request<RuleResource>("/rule-resources", { method: "POST", body: input })).data; },
  async patch(id, input) { return (await request<RuleResource>(`/rule-resources/${id}`, { method: "PATCH", body: input })).data; },
  async update(id) { return (await request<RuleUpdateResult>(`/rule-resources/${id}/update`, { method: "POST" })).data; },
  async remove(id) { await request<void>(`/rule-resources/${id}`, { method: "DELETE" }); },
  async references(id) { return (await request<{ profiles: ProfileReference[] }>(`/rule-resources/${id}/references`)).data.profiles; }
};
