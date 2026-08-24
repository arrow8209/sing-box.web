import { request } from "@/api/client";
import type { SubscriptionDetail, SubscriptionInput, SubscriptionSummary } from "./types";

export interface SubscriptionApi {
  list(): Promise<SubscriptionSummary[]>;
  get(id: string): Promise<SubscriptionDetail>;
  create(input: SubscriptionInput): Promise<SubscriptionSummary>;
  replace(id: string, input: SubscriptionInput): Promise<SubscriptionSummary>;
  remove(id: string): Promise<void>;
}

export const subscriptionApi: SubscriptionApi = {
  async list() { return (await request<SubscriptionSummary[]>("/subscriptions")).data; },
  async get(id) { return (await request<SubscriptionDetail>(`/subscriptions/${id}`)).data; },
  async create(input) { return (await request<SubscriptionSummary>("/subscriptions", { method: "POST", body: input })).data; },
  async replace(id, input) { return (await request<SubscriptionSummary>(`/subscriptions/${id}`, { method: "PUT", body: input })).data; },
  async remove(id) { await request<void>(`/subscriptions/${id}`, { method: "DELETE" }); }
};
