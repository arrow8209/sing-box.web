import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import OutboundsEditor from "./OutboundsEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

function makeDraft(outbounds: unknown[]): Draft {
  return {
    profileId,
    revision: 1,
    content: { config: { outbounds }, editorBuffers: {} },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

interface OutboundLike {
  type: string;
  tag: string;
  [key: string]: unknown;
}

describe("OutboundsEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("edits direct and block tags", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      { type: "direct", tag: "direct", future_field: 1 },
      { type: "block", tag: "block" },
    ]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastOutbounds = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return config.outbounds as OutboundLike[];
    };

    const wrapper = mount(OutboundsEditor);
    const tagInputs = wrapper.findAll('input[aria-label="出站标签"]');
    await tagInputs[0].setValue("direct-out");
    expect(lastOutbounds()[0].tag).toBe("direct-out");
    expect(lastOutbounds()[0].future_field).toBe(1);
  });

  it("edits selector with ordered outbounds references and interrupt flag", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      { type: "direct", tag: "direct" },
      { type: "block", tag: "block" },
      { type: "selector", tag: "sel", outbounds: ["direct"], interrupt_exist_connections: false },
    ]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastSelector = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.outbounds as OutboundLike[]).find((outbound) => outbound.type === "selector");
    };

    const wrapper = mount(OutboundsEditor);
    await wrapper.get('input[aria-label="选择器出站引用"]').setValue("direct,block");
    expect(lastSelector()?.outbounds).toEqual(["direct", "block"]);

    await wrapper.get('input[aria-label="中断已有连接"]').setValue(true);
    expect(lastSelector()?.interrupt_exist_connections).toBe(true);
  });

  it("edits urltest url, interval and tolerance", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      { type: "direct", tag: "direct" },
      { type: "urltest", tag: "ut", outbounds: ["direct"], url: "https://a/", interval: "3m", tolerance: 150 },
    ]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastUrltest = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.outbounds as OutboundLike[]).find((outbound) => outbound.type === "urltest");
    };

    const wrapper = mount(OutboundsEditor);
    await wrapper.get('input[aria-label="测试 URL"]').setValue("https://example.test/gen_204");
    expect(lastUrltest()?.url).toBe("https://example.test/gen_204");

    await wrapper.get('input[aria-label="测试间隔"]').setValue("5m");
    expect(lastUrltest()?.interval).toBe("5m");

    await wrapper.get('input[aria-label="容忍度"]').setValue("200");
    expect(lastUrltest()?.tolerance).toBe(200);
  });

  it("accepts custom full outbound JSON for unsupported protocols", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(OutboundsEditor);
    await wrapper.get('button[name="添加出站"]').trigger("click");
    await wrapper.get('button[name="编辑 JSON"]').trigger("click");
    await wrapper
      .get("textarea")
      .setValue('{"type":"hysteria2","tag":"hy2","server":"example.test","server_port":443,"password":"p","extra":{"x":1}}');

    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    const outbound = (config.outbounds as OutboundLike[])[0];
    expect(outbound.type).toBe("hysteria2");
    expect(outbound.extra).toEqual({ x: 1 });
  });

  it("shows missing references as errors but never removes them", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      { type: "selector", tag: "sel", outbounds: ["missing-out"] },
    ]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(OutboundsEditor);
    expect(wrapper.text()).toContain("缺失引用");

    const config = draft.draft.content.config as Record<string, unknown>;
    const outbound = (config.outbounds as OutboundLike[])[0];
    expect(outbound.outbounds).toEqual(["missing-out"]);
    expect(replaceConfig).not.toHaveBeenCalled();
  });

  it("highlights duplicate tags via client-side preview", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      { type: "direct", tag: "dup" },
      { type: "block", tag: "dup" },
    ]);
    draft.status = "saved";

    const wrapper = mount(OutboundsEditor);
    expect(wrapper.text()).toContain("标签重复");
  });
});
