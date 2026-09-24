import { describe, expect, it } from "vitest";
import { persistOutboundInputs, renameOutboundTagInBuffer, removeOutboundGroup, updateOutboundGroup, type OutboundEditorGroup } from "@/features/config-editor/outbound";
import type { Draft } from "@/features/profiles/types";

function fixture(): { draft: Draft; group: OutboundEditorGroup } {
  const buffer = '[{"type":"selector","tag":"选择","outbounds":["old"],"future":9007199254740993},{"type":"ssh","tag":"独立","server":"example.test","future":9007199254740995}]';
  const draft: Draft = {
    profileId: "c".repeat(32), revision: 2, schemaVersion: 2,
    references: { version: 2, outbounds: [{ id: "g", tag: "选择", type: "selector", references: [{ kind: "raw", tag: "独立" }] }], inbounds: [] },
    config: { outbounds: JSON.parse(buffer) }, buffers: { outbounds: buffer }, bindings: [], savedAt: "2026-09-24T00:00:00Z"
  };
  const group: OutboundEditorGroup = { id: "g", tag: "选择", type: "selector", hidden: true, include: "", exclude: "", icon: "star",
    references: [{ kind: "raw", tag: "独立" }], runtimeTags: ["独立"],
    config: { type: "selector", tag: "选择", outbounds: ["old"], future: Number("9007199254740993") } };
  return { draft, group };
}

describe("出站表单输入", () => {
  it("保存策略组引用时不展开节点，保留独立 JSON 与未知大整数", () => {
    const { draft, group } = fixture();
    const renamed: OutboundEditorGroup = { ...group, tag: "新选择", include: "HK", config: { ...group.config, tag: "新选择" },
      references: [{ kind: "subscription", subscriptionId: "a".repeat(32), tag: "订阅" }] };
    const result = persistOutboundInputs(draft, [group], [renamed], { from: "选择", to: "新选择" });
    expect(result.text).toContain("9007199254740993");
    expect(result.text).toContain("9007199254740995");
    expect(result.text).not.toContain('"outbounds":["old"]');
    expect(result.text).not.toContain('"tag":"订阅"');
    expect(result.references.outbounds).toEqual([{ id: "g", tag: "新选择", type: "selector", include: "HK", exclude: "",
      references: [{ kind: "subscription", subscriptionId: "a".repeat(32), tag: "订阅" }] }]);
    expect(result.uiState.groups).toEqual([{ id: "g", tag: "新选择", hidden: true, icon: "star" }]);
  });

  it("策略组增删和路由改名保持表单语义", () => {
    const { draft, group } = fixture();
    const another: OutboundEditorGroup = { ...group, id: "next", tag: "备用", config: { type: "urltest", tag: "备用" }, type: "urltest" };
    const added = updateOutboundGroup([group], another);
    expect(added.map((item) => item.tag)).toEqual(["备用", "选择"]);
    expect(removeOutboundGroup(added, "next").map((item) => item.tag)).toEqual(["选择"]);
    const input = persistOutboundInputs(draft, [group], added);
    expect(JSON.parse(input.text).map((item: { tag: string }) => item.tag)).toEqual(["选择", "独立", "备用"]);
    expect(renameOutboundTagInBuffer('{"final":"选择","rules":[{"outbound":"选择"}]}', "route", "选择", "备用"))
      .toContain('"final":"备用"');
  });
});
