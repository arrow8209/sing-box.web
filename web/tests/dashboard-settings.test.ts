import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { routerKey } from "vue-router";
import DashboardView from "@/features/dashboard/DashboardView.vue";
import AppearanceSettings from "@/features/settings/AppearanceSettings.vue";
import PasswordSettings from "@/features/settings/PasswordSettings.vue";

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
