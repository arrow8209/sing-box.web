import { request } from "@/api/client";
import type { InboundDetail, InboundInput, InboundSummary } from "./types";

export interface InboundApi {
  list(): Promise<InboundSummary[]>;
  get(id: string): Promise<InboundDetail>;
  create(input: InboundInput): Promise<InboundSummary>;
  replace(id: string, input: InboundInput): Promise<InboundSummary>;
  remove(id: string): Promise<void>;
}

export const inboundApi: InboundApi = {
  async list() { return (await request<InboundSummary[]>("/inbound-resources")).data; },
  async get(id) { return (await request<InboundDetail>(`/inbound-resources/${encodeURIComponent(id)}`)).data; },
  async create(input) { return (await request<InboundSummary>("/inbound-resources", { method: "POST", body: input })).data; },
  async replace(id, input) { return (await request<InboundSummary>(`/inbound-resources/${encodeURIComponent(id)}`, { method: "PUT", body: input })).data; },
  async remove(id) { await request<void>(`/inbound-resources/${encodeURIComponent(id)}`, { method: "DELETE" }); }
};
