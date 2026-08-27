import { request } from "@/api/client";
import type { CoreStatus, CoreUpdateStatus, LatestRelease } from "./types";

export interface CoreApi {
  status(): Promise<CoreStatus>;
  latest(): Promise<LatestRelease>;
  updateStatus(): Promise<CoreUpdateStatus>;
  startUpdate(input: { confirmedVersion: string }): Promise<{ operationId: string }>;
  cancelUpdate(): Promise<{ operationId: string }>;
}

export const coreApi: CoreApi = {
  async status() { return (await request<CoreStatus>("/core")).data; },
  async latest() { return (await request<LatestRelease>("/core/releases/latest")).data; },
  async updateStatus() { return (await request<CoreUpdateStatus>("/core/update")).data; },
  async startUpdate(input) { return (await request<{ operationId: string }>("/core/update", { method: "POST", body: input })).data; },
  async cancelUpdate() { return (await request<{ operationId: string }>("/core/update", { method: "DELETE" })).data; }
};
