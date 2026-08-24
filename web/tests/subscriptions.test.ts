import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import SubscriptionJsonDialog from "@/features/subscriptions/SubscriptionJsonDialog.vue";
import SubscriptionsView from "@/features/subscriptions/SubscriptionsView.vue";
import { validateSubscriptionJson } from "@/features/subscriptions/json";
import type { SubscriptionInput } from "@/features/subscriptions/types";

describe("JSON 订阅", () => {
  it("接受完整配置、nodes 或节点数组并完整保留节点字段", () => {
    const complete = validateSubscriptionJson(JSON.stringify({
      name: "  家庭节点  ",
      log: { level: "debug" },
      outbounds: [
        { type: " trojan ", tag: " 新加坡 01 ", server: "sg.example.test", tls: { enabled: true }, password: "keep-me" },
        { type: "vless", tag: "香港 01", server: "hk.example.test", packet_encoding: "xudp" }
      ]
    }));
    expect(complete.valid).toBe(true);
    if (!complete.valid) return;
    expect(complete.name).toBe("家庭节点");
    expect(complete.message).toBe("JSON 有效 · 将导入 2 个节点 · 2 种协议");
    expect(complete.outbounds[0]).toMatchObject({ type: "trojan", tag: "新加坡 01", password: "keep-me", tls: { enabled: true } });
    expect(complete.outbounds[1]).toMatchObject({ packet_encoding: "xudp" });

    expect(validateSubscriptionJson(`{"nodes":[{"type":"direct","tag":"direct"}]}`).valid).toBe(true);
    expect(validateSubscriptionJson(`[{"type":"block","tag":"block"}]`).valid).toBe(true);
  });

  it("拒绝空节点、缺失字段和规范化后重复的标签", () => {
    expect(validateSubscriptionJson(`{"outbounds":[]}`)).toMatchObject({ valid: false, message: expect.stringContaining("至少添加一个") });
    expect(validateSubscriptionJson(`[{"type":"trojan"}]`)).toMatchObject({ valid: false, message: expect.stringContaining("第 1 个节点") });
    expect(validateSubscriptionJson(`[{"type":"trojan","tag":"香港"},{"type":"vless","tag":" 香港 "}]`)).toEqual({ valid: false, message: "节点标签不能重复：香港" });
  });

  it("编辑器提交规范化后的受管输入，且不发送其他顶层字段", async () => {
    const wrapper = mount(SubscriptionJsonDialog, {
      props: { open: true, subscription: null, suggestedName: "JSON 订阅 1" }
    });
    await wrapper.get("textarea").setValue(JSON.stringify({
      name: "E2E 节点",
      experimental: { cache_file: { enabled: true } },
      outbounds: [{ type: " trojan ", tag: " 新加坡 ", server: "sg.example.test", password: "secret" }]
    }));
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as SubscriptionInput;
    expect(submitted).toEqual({
      name: "E2E 节点",
      outbounds: [{ type: "trojan", tag: "新加坡", server: "sg.example.test", password: "secret" }]
    });
    expect(JSON.stringify(submitted)).not.toContain("experimental");
  });

  it("列表先读取无凭据摘要，仅在查看节点时请求完整详情", async () => {
    const id = "a".repeat(32);
    const summary = { id, name: "家庭节点", nodeCount: 1, protocols: ["trojan"], updatedAt: "2026-08-24T08:00:00Z" };
    const detail = { id, name: "家庭节点", outbounds: [{ type: "trojan", tag: "新加坡", server: "sg.example.test", password: "secret" }], updatedAt: summary.updatedAt };
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input);
      const payload = path.endsWith(`/subscriptions/${id}`) ? detail : [summary];
      return new Response(JSON.stringify(payload), { status: 200, headers: { "Content-Type": "application/json" } });
    });
    vi.stubGlobal("fetch", fetchMock);
    const wrapper = mount(SubscriptionsView);
    await flushPromises();
    expect(wrapper.text()).toContain("家庭节点");
    expect(wrapper.text()).toContain("1 个出站");
    expect(wrapper.text()).not.toContain("secret");
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await wrapper.get(".subscription-footer button").trigger("click");
    await flushPromises();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain(`/subscriptions/${id}`);
    expect(wrapper.get('[aria-label="全部订阅节点"]').text()).toContain("sg.example.test");
  });
});
