import { request } from "@/api/client";
import type { ConnectionsSnapshot } from "./types";

export interface ConnectionsApi {
  list(): Promise<ConnectionsSnapshot>;
  close(id: string): Promise<void>;
  closeAll(): Promise<void>;
}

export const connectionsApi: ConnectionsApi = {
  async list() { return (await request<ConnectionsSnapshot>("/runtime/connections")).data; },
  async close(id) { await request<void>(`/runtime/connections/${encodeURIComponent(id)}`, { method: "DELETE" }); },
  async closeAll() { await request<void>("/runtime/connections", { method: "DELETE" }); }
};
