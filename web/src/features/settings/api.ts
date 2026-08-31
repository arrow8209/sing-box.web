import { request } from "@/api/client";

export interface ProxySettings {
  proxyUrl: string;
}

export interface ProxySettingsApi {
  get(): Promise<ProxySettings>;
  update(input: ProxySettings): Promise<ProxySettings>;
}

export interface CoreBehaviorSettings {
  autoStart: boolean;
  autoReload: boolean;
}

export interface CoreBehaviorSettingsApi {
  get(): Promise<CoreBehaviorSettings>;
  update(input: CoreBehaviorSettings): Promise<CoreBehaviorSettings>;
}

export const proxySettingsApi: ProxySettingsApi = {
  async get() {
    return (await request<ProxySettings>("/settings/proxy")).data;
  },
  async update(input) {
    return (await request<ProxySettings>("/settings/proxy", { method: "PUT", body: input })).data;
  }
};

export const coreBehaviorSettingsApi: CoreBehaviorSettingsApi = {
  async get() {
    return (await request<CoreBehaviorSettings>("/settings/core")).data;
  },
  async update(input) {
    return (await request<CoreBehaviorSettings>("/settings/core", { method: "PUT", body: input })).data;
  }
};
