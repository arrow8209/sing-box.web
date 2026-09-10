import { enableAutoUnmount, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import DashboardProxySelector from "@/features/dashboard/DashboardProxySelector.vue";

enableAutoUnmount(afterEach);
afterEach(() => vi.useRealTimers());

const testOutbounds = [
  { type: "selector", tag: "主策略组", outbounds: ["香港 01", "新加坡 01", "拦截"] },
  { type: "selector", tag: "备用策略组", outbounds: ["新加坡 01", "日本 01"] },
  { type: "anytls", tag: "香港 01" },
  { type: "anytls", tag: "新加坡 01" },
  { type: "anytls", tag: "日本 01" },
  { type: "block", tag: "拦截" }
];

describe("仪表盘节点选择", () => {
  it("按实际引用顺序显示整个订阅的全部节点和两个单独出站", () => {
    const subscriptionTags = ["🇭🇰 香港 01", "🇭🇰 香港 02", "🇭🇰 香港 03", "🇭🇰 香港 04"];
    const individualTags = ["新加坡 01", "日本 01"];
    const runtimeTags = [...subscriptionTags, ...individualTags];
    const outbounds = [
      { type: "selector", tag: "🚀 节点选择", outbounds: runtimeTags },
      ...subscriptionTags.map((tag) => ({ type: "anytls", tag })),
      { type: "trojan", tag: individualTags[0] },
      { type: "vless", tag: individualTags[1] }
    ];
    const editorState = {
      version: 1,
      groups: [{
        id: "node-selector",
        tag: "🚀 节点选择",
        type: "selector",
        hidden: false,
        include: "",
        exclude: "",
        icon: "",
        references: [
          { kind: "subscription", subscriptionId: "subscription-a", tag: "家庭订阅", resolvedTags: subscriptionTags },
          { kind: "subscription-node", subscriptionId: "subscription-b", nodeTag: individualTags[0], resolvedTags: [individualTags[0]] },
          { kind: "subscription-node", subscriptionId: "subscription-b", nodeTag: individualTags[1], resolvedTags: [individualTags[1]] }
        ],
        runtimeTags
      }],
      generatedTags: runtimeTags
    };

    const wrapper = mount(DashboardProxySelector, { props: { outbounds, editorState } });
    const group = wrapper.get('[data-proxy-group-tag="🚀 节点选择"]');
    const members = group.findAll(".proxy-node-card");

    expect(members).toHaveLength(6);
    expect(members.map((member) => member.attributes("data-proxy-member-tag"))).toEqual(runtimeTags);
    expect(group.text()).toContain("🇭🇰 香港 04");
    expect(members[0]!.get(".proxy-node-select").text()).toBe("🇭🇰 香港 01");
    expect(group.text()).toContain("新加坡 01");
    expect(group.text()).toContain("日本 01");
    expect(wrapper.text()).toContain("6 个节点已配置");
    expect(wrapper.text()).not.toContain("nas-ss");
  });

  it("单节点测速不切换选中项，只同步同一节点在各策略组中的结果", async () => {
    vi.useFakeTimers();
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds } });
    const main = wrapper.get('[data-proxy-group-tag="主策略组"]');
    const first = main.get('[data-proxy-member-tag="香港 01"]');
    const target = main.get('[data-proxy-member-tag="新加坡 01"]');
    const other = wrapper.get('[data-proxy-member-tag="日本 01"]');
    const shared = wrapper.get('[data-proxy-group-tag="备用策略组"] [data-proxy-member-tag="新加坡 01"]');

    await target.get(".proxy-latency").trigger("click");
    expect(target.get(".proxy-latency").text()).toBe("测速中");
    expect(shared.get(".proxy-latency").text()).toBe("测速中");
    expect(first.get(".proxy-latency").text()).toBe("未测速");
    expect(other.get(".proxy-latency").text()).toBe("未测速");
    expect(first.get(".proxy-node-select").attributes("aria-pressed")).toBe("true");
    expect(target.get(".proxy-node-select").attributes("aria-pressed")).toBe("false");
    expect(wrapper.emitted("notice")).toBeUndefined();

    await vi.advanceTimersByTimeAsync(700);
    expect(target.get(".proxy-latency").text()).toMatch(/^\d+ ms$/);
    expect(shared.get(".proxy-latency").text()).toBe(target.get(".proxy-latency").text());
    expect(first.get(".proxy-latency").text()).toBe("未测速");
    expect(other.get(".proxy-latency").text()).toBe("未测速");
    expect(wrapper.emitted("notice")).toEqual([["新加坡 01 延迟已刷新。"]]);

    await target.get(".proxy-node-select").trigger("click");
    expect(target.get(".proxy-node-select").attributes("aria-pressed")).toBe("true");
    expect(first.get(".proxy-node-select").attributes("aria-pressed")).toBe("false");
    await target.get(".proxy-latency").trigger("click");
    expect(target.get(".proxy-latency").text()).toBe("测速中");
  });

  it("策略组和全部测速只更新对应节点，并跳过不适用的出站", async () => {
    vi.useFakeTimers();
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds } });
    const other = wrapper.get('[data-proxy-member-tag="日本 01"]');
    const blocked = wrapper.get('[data-proxy-member-tag="拦截"]');

    await wrapper.get('[aria-label="测试 主策略组 延迟"]').trigger("click");
    expect(other.get(".proxy-latency").text()).toBe("未测速");
    expect(blocked.get(".proxy-latency").text()).toBe("不适用");
    expect(blocked.find("button.proxy-latency").exists()).toBe(false);
    await vi.advanceTimersByTimeAsync(700);
    expect(other.get(".proxy-latency").text()).toBe("未测速");

    await wrapper.get(".proxy-selector-actions button").trigger("click");
    expect(other.get(".proxy-latency").text()).toBe("测速中");
    await vi.advanceTimersByTimeAsync(700);
    for (const button of wrapper.findAll("button.proxy-latency")) {
      expect(button.text()).toMatch(/^\d+ ms$/);
    }
    expect(blocked.get(".proxy-latency").text()).toBe("不适用");
  });

  it("切换配置或离开页面时取消未完成的测速", async () => {
    vi.useFakeTimers();
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds } });
    await wrapper.get("button.proxy-latency").trigger("click");
    await wrapper.setProps({ outbounds: testOutbounds.map((outbound) => ({ ...outbound })) });
    await vi.advanceTimersByTimeAsync(700);
    expect(wrapper.get("button.proxy-latency").text()).toBe("未测速");
    expect(wrapper.emitted("notice")).toBeUndefined();

    await wrapper.get("button.proxy-latency").trigger("click");
    wrapper.unmount();
    expect(vi.getTimerCount()).toBe(0);
    expect(wrapper.emitted("notice")).toBeUndefined();
  });
});
