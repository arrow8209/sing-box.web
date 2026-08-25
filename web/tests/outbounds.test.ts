import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import OutboundEditorDialog from "@/features/config-editor/OutboundEditorDialog.vue";
import {
  applyOutboundBufferEdit,
  mergeOutboundEdit,
  validateOutboundJson,
  type OutboundConfig,
  type OutboundEditorResult
} from "@/features/config-editor/outbound";
import type { SubscriptionDetail } from "@/features/subscriptions/types";

describe("自定义配置出站", () => {
  it("校验单个完整对象、保留未知字段并拒绝重复 tag", () => {
    const result = validateOutboundJson(`{"type":" future ","tag":" proxy-a ","password":"keep","future":{"nested":[1,true]}}`);
    expect(result.valid).toBe(true);
    if (!result.valid) return;
    expect(result.config).toEqual({ type: "future", tag: "proxy-a", password: "keep", future: { nested: [1, true] } });
    expect(validateOutboundJson(`[{"type":"direct","tag":"direct"}]`).valid).toBe(false);
    expect(validateOutboundJson(`{"type":"direct","tag":"direct"}`, ["direct"])).toEqual({ valid: false, message: "出站名称已存在：direct" });
  });

  it("把订阅节点复制为独立出站，并让策略组只保存最终 tag", () => {
    const source: unknown[] = [{ type: "direct", tag: "direct", future: { keep: true } }];
    const result: OutboundEditorResult = {
      config: { type: "selector", tag: "节点选择", hidden: false, future_option: "keep", outbounds: [] },
      references: [
        { key: "profile:0", tag: "direct" },
        {
          key: "subscription-node:a:0",
          tag: "香港 01",
          config: { type: "trojan", tag: "香港 01", password: "secret", tls: { enabled: true }, future: { x: 1 } }
        }
      ]
    };
    const merged = mergeOutboundEdit(source, null, result);
    expect(merged.importedCount).toBe(1);
    expect(merged.items[0]).toMatchObject({ type: "selector", tag: "节点选择", outbounds: ["direct", "香港 01"], future_option: "keep" });
    expect(merged.items[2]).toEqual({ type: "trojan", tag: "香港 01", password: "secret", tls: { enabled: true }, future: { x: 1 } });
    expect(JSON.stringify(merged.items)).not.toContain("subscription-node");
    expect(JSON.stringify(merged.items)).not.toContain("sourceId");
  });

  it("订阅节点 tag 冲突时保留完整副本并自动改名", () => {
    const merged = mergeOutboundEdit(
      [{ type: "direct", tag: "香港 01" }],
      null,
      {
        editorType: "selector",
        config: { type: "selector", tag: "节点选择", outbounds: [] },
        references: [{ key: "subscription-node:a:0", tag: "香港 01", config: { type: "trojan", tag: "香港 01", password: "keep" } }]
      }
    );
    expect(merged.referencedTags).toEqual(["香港 01-2"]);
    expect(merged.imports).toEqual([{ type: "trojan", tag: "香港 01-2", password: "keep" }]);
  });

  it("完整 JSON 模式保留用户手工填写的策略组引用", () => {
    const merged = mergeOutboundEdit(
      [{ type: "direct", tag: "manual" }],
      null,
      {
        editorType: "custom",
        config: { type: "selector", tag: "自定义策略", outbounds: ["manual"], future: { keep: true } },
        references: []
      }
    );
    expect(merged.config).toMatchObject({ outbounds: ["manual"], future: { keep: true } });
  });

  it("结构化修改只编辑目标字段，保留未建模的大整数 token", () => {
    const text = `[
  {"type":"selector","tag":"节点选择","outbounds":["direct"],"future":{"large":9007199254740993}},
  {"type":"future","tag":"未来节点","large":9007199254740995}
]`;
    const items = [
      { type: "selector", tag: "节点选择", outbounds: ["direct"], future: { large: 9007199254740992 } },
      { type: "future", tag: "未来节点", large: 9007199254740996 }
    ];
    const edited = applyOutboundBufferEdit(text, items, 0, {
      editorType: "selector",
      config: { type: "selector", tag: "节点选择", include: "HK", outbounds: [] },
      references: [{ key: "profile:1", tag: "direct" }]
    });
    expect(edited.text).toContain(`"include": "HK"`);
    expect(edited.text).toContain("9007199254740993");
    expect(edited.text).toContain("9007199254740995");
  });

  it("结构化编辑同类型策略组时保留未知字段和当前引用", async () => {
    const outbound: OutboundConfig = {
      type: "selector",
      tag: "节点选择",
      outbounds: ["direct"],
      hidden: false,
      future_option: { nested: 900 }
    };
    const wrapper = mount(OutboundEditorDialog, {
      props: {
        open: true,
        outbound,
        editingIndex: 0,
        outbounds: [outbound, { type: "direct", tag: "direct" }]
      }
    });
    await wrapper.get('input[name="include"]').setValue("HK|SG");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as OutboundEditorResult;
    expect(submitted.config).toMatchObject({ type: "selector", tag: "节点选择", include: "HK|SG", future_option: { nested: 900 } });
    expect(submitted.references.map((reference) => reference.tag)).toEqual(["direct"]);
  });

  it("显示原型中的五种类型，订阅凭据仅在确认选择后随副本提交", async () => {
    const subscription: SubscriptionDetail = {
      id: "a".repeat(32),
      name: "家庭节点",
      outbounds: [
        { type: "trojan", tag: "新加坡 01", password: "not-visible-in-dom" },
        { type: "vless", tag: "香港 01", uuid: "00000000-0000-0000-0000-000000000000" }
      ],
      updatedAt: "2026-08-25T00:00:00Z"
    };
    const wrapper = mount(OutboundEditorDialog, {
      props: { open: true, outbound: null, editingIndex: null, outbounds: [], subscriptions: [subscription] }
    });
    expect(wrapper.findAll('.type-control input[type="radio"]')).toHaveLength(5);
    expect(wrapper.text()).not.toContain("not-visible-in-dom");
    const subscriptionGroup = wrapper.findAll(".reference-group").find((group) => group.get(".reference-head").text().includes("订阅"));
    expect(subscriptionGroup).toBeTruthy();
    await subscriptionGroup!.get('input[type="checkbox"]').setValue(true);
    await wrapper.get('input[name="tag"]').setValue("订阅选择");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as OutboundEditorResult;
    expect(submitted.references).toHaveLength(2);
    expect(submitted.references[0]?.config).toMatchObject({ tag: "新加坡 01", password: "not-visible-in-dom" });
    expect(submitted.references[1]?.config).toMatchObject({ tag: "香港 01" });
  });
});
