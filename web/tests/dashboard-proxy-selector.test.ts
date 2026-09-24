import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import DashboardProxySelector from "@/features/dashboard/DashboardProxySelector.vue";
import type { ProxySelection, RuntimeProxyGroup } from "@/features/dashboard/api";
import { outboundOrderFromDraft } from "@/features/dashboard/proxy-groups";
import type { Draft } from "@/features/profiles/types";

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

function createApi() {
  let groups: RuntimeProxyGroup[] = [
    { tag: "主策略组", type: "Selector", selected: "香港 01", members: [{ tag: "香港 01", type: "AnyTLS" }, { tag: "新加坡 01", type: "AnyTLS" }, { tag: "拦截", type: "Reject" }] },
    { tag: "备用策略组", type: "Selector", selected: "新加坡 01", members: [{ tag: "新加坡 01", type: "AnyTLS" }, { tag: "日本 01", type: "AnyTLS" }] }
  ];
  return {
    list: vi.fn(async () => structuredClone(groups)),
    select: vi.fn(async (tag: string, selected: string): Promise<ProxySelection> => {
      groups = groups.map((group) => group.tag === tag ? { ...group, selected } : group);
      return { tag, selected };
    })
  };
}

describe("仪表盘节点选择", () => {
  it("按配置页的出站顺序显示策略组，同时保留核心的节点顺序和选择", async () => {
    const outbounds = [
      { type: "selector", tag: "备用组", outbounds: ["节点二", "节点一"] },
      { type: "selector", tag: "主选择", outbounds: ["节点一", "节点二"] },
      { type: "urltest", tag: "自动组", outbounds: ["节点一"] },
      { type: "trojan", tag: "节点一" },
      { type: "vless", tag: "节点二" }
    ];
    const draft: Draft = {
      profileId: "a".repeat(32), revision: 2, schemaVersion: 2,
      config: { outbounds }, buffers: {}, bindings: [], savedAt: "2026-09-24T00:00:00Z",
      references: { version: 2, inbounds: [], outbounds: [
        { id: "main", tag: "主选择", type: "selector", references: [] },
        { id: "backup", tag: "备用组", type: "selector", references: [] },
        { id: "auto", tag: "自动组", type: "urltest", references: [] }
      ] }
    };
    const order = outboundOrderFromDraft(draft);
    expect(order.slice(0, 3)).toEqual(["主选择", "备用组", "自动组"]);
    expect(outboundOrderFromDraft({ ...draft, schemaVersion: 1 })).toEqual(outbounds.map((item) => item.tag));

    const api = { list: vi.fn(async (): Promise<RuntimeProxyGroup[]> => [
      { tag: "备用组", type: "Selector", selected: "节点二", members: [{ tag: "节点二", type: "Vless" }, { tag: "节点一", type: "Trojan" }] },
      { tag: "自动组", type: "URLTest", selected: "节点一", members: [{ tag: "节点一", type: "Trojan" }] },
      { tag: "主选择", type: "Selector", selected: "节点二", members: [{ tag: "节点二", type: "Vless" }, { tag: "节点一", type: "Trojan" }] }
    ]), select: vi.fn(async (tag: string, selected: string) => ({ tag, selected })) };
    const wrapper = mount(DashboardProxySelector, { props: { outbounds, outboundOrder: order, running: false, api } });
    const groupTags = () => wrapper.findAll(".proxy-group").map((group) => group.attributes("data-proxy-group-tag"));
    expect(groupTags()).toEqual(["主选择", "备用组", "自动组"]);

    await wrapper.setProps({ running: true });
    await flushPromises();
    expect(groupTags()).toEqual(["主选择", "备用组", "自动组"]);
    const main = wrapper.get('[data-proxy-group-tag="主选择"]');
    expect(main.findAll(".proxy-node-card").map((member) => member.attributes("data-proxy-member-tag"))).toEqual(["节点二", "节点一"]);
    expect(main.get('[data-proxy-member-tag="节点二"] .proxy-node-select').attributes("aria-pressed")).toBe("true");

    await wrapper.setProps({ outboundOrder: ["备用组", "主选择", "自动组"] });
    expect(groupTags()).toEqual(["备用组", "主选择", "自动组"]);
    expect(wrapper.get('[data-proxy-group-tag="主选择"] [data-proxy-member-tag="节点二"] .proxy-node-select').attributes("aria-pressed")).toBe("true");
  });

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

    const wrapper = mount(DashboardProxySelector, { props: { outbounds, editorState, running: false } });
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
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api: createApi() } });
    await flushPromises();
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
    await flushPromises();
    expect(target.get(".proxy-node-select").attributes("aria-pressed")).toBe("true");
    expect(first.get(".proxy-node-select").attributes("aria-pressed")).toBe("false");
    await target.get(".proxy-latency").trigger("click");
    expect(target.get(".proxy-latency").text()).toBe("测速中");
  });

  it("策略组和全部测速只更新对应节点，并跳过不适用的出站", async () => {
    vi.useFakeTimers();
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api: createApi() } });
    await flushPromises();
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
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api: createApi() } });
    await flushPromises();
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

  it("核心确认后才更新勾选，重新打开页面读取已切换节点", async () => {
    const api = createApi();
    const select = api.select.getMockImplementation()!;
    let confirm!: () => void;
    api.select.mockImplementationOnce((tag, selected) => new Promise((resolve) => {
      confirm = () => { void select(tag, selected).then(resolve); };
    }));
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api } });
    await flushPromises();
    const main = wrapper.get('[data-proxy-group-tag="主策略组"]');
    const first = main.get('[aria-label="选择 香港 01"]');
    const target = main.get('[aria-label="选择 新加坡 01"]');
    await target.trigger("click");
    expect(api.select).toHaveBeenCalledWith("主策略组", "新加坡 01");
    expect(first.attributes("aria-pressed")).toBe("true");
    expect(target.attributes("disabled")).toBeDefined();
    expect(wrapper.emitted("notice")).toBeUndefined();
    await target.trigger("click");
    expect(api.select).toHaveBeenCalledTimes(1);
    confirm();
    await flushPromises();
    expect(target.attributes("aria-pressed")).toBe("true");
    expect(first.attributes("aria-pressed")).toBe("false");
    expect(wrapper.emitted("notice")).toEqual([["主策略组 已切换到 新加坡 01。"]]);
    wrapper.unmount();

    const reopened = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api } });
    await flushPromises();
    expect(reopened.get('[data-proxy-group-tag="主策略组"] [aria-label="选择 新加坡 01"]').attributes("aria-pressed")).toBe("true");
  });

  it("切换失败保留原节点并显示错误，不宣称成功", async () => {
    const api = createApi();
    api.select.mockRejectedValueOnce(new Error("核心拒绝切换"));
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api } });
    await flushPromises();
    const main = wrapper.get('[data-proxy-group-tag="主策略组"]');
    await main.get('[aria-label="选择 新加坡 01"]').trigger("click");
    await flushPromises();
    expect(main.get('[aria-label="选择 香港 01"]').attributes("aria-pressed")).toBe("true");
    expect(main.get('[aria-label="选择 新加坡 01"]').attributes("disabled")).toBeUndefined();
    expect(wrapper.emitted("error")).toEqual([["核心拒绝切换"]]);
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it("切换前发出的旧轮询响应不会覆盖新选择", async () => {
    const api = createApi();
    const original = await api.list();
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api } });
    await flushPromises();
    let finishRead!: (value: RuntimeProxyGroup[]) => void;
    api.list.mockImplementationOnce(() => new Promise((resolve) => { finishRead = resolve; }));
    const pending = wrapper.vm.refresh();
    const target = wrapper.get('[data-proxy-group-tag="主策略组"] [aria-label="选择 新加坡 01"]');
    await target.trigger("click");
    await flushPromises();
    finishRead(original);
    await pending;
    await flushPromises();
    expect(target.attributes("aria-pressed")).toBe("true");
  });

  it("运行配置变化后忽略旧切换响应，成员列表以核心为准", async () => {
    const api = createApi();
    let finishSwitch!: (value: ProxySelection) => void;
    api.select.mockImplementationOnce(() => new Promise((resolve) => { finishSwitch = resolve; }));
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api, runtimeKey: "old" } });
    await flushPromises();
    await wrapper.get('[data-proxy-group-tag="主策略组"] [aria-label="选择 新加坡 01"]').trigger("click");
    api.list.mockResolvedValue([{ tag: "主策略组", type: "Selector", selected: "新运行节点", members: [{ tag: "新运行节点", type: "Trojan" }] }]);
    await wrapper.setProps({ runtimeKey: "new" });
    await flushPromises();
    finishSwitch({ tag: "主策略组", selected: "新加坡 01" });
    await flushPromises();
    expect(wrapper.findAll(".proxy-node-select")).toHaveLength(1);
    expect(wrapper.get('[aria-label="选择 新运行节点"]').attributes("aria-pressed")).toBe("true");
    expect(wrapper.emitted("notice")).toBeUndefined();
  });

  it("URLTest 只显示核心选择；核心停止或读取失败时不能切换", async () => {
    const api = createApi();
    api.list.mockResolvedValue([{ tag: "自动选择", type: "URLTest", selected: "新加坡 01", members: [{ tag: "香港 01", type: "AnyTLS" }, { tag: "新加坡 01", type: "AnyTLS" }] }]);
    const wrapper = mount(DashboardProxySelector, { props: { outbounds: testOutbounds, api } });
    await flushPromises();
    expect(wrapper.get('[aria-label="选择 新加坡 01"]').attributes("aria-pressed")).toBe("true");
    for (const button of wrapper.findAll(".proxy-node-select")) expect(button.attributes("disabled")).toBeDefined();
    await wrapper.get('[aria-label="选择 香港 01"]').trigger("click");
    expect(api.select).not.toHaveBeenCalled();
    await wrapper.setProps({ running: false });
    expect(wrapper.text()).toContain("启动核心后可切换节点");
    expect(wrapper.findAll('.proxy-node-select[aria-pressed="true"]')).toHaveLength(0);
    const calls = api.list.mock.calls.length;
    await wrapper.vm.refresh();
    expect(api.list).toHaveBeenCalledTimes(calls);
    api.list.mockRejectedValueOnce(new Error("控制 API 不可用"));
    await wrapper.setProps({ running: true });
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("控制 API 不可用");
    for (const button of wrapper.findAll(".proxy-node-select")) expect(button.attributes("disabled")).toBeDefined();
  });
});
