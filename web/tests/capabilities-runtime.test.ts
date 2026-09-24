import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import RuntimeUnavailableView from "@/features/runtime/RuntimeUnavailableView.vue";
import { UnavailableRuntimeApi } from "@/features/runtime/api";

function mockCapabilities(value: Record<string, boolean>): void {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } })));
}

describe("运行能力边界", () => {
  it("实时指标已接入时引导至仪表盘，不再提示流量尚未提供", async () => {
    mockCapabilities({ runtime: false, traffic: true, connections: true, proxyGroups: true });
    const wrapper = mount(RuntimeUnavailableView);
    await flushPromises();
    expect(wrapper.text()).toContain("实时指标已接入仪表盘");
    expect(wrapper.get('a[href="/"]').text()).toBe("查看仪表盘");
    expect(wrapper.text()).not.toContain("将在后续阶段提供");
    wrapper.unmount();
  });
  it("显示已接入的活动连接与尚不可用的其他运行能力", async () => {
    mockCapabilities({ configuration: true, inboundResources: true, subscriptions: true, ruleResources: true, application: false, coreUpdate: true, runtime: false, traffic: false, connections: true, proxyGroups: false });
    const wrapper = mount(RuntimeUnavailableView);
    await flushPromises();
    expect(wrapper.text()).toContain("其余运行数据尚未接入");
    expect(wrapper.text()).toContain("流量");
    expect(wrapper.get('a[href="/connections"]').text()).toBe("查看活动连接");
    expect(wrapper.find("button[data-runtime-action]").exists()).toBe(false);
  });

  it("运行 API 在一期稳定地返回 501", async () => {
    await expect(new UnavailableRuntimeApi().getStatus()).rejects.toMatchObject({ status: 501, body: { code: "runtime_unavailable" } });
  });
});
