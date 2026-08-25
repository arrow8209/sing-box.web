import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import InboundConfigPanel from "@/features/config-editor/InboundConfigPanel.vue";
import InboundEditorDialog from "@/features/inbounds/InboundEditorDialog.vue";
import InboundImportDialog from "@/features/inbounds/InboundImportDialog.vue";
import { uniqueInboundTag, validateInboundJson } from "@/features/inbounds/model";
import { useInboundStore } from "@/features/inbounds/store";
import type { Draft } from "@/features/profiles/types";

function draft(inbounds: unknown[]): Draft {
  return {
    profileId: "a".repeat(32), revision: 1, config: { inbounds },
    buffers: { inbounds: JSON.stringify(inbounds, null, 2) }, bindings: [], savedAt: "2026-08-25T00:00:00Z"
  };
}

describe("入站配置", () => {
  it("校验单个完整对象、保留未知字段并生成唯一 tag", () => {
    const result = validateInboundJson(`{"type":" naive ","tag":" custom-in ","future":{"large":900},"token":"keep"}`);
    expect(result.valid).toBe(true);
    if (!result.valid) return;
    expect(result.config).toEqual({ type: "naive", tag: "custom-in", future: { large: 900 }, token: "keep" });
    expect(validateInboundJson(`[{"type":"mixed","tag":"x"}]`).valid).toBe(false);
    expect(validateInboundJson(`{"type":"mixed","tag":"x"}`, ["x"])).toEqual({ valid: false, message: "入站名称已存在：x" });
    expect(uniqueInboundTag("mixed-in", ["mixed-in", "mixed-in-2"])).toBe("mixed-in-3");
  });

  it("结构化编辑器提交常用 HTTP 入站字段", async () => {
    const wrapper = mount(InboundEditorDialog, {
      props: { open: true, resource: null, existingTags: [] }
    });
    expect(wrapper.findAll(".type-grid > label")).toHaveLength(6);
    await wrapper.get('input[type="radio"][value="http"]').setValue(true);
    await wrapper.get('input[name="tag"]').setValue("http-lan");
    await wrapper.get('input[required]:not([name="tag"])').setValue("0.0.0.0");
    await wrapper.get('input[type="number"]').setValue("18080");
    await wrapper.get("form").trigger("submit");
    expect(wrapper.emitted("submit")?.[0]?.[0]).toMatchObject({
      mode: "structured",
      config: { type: "http", tag: "http-lan", listen: "0.0.0.0", listen_port: 18080, sniff: true }
    });
  });

  it("结构化编辑器生成 TUN 地址、路由与开关", async () => {
    const wrapper = mount(InboundEditorDialog, { props: { open: true, resource: null, existingTags: [] } });
    await wrapper.get('input[type="radio"][value="tun"]').setValue(true);
    await wrapper.get('input[name="tag"]').setValue("tun-system");
    await wrapper.get('input[aria-label="TUN 地址 1"]').setValue("172.20.0.1/30");
    await wrapper.get("form").trigger("submit");
    expect(wrapper.emitted("submit")?.[0]?.[0]).toMatchObject({
      mode: "structured",
      config: { type: "tun", tag: "tun-system", address: ["172.20.0.1/30", "fdfe:dcba:9876::1/126"], mtu: 9000, stack: "mixed", auto_route: true, strict_route: true }
    });
  });

  it("从资源库导入深拷贝，重复 tag 自动改名且草稿不保存来源 ID", async () => {
    const id = "b".repeat(32);
    const source = { type: "mixed", tag: "mixed-in", listen: "0.0.0.0", listen_port: 19090, future: { keep: true } };
    const summary = { id, mode: "structured", type: "mixed", tag: "mixed-in", endpoint: "0.0.0.0:19090", fieldCount: 6, updatedAt: "2026-08-25T02:00:00Z" };
    useInboundStore().resources.value = [];
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input);
      const payload = path.endsWith(`/inbound-resources/${id}`)
        ? { id, mode: "structured", config: source, updatedAt: summary.updatedAt }
        : [summary];
      return new Response(JSON.stringify(payload), { status: 200, headers: { "Content-Type": "application/json" } });
    });
    vi.stubGlobal("fetch", fetchMock);
    const wrapper = mount(InboundConfigPanel, {
      props: { draft: draft([{ type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 7891 }]), profileName: "测试配置" },
      global: { stubs: { RouterLink: { template: "<a><slot /></a>" } } }
    });
    await wrapper.get(".import-button").trigger("click");
    await flushPromises();
    wrapper.findComponent(InboundImportDialog).vm.$emit("import", [id]);
    await flushPromises();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(wrapper.emitted("save")).toBeTruthy();
    const text = wrapper.emitted("save")?.[0]?.[0] as string;
    const saved = JSON.parse(text) as Array<Record<string, unknown>>;
    expect(saved).toHaveLength(2);
    expect(saved[1]).toEqual({ ...source, tag: "mixed-in-2" });
    expect(JSON.stringify(saved)).not.toContain(id);
    expect(JSON.stringify(saved)).not.toContain("sourceId");
    source.listen_port = 19091;
    expect(saved[1]?.listen_port).toBe(19090);
  });
});
