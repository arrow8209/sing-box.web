import { request } from "@/api/client";

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
