import { request } from "@/api/client";

export interface RuntimeMetrics {
  uploadTotal: number;
  downloadTotal: number;
  memory: number;
  connectionCount: number;
  runtimeKey: string;
  observedAt: string;
}
export interface RuntimeMetricsApi { get(signal?: AbortSignal): Promise<RuntimeMetrics>; }
export const runtimeMetricsApi: RuntimeMetricsApi = {
  async get(signal) { return (await request<RuntimeMetrics>("/runtime/metrics", { signal })).data; }
};

export type RouteMode = "rule" | "global" | "direct";
export interface RuntimeMode { mode: string; availableModes: RouteMode[]; }
export interface RuntimeModeApi {
  get(): Promise<RuntimeMode>;
  set(mode: RouteMode): Promise<RuntimeMode>;
}

export const runtimeModeApi: RuntimeModeApi = {
  async get() { return (await request<RuntimeMode>("/runtime/mode")).data; },
  async set(mode) { return (await request<RuntimeMode>("/runtime/mode", { method: "PUT", body: { mode } })).data; }
};

export interface RuntimeProxyGroup {
  tag: string;
  type: "Selector" | "URLTest";
  selected: string;
  members: Array<{ tag: string; type: string }>;
}

export interface ProxySelection { tag: string; selected: string; }

export interface ProxyGroupsApi {
  list(): Promise<RuntimeProxyGroup[]>;
  select(tag: string, selected: string): Promise<ProxySelection>;
}

export const proxyGroupsApi: ProxyGroupsApi = {
  async list() { return (await request<RuntimeProxyGroup[]>("/runtime/proxy-groups")).data; },
  async select(tag, selected) {
    return (await request<ProxySelection>("/runtime/proxy-groups", { method: "PUT", body: { tag, selected } })).data;
  }
};
