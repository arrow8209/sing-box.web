import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { routerKey } from "vue-router";
import DashboardView from "@/features/dashboard/DashboardView.vue";
import AppearanceSettings from "@/features/settings/AppearanceSettings.vue";
import CoreSettings from "@/features/settings/CoreSettings.vue";
import PasswordSettings from "@/features/settings/PasswordSettings.vue";
import ProxySettings from "@/features/settings/ProxySettings.vue";

describe("仪表盘与设置", () => {
  afterEach(() => localStorage.clear());
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
  it("核心设置只展示真实状态和稳定版能力", async () => {
    const api = {
      status: vi.fn(async () => ({ installedVersion: "1.12.2", installedDigest: "", state: "running" as const, pid: 4321, currentProfileId: "profile-a", appliedRevision: 1, expectedRunning: true, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } })),
      latest: vi.fn(async () => ({ version: "1.12.3", arch: "amd64" }))
    };
    const wrapper = mount(CoreSettings, {
      props: { api },
      global: { stubs: { RouterLink: { template: '<a href="/core"><slot /></a>' } } }
    });
    await flushPromises();
    expect(wrapper.text()).toContain("v1.12.2");
    expect(wrapper.text()).toContain("Stable · 1.12.3");
    expect(wrapper.text()).toContain("当前 1.12.2 · 最新 1.12.3");
    expect(wrapper.get('[aria-label="开机自动运行（只读）"]').attributes("aria-checked")).toBe("true");
    expect(wrapper.get('[aria-label="配置变更自动重载（只读）"]').attributes("aria-checked")).toBe("false");
    expect(wrapper.text()).not.toContain("Beta");
  });
  it("仪表盘只渲染服务端摘要，不含模拟运行指标", async () => {
    const core = { installedVersion: "1.12.2", installedDigest: "", state: "running", pid: 4321, currentProfileId: "profile-a", appliedRevision: 1, expectedRunning: true, configDirectoryDigest: "", startedAt: null, lastError: "", operation: { active: false } };
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const path = String(input); const data = path.endsWith("/core") ? core : path.endsWith("/profiles") ? [{ id: "profile-a", name: "家庭出口", draftRevision: 1, appliedRevision: 1, appliedDigest: "", appliedAt: null, needsApply: false, lastValidation: null }] : [];
      return Promise.resolve(new Response(JSON.stringify(data), { headers: { "Content-Type": "application/json" } }));
    }));
    const wrapper = mount(DashboardView); await flushPromises();
    expect(wrapper.text()).toContain("PID 4321"); expect(wrapper.text()).toContain("家庭出口");
    for (const forbidden of ["实时流量", "内存占用", "连接数", "延迟", "206 ms"]) expect(wrapper.text()).not.toContain(forbidden);
  });
});
