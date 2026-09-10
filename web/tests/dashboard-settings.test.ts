import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { routerKey } from "vue-router";
import DashboardView from "@/features/dashboard/DashboardView.vue";
import type { CoreStatus } from "@/features/core/types";
import { useProfileStore } from "@/features/profiles/store";
import AppearanceSettings from "@/features/settings/AppearanceSettings.vue";
import CoreSettings from "@/features/settings/CoreSettings.vue";
import PasswordSettings from "@/features/settings/PasswordSettings.vue";
import ProxySettings from "@/features/settings/ProxySettings.vue";

describe("仪表盘与设置", () => {
  afterEach(() => {
    localStorage.clear();
    const profiles = useProfileStore();
    profiles.activeProfileId.value = null;
    profiles.draft.value = null;
  });
  it("外观偏好只写入白名单键", async () => {
    const wrapper = mount(AppearanceSettings); await wrapper.get("select").setValue("dark");
    expect(localStorage.getItem("singbox-theme")).toBe("dark");
    expect(Object.keys(localStorage)).toEqual(["singbox-theme"]);
  });
  it("新密码不一致时不发送修改请求", async () => {
    const fetchMock = vi.fn(); vi.stubGlobal("fetch", fetchMock);
    const wrapper = mount(PasswordSettings, { global: { provide: { [routerKey as symbol]: { replace: vi.fn() } } } });
    const inputs = wrapper.findAll("input"); await inputs[0].setValue("current"); await inputs[1].setValue("abcdef"); await inputs[2].setValue("xxxxxx"); await wrapper.get("form").trigger("submit");
    expect(fetchMock).not.toHaveBeenCalled(); expect(wrapper.text()).toContain("两次输入的新密码不一致");
  });
  it("读取并保存服务端下载代理设置", async () => {
    const api = {
      get: vi.fn(async () => ({ proxyUrl: "" })),
      update: vi.fn(async (input: { proxyUrl: string }) => input)
    };
    const wrapper = mount(ProxySettings, { props: { api } });
    await flushPromises();
    const input = wrapper.get("input[type=url]");
    expect((input.element as HTMLInputElement).value).toBe("");
    expect(wrapper.text()).toContain("直连");
    await input.setValue("  http://127.0.0.1:7890  ");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.update).toHaveBeenCalledWith({ proxyUrl: "http://127.0.0.1:7890" });
    expect(wrapper.text()).toContain("代理设置已保存并即时生效");
    expect(wrapper.text()).toContain("使用代理");
  });
  it("读取并保存核心行为设置", async () => {
    let behavior = { autoStart: true, autoReload: false };
    const api = {
      status: vi.fn(async () => ({ installedVersion: "1.12.2", installedDigest: "", state: "running" as const, pid: 4321, currentProfileId: "profile-a", appliedRevision: 1, expectedRunning: true, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } })),
      latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" })),
      get: vi.fn(async () => behavior),
      update: vi.fn(async (input: typeof behavior) => { behavior = input; return behavior; })
    };
    const wrapper = mount(CoreSettings, {
      props: { api },
      global: { stubs: { RouterLink: { template: '<a href="/core"><slot /></a>' } } }
    });
    await flushPromises();
    expect(wrapper.text()).toContain("v1.12.2");
    expect(wrapper.text()).toContain("Stable · 1.12.3");
    expect(wrapper.text()).toContain("当前 1.12.2 · 最新 1.12.3");
    const autoStart = wrapper.get('[aria-label="开机自动运行"]');
    const autoReload = wrapper.get('[aria-label="配置变更自动重载"]');
    const descriptions = wrapper.findAll(".setting-copy > span").map((item) => item.text());
    expect(autoStart.attributes("aria-checked")).toBe("true");
    expect(wrapper.text()).toContain("没有配置则保持停止");
    expect(autoReload.attributes("aria-checked")).toBe("false");
    await autoStart.trigger("click");
    await flushPromises();
    expect(api.update).toHaveBeenLastCalledWith({ autoStart: false, autoReload: false });
    expect(autoStart.attributes("aria-checked")).toBe("false");
    expect(wrapper.get(".app-toast").text()).toBe("开机自动运行已关闭");
    expect(wrapper.findAll(".setting-copy > span").map((item) => item.text())).toEqual(descriptions);
    await autoReload.trigger("click");
    await flushPromises();
    expect(api.update).toHaveBeenLastCalledWith({ autoStart: false, autoReload: true });
    expect(autoReload.attributes("aria-checked")).toBe("true");
    expect(wrapper.get(".app-toast").text()).toBe("配置自动重载已开启");
    expect(wrapper.text()).toContain("保存当前已应用档案会自动检查并应用");
    expect(wrapper.findAll(".setting-copy > span").map((item) => item.text())).toEqual(descriptions);
    expect(wrapper.text()).not.toContain("Beta");
    wrapper.unmount();
  });
  it("核心行为设置保存失败时回滚开关", async () => {
    const api = {
      status: vi.fn(async () => ({ installedVersion: "1.12.2", installedDigest: "", state: "stopped" as const, pid: 0, currentProfileId: "", appliedRevision: null, expectedRunning: true, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } })),
      latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" })),
      get: vi.fn(async () => ({ autoStart: false, autoReload: false })),
      update: vi.fn(async () => { throw new Error("设置写入失败"); })
    };
    const wrapper = mount(CoreSettings, {
      props: { api },
      global: { stubs: { RouterLink: { template: '<a href="/core"><slot /></a>' } } }
    });
    await flushPromises();
    const autoStart = wrapper.get('[aria-label="开机自动运行"]');
    expect(autoStart.attributes("aria-checked")).toBe("false");
    expect(wrapper.text()).toContain("未启用");
    await autoStart.trigger("click");
    await flushPromises();
    expect(autoStart.attributes("aria-checked")).toBe("false");
    expect(wrapper.get('.app-toast[role="alert"]').text()).toBe("设置写入失败");
    wrapper.unmount();
  });
  it("核心设置成功提示自动消失，连续保存会重新计时", async () => {
    vi.useFakeTimers();
    const api = {
      status: vi.fn(async () => ({ installedVersion: "1.13.19", installedDigest: "", state: "stopped" as const, pid: 0, currentProfileId: "", appliedRevision: null, expectedRunning: false, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } })),
      latest: vi.fn(async () => ({ version: "1.13.19", arch: "amd64" })),
      get: vi.fn(async () => ({ autoStart: true, autoReload: false })),
      update: vi.fn(async (input: { autoStart: boolean; autoReload: boolean }) => input)
    };
    const wrapper = mount(CoreSettings, {
      props: { api },
      global: { stubs: { RouterLink: { template: '<a href="/core"><slot /></a>' } } }
    });
    try {
      await flushPromises();
      await wrapper.get('[aria-label="开机自动运行"]').trigger("click");
      await flushPromises();
      await vi.advanceTimersByTimeAsync(2000);
      expect(wrapper.get(".app-toast").text()).toBe("开机自动运行已关闭");

      await wrapper.get('[aria-label="配置变更自动重载"]').trigger("click");
      await flushPromises();
      await vi.advanceTimersByTimeAsync(2000);
      expect(wrapper.get(".app-toast").text()).toBe("配置自动重载已开启");
      await vi.advanceTimersByTimeAsync(1000);
      expect(wrapper.find(".app-toast").exists()).toBe(false);

      await wrapper.get('[aria-label="开机自动运行"]').trigger("click");
      await flushPromises();
      wrapper.unmount();
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      if (wrapper.exists()) wrapper.unmount();
      vi.useRealTimers();
    }
  });
  it("仪表盘按原型渲染完整运行概览并保留服务端状态", async () => {
    const core = { installedVersion: "1.12.2", installedDigest: "", state: "running", pid: 4321, currentProfileId: "profile-a", appliedRevision: 1, expectedRunning: true, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } };
    const outbounds = [
      { type: "selector", tag: "🚀 节点选择", outbounds: ["香港 01", "香港 02", "香港 03", "香港 04", "新加坡 01", "日本 01"] },
      { type: "urltest", tag: "🎈 自动选择", outbounds: ["direct"] },
      { type: "selector", tag: "🎯 全球直连", outbounds: ["direct", "block"] },
      { type: "selector", tag: "🛑 全球拦截", outbounds: ["block", "direct"] },
      { type: "selector", tag: "🐟 漏网之鱼", outbounds: ["🚀 节点选择", "🎯 全球直连"] },
      { type: "selector", tag: "GLOBAL", outbounds: ["🚀 节点选择", "🎈 自动选择", "🎯 全球直连", "🛑 全球拦截", "🐟 漏网之鱼"] },
      ...["香港 01", "香港 02", "香港 03", "香港 04", "新加坡 01", "日本 01"].map((tag) => ({ type: "anytls", tag })),
      { type: "direct", tag: "direct" },
      { type: "block", tag: "block" }
    ];
    const profiles = useProfileStore();
    profiles.activeProfileId.value = "profile-a";
    profiles.draft.value = {
      profileId: "profile-a",
      revision: 1,
      config: { inbounds: [{}], outbounds, route: { rules: new Array(11).fill({}) } },
      buffers: {},
      bindings: [],
      savedAt: "2026-08-31T00:00:00Z"
    };
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      const data = path.endsWith("/core") && init?.method === "DELETE"
        ? { ...core, state: "stopped", pid: 0, expectedRunning: false }
        : path.endsWith("/core") ? core : path.endsWith("/profiles") ? [{ id: "profile-a", name: "家庭出口", draftRevision: 1, appliedRevision: 1, appliedDigest: "", appliedAt: null, needsApply: false, lastValidation: null }] : [];
      return Promise.resolve(new Response(JSON.stringify(data), { headers: { "Content-Type": "application/json" } }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const wrapper = mount(DashboardView); await flushPromises();
    expect(wrapper.text()).toContain("sing-box 正在运行");
    expect(wrapper.text()).toContain("1.12.2");
    expect(wrapper.text()).toContain("家庭出口");
    const processId = wrapper.findAll(".hero-meta-item")[3];
    expect(processId.text()).toContain("进程 ID");
    expect(processId.text()).toContain("4321");
    expect(wrapper.text()).toContain("实时流量");
    expect(wrapper.text()).toContain("内存占用");
    expect(wrapper.findAll(".metric-card")).toHaveLength(4);
    expect(wrapper.get('a.metric-card-link[href="/connections"]').text()).toContain("活动连接");
    expect(wrapper.findAll(".proxy-group")).toHaveLength(6);
    expect(wrapper.get('[data-proxy-group-tag="🚀 节点选择"]').findAll(".proxy-node-card")).toHaveLength(6);
    expect(wrapper.get(".traffic-chart").attributes("aria-label")).toBe("实时流量折线图");
    const systemProxy = wrapper.get('[aria-label="切换系统代理"]');
    const tunMode = wrapper.get('[aria-label="切换 TUN 模式"]');
    expect(systemProxy.attributes("aria-checked")).toBe("false");
    expect(tunMode.attributes("aria-checked")).toBe("false");
    await systemProxy.trigger("click");
    expect(systemProxy.attributes("aria-checked")).toBe("true");
    await wrapper.findAll(".mode-option")[2].trigger("click");
    expect(wrapper.findAll(".mode-option")[2].classes()).toContain("active");
    await wrapper.get("[data-core-toggle]").trigger("click");
    await flushPromises();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/core", expect.objectContaining({ method: "DELETE" }));
    expect(wrapper.text()).toContain("sing-box 已停止");
    expect(wrapper.text()).toContain("sing-box 进程已结束");
    wrapper.unmount();
  });
  it("仪表盘通过真实 API 启动并重启核心", async () => {
    let current: CoreStatus = { installedVersion: "1.12.2", installedDigest: "", state: "stopped", pid: 0, currentProfileId: "profile-a", appliedRevision: 1, expectedRunning: false, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } };
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      if (path.endsWith("/core/restart") && init?.method === "POST") current = { ...current, state: "running", pid: 5102, expectedRunning: true, startedAt: "2026-08-31T00:00:00Z" };
      else if (path.endsWith("/core") && init?.method === "POST") current = { ...current, state: "running", pid: 5101, expectedRunning: true, startedAt: "2026-08-31T00:00:00Z" };
      const data = path.endsWith("/core") || path.endsWith("/core/restart") ? current : path.endsWith("/profiles") ? [] : { operationId: "", stage: "idle", startedAt: "" };
      return Promise.resolve(new Response(JSON.stringify(data), { headers: { "Content-Type": "application/json" } }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const wrapper = mount(DashboardView);
    await flushPromises();

    await wrapper.get("[data-core-toggle]").trigger("click");
    await flushPromises();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/core", expect.objectContaining({ method: "POST" }));
    expect(wrapper.text()).toContain("5101");

    await wrapper.get("[data-core-restart]").trigger("click");
    await flushPromises();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/core/restart", expect.objectContaining({ method: "POST" }));
    expect(wrapper.text()).toContain("5102");
    wrapper.unmount();
  });
});
