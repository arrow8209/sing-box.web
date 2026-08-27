import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import RuntimeUnavailableView from "@/features/runtime/RuntimeUnavailableView.vue";
import { UnavailableRuntimeApi } from "@/features/runtime/api";

function mockCapabilities(value: Record<string, boolean>): void {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } })));
}

describe("一期运行能力边界", () => {
  it("将二期运行能力显示为不可用", async () => {
    mockCapabilities({ configuration: true, inboundResources: true, subscriptions: true, ruleResources: true, application: false, coreUpdate: true, runtime: false, traffic: false, connections: false, proxyGroups: false });
    const wrapper = mount(RuntimeUnavailableView);
    await flushPromises();
    expect(wrapper.text()).toContain("二期尚未接入");
    expect(wrapper.text()).toContain("流量");
    expect(wrapper.find("button[data-runtime-action]").exists()).toBe(false);
  });

  it("运行 API 在一期稳定地返回 501", async () => {
    await expect(new UnavailableRuntimeApi().getStatus()).rejects.toMatchObject({ status: 501, body: { code: "runtime_unavailable" } });
  });
});
