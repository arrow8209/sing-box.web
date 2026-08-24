import { request } from "@/api/client";

export interface ProxySettings {
  proxyUrl: string;
}

export interface ProxySettingsApi {
  get(): Promise<ProxySettings>;
  update(input: ProxySettings): Promise<ProxySettings>;
}

export const proxySettingsApi: ProxySettingsApi = {
  async get() {
    return (await request<ProxySettings>("/settings/proxy")).data;
  },
  async update(input) {
    return (await request<ProxySettings>("/settings/proxy", { method: "PUT", body: input })).data;
  }
};
