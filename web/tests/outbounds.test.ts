import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import OutboundConfigPanel from "@/features/config-editor/OutboundConfigPanel.vue";
import OutboundEditorDialog from "@/features/config-editor/OutboundEditorDialog.vue";
import {
  applyOutboundEditorState,
  outboundEditorGroups,
  renameOutboundTagInBuffer,
  removeOutboundGroup,
  restoreOutboundEditorState,
  updateOutboundGroup,
  type OutboundEditorGroup,
  type OutboundEditorResult
} from "@/features/config-editor/outbound";
import type { Draft } from "@/features/profiles/types";
import { subscriptionApi } from "@/features/subscriptions/api";
import type { SubscriptionDetail } from "@/features/subscriptions/types";

const subscription: SubscriptionDetail = {
  id: "a".repeat(32),
  name: "家庭节点",
  outbounds: [
    { type: "trojan", tag: "新加坡 01", server: "sg.example.test", password: "not-visible-in-dom" },
    { type: "vless", tag: "香港 01", server: "hk.example.test", uuid: "00000000-0000-0000-0000-000000000000" }
  ],
  updatedAt: "2026-08-25T00:00:00Z"
};

function selectorGroup(overrides: Partial<OutboundEditorGroup> = {}): OutboundEditorGroup {
  return {
    id: "group-select",
    tag: "节点选择",
    type: "selector",
    hidden: false,
    include: "",
    exclude: "",
    icon: "",
    references: [{ kind: "builtin", tag: "direct", resolvedTags: ["direct"] }],
    runtimeTags: ["direct"],
    config: { type: "selector", tag: "节点选择", interrupt_exist_connections: true, outbounds: ["direct"] },
    ...overrides
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("自定义配置出站", () => {
  it("从运行 JSON 恢复源项目模型，并隐藏内置和订阅展开节点", () => {
    const source = [
      { type: "selector", tag: "节点选择", outbounds: ["direct", "新加坡 01", "香港 01"] },
      { type: "trojan", tag: "新加坡 01", server: "sg.example.test", password: "not-visible-in-dom" },
      { type: "vless", tag: "香港 01", server: "hk.example.test", uuid: "00000000-0000-0000-0000-000000000000" },
      { type: "direct", tag: "direct" },
      { type: "block", tag: "block" }
    ];

    const state = restoreOutboundEditorState(source, [subscription]);
    const groups = outboundEditorGroups(source, state);

    expect(groups.map((group) => group.tag)).toEqual(["节点选择"]);
    expect(state.generatedTags).toEqual(expect.arrayContaining(["新加坡 01", "香港 01", "direct", "block"]));
    expect(groups[0]?.references).toEqual([
      { kind: "builtin", tag: "direct", resolvedTags: ["direct"] },
      { kind: "subscription", subscriptionId: subscription.id, tag: subscription.name, resolvedTags: ["新加坡 01", "香港 01"] }
    ]);
  });

  it("整个订阅保持为一个选择项，并按包含和排除条件展开运行节点", () => {
    const source = [
      { type: "selector", tag: "节点选择", hidden: true, include: "新加坡|香港", icon: "https://old.test/icon.png", outbounds: ["新加坡 01", "香港 01"] },
      subscription.outbounds[0],
      subscription.outbounds[1]
    ];
    const text = JSON.stringify(source, null, 2);
    const state = restoreOutboundEditorState(source, [subscription]);
    const group = outboundEditorGroups(source, state)[0]!;
    group.include = "香港";
    group.exclude = "新加坡";
    const result = applyOutboundEditorState(text, source, state, [group], [subscription]);
    const saved = JSON.parse(result.text) as Array<Record<string, unknown>>;
    const selector = saved.find((item) => item.tag === "节点选择");

    expect(result.state.groups[0]?.references).toHaveLength(1);
    expect(result.state.groups[0]?.references[0]).toMatchObject({ kind: "subscription", subscriptionId: subscription.id, resolvedTags: ["香港 01"] });
    expect(selector?.outbounds).toEqual(["香港 01"]);
    expect(selector).not.toHaveProperty("hidden");
    expect(selector).not.toHaveProperty("include");
    expect(selector).not.toHaveProperty("exclude");
    expect(selector).not.toHaveProperty("icon");
    expect(saved.find((item) => item.tag === "香港 01")).toMatchObject({ uuid: expect.any(String) });
    expect(saved.some((item) => item.tag === "新加坡 01")).toBe(false);
    expect(result.state.groups[0]).toMatchObject({ hidden: true, include: "香港", icon: "https://old.test/icon.png" });
  });

  it("重命名出站会按稳定 ID 更新其他策略组引用并保留未知大整数 token", () => {
    const text = `[
  {"type":"selector","tag":"节点选择","outbounds":["direct"],"future":{"large":9007199254740993}},
  {"type":"selector","tag":"回退","outbounds":["节点选择"],"future":{"large":9007199254740995}},
  {"type":"future","tag":"手工节点","large":9007199254740997},
  {"type":"direct","tag":"direct"}
]`;
    const source = JSON.parse(text) as unknown[];
    const state = restoreOutboundEditorState(source, []);
    const groups = outboundEditorGroups(source, state);
    const renamed = { ...groups[0]!, tag: "新节点选择", config: { ...groups[0]!.config, tag: "新节点选择" } };
    const result = applyOutboundEditorState(text, source, state, updateOutboundGroup(groups, renamed), []);
    const saved = JSON.parse(result.text) as Array<Record<string, unknown>>;

    expect(saved.find((item) => item.tag === "回退")?.outbounds).toEqual(["新节点选择"]);
    expect(result.state.groups.find((group) => group.tag === "回退")?.references[0]).toMatchObject({ kind: "outbound", tag: "新节点选择" });
    expect(result.text).toContain("9007199254740993");
    expect(result.text).toContain("9007199254740995");
    expect(result.text).toContain("9007199254740997");
  });

  it("重命名同步更新路由、入站和 detour 引用且不重写未知 token", () => {
    const route = `{"final":"节点选择","rules":[{"action":"route","outbound":"节点选择","future":9007199254740993}]}`;
    const inbounds = `[{"type":"mixed","tag":"mixed-in","detour":"节点选择","future":9007199254740995}]`;
    const outbounds = `[{"type":"future","tag":"手工节点","detour":"节点选择","future":9007199254740997}]`;

    const nextRoute = renameOutboundTagInBuffer(route, "route", "节点选择", "新节点选择");
    const nextInbounds = renameOutboundTagInBuffer(inbounds, "inbounds", "节点选择", "新节点选择");
    const nextOutbounds = renameOutboundTagInBuffer(outbounds, "outbounds", "节点选择", "新节点选择");

    expect(nextRoute).toContain('"final":"新节点选择"');
    expect(nextRoute).toContain('"outbound":"新节点选择"');
    expect(nextRoute).toContain("9007199254740993");
    expect(nextInbounds).toContain('"detour":"新节点选择"');
    expect(nextInbounds).toContain("9007199254740995");
    expect(nextOutbounds).toContain('"detour":"新节点选择"');
    expect(nextOutbounds).toContain("9007199254740997");
  });

  it("删除被引用出站时保留原 tag 并转换为失效引用", () => {
    const selected = selectorGroup();
    const fallback = selectorGroup({
      id: "group-fallback",
      tag: "回退",
      references: [{ kind: "outbound", outboundId: selected.id, tag: selected.tag, resolvedTags: [selected.tag] }],
      runtimeTags: [selected.tag],
      config: { type: "selector", tag: "回退", outbounds: [selected.tag] }
    });

    const groups = removeOutboundGroup([selected, fallback], selected.id);

    expect(groups).toHaveLength(1);
    expect(groups[0]?.references).toEqual([{ kind: "missing", tag: "节点选择", resolvedTags: ["节点选择"] }]);
  });

  it("切换为直连或阻断时保留编辑选择但不展开策略引用", () => {
    const source = [
      { type: "selector", tag: "节点选择", outbounds: ["新加坡 01", "香港 01"] },
      subscription.outbounds[0],
      subscription.outbounds[1]
    ];
    const state = restoreOutboundEditorState(source, [subscription]);
    const group = outboundEditorGroups(source, state)[0]!;
    group.type = "direct";
    group.config.type = "direct";

    const result = applyOutboundEditorState(JSON.stringify(source), source, state, [group], [subscription]);
    const saved = JSON.parse(result.text) as Array<Record<string, unknown>>;

    expect(saved).toEqual([{ type: "direct", tag: "节点选择" }]);
    expect(result.state.groups[0]?.references).toHaveLength(1);
    expect(result.state.groups[0]?.runtimeTags).toEqual([]);
  });

  it("新增/编辑对话框只提供源项目四种类型，整订阅不会拆成多个勾选项", async () => {
    const wrapper = mount(OutboundEditorDialog, {
      props: { open: true, group: null, groups: [], subscriptions: [subscription] }
    });

    expect(wrapper.findAll('.type-control input[type="radio"]')).toHaveLength(4);
    expect(wrapper.text()).not.toContain("完整 JSON");
    expect(wrapper.text()).not.toContain("not-visible-in-dom");
    const subscriptionGroup = wrapper.findAll(".reference-group").find((item) => item.get(".reference-head").text().includes("订阅"));
    expect(subscriptionGroup).toBeTruthy();
    await subscriptionGroup!.get('input[type="checkbox"]').setValue(true);
    await wrapper.get('input[name="tag"]').setValue("订阅选择");
    expect(wrapper.text()).toContain("已选 1 项 · 2 个出站");
    await wrapper.get("form").trigger("submit");

    const submitted = wrapper.emitted("submit")?.[0]?.[0] as OutboundEditorResult;
    expect(submitted.group.references).toEqual([{ kind: "subscription", subscriptionId: subscription.id, tag: subscription.name }]);
  });

  it("编辑时从编辑器状态回填整个订阅、隐藏和筛选字段", async () => {
    const group = selectorGroup({
      hidden: true,
      include: "香港|新加坡",
      references: [{ kind: "subscription", subscriptionId: subscription.id, tag: subscription.name, resolvedTags: ["新加坡 01", "香港 01"] }],
      runtimeTags: ["新加坡 01", "香港 01"],
      config: { type: "selector", tag: "节点选择", outbounds: ["新加坡 01", "香港 01"] }
    });
    const wrapper = mount(OutboundEditorDialog, {
      props: { open: true, group, groups: [group], subscriptions: [subscription] }
    });

    expect((wrapper.get('input[name="hidden"]').element as HTMLInputElement).checked).toBe(true);
    expect((wrapper.get('input[name="include"]').element as HTMLInputElement).value).toBe("香港|新加坡");
    expect(wrapper.text()).toContain("已选 1 项 · 2 个出站");
    expect(wrapper.findAll('.reference-group input[type="checkbox"]:checked')).toHaveLength(1);

    await wrapper.get('input[name="tag"]').setValue("正在编辑的新名称");
    await wrapper.setProps({ group: { ...group, config: { ...group.config } } });
    expect((wrapper.get('input[name="tag"]').element as HTMLInputElement).value).toBe("正在编辑的新名称");
  });

  it("面板只显示策略出站卡片，保存时同时提交运行 JSON 和编辑器状态", async () => {
    vi.spyOn(subscriptionApi, "list").mockResolvedValue([{
      id: subscription.id,
      name: subscription.name,
      nodeCount: 2,
      protocols: ["trojan", "vless"],
      updatedAt: subscription.updatedAt
    }]);
    vi.spyOn(subscriptionApi, "get").mockResolvedValue(subscription);
    const outbounds = [
      { type: "selector", tag: "节点选择", outbounds: ["新加坡 01", "香港 01", "direct"] },
      subscription.outbounds[0],
      subscription.outbounds[1],
      { type: "direct", tag: "direct" }
    ];
    const draft: Draft = {
      profileId: "b".repeat(32),
      revision: 1,
      config: { outbounds },
      buffers: { outbounds: JSON.stringify(outbounds, null, 2) },
      bindings: [],
      savedAt: "2026-08-25T00:00:00Z"
    };
    const wrapper = mount(OutboundConfigPanel, { props: { draft } });
    await flushPromises();

    expect(wrapper.findAll(".outbound-card")).toHaveLength(1);
    expect(wrapper.find('[data-outbound-tag="节点选择"]').exists()).toBe(true);
    expect(wrapper.find('[data-outbound-tag="新加坡 01"]').exists()).toBe(false);
    expect(wrapper.find('[data-outbound-tag="direct"]').exists()).toBe(false);
    await wrapper.get('[aria-label="删除出站 节点选择"]').trigger("click");
    const payload = wrapper.emitted("save")?.[0]?.[0] as { text: string; editorState: { groups: unknown[] } };
    expect(JSON.parse(payload.text)).toEqual([]);
    expect(payload.editorState.groups).toEqual([]);
  });
});
