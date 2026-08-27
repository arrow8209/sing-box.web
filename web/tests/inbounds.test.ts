import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import InboundConfigPanel from "@/features/config-editor/InboundConfigPanel.vue";
import InboundEditorDialog from "@/features/inbounds/InboundEditorDialog.vue";
import InboundImportDialog from "@/features/inbounds/InboundImportDialog.vue";
import { applyInboundRouteIntents } from "@/features/config-editor/inbound";
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

  it("结构化编辑器提交常用 HTTP 入站字段且不再生成旧嗅探字段", async () => {
    const wrapper = mount(InboundEditorDialog, {
      props: { open: true, resource: null, existingTags: [] }
    });
    expect(wrapper.findAll(".type-grid > label")).toHaveLength(6);
    await wrapper.get('input[type="radio"][value="http"]').setValue(true);
    await wrapper.get('input[name="tag"]').setValue("http-lan");
    await wrapper.get('input[required]:not([name="tag"])').setValue("0.0.0.0");
    await wrapper.get('input[type="number"]').setValue("18080");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as { config: Record<string, unknown> };
    expect(submitted).toMatchObject({
      mode: "structured",
      config: { type: "http", tag: "http-lan", listen: "0.0.0.0", listen_port: 18080 }
    });
    expect(submitted.config).not.toHaveProperty("sniff");
    expect(submitted.config).not.toHaveProperty("sniff_override_destination");
  });

  it("档案结构化编辑按官方规则迁移嗅探，完整 JSON 则原样提交", async () => {
    const legacy = {
      type: "mixed", tag: "legacy-in", listen: "127.0.0.1", listen_port: 7891,
      sniff: true, sniff_override_destination: true, sniff_timeout: "1s",
      domain_strategy: "prefer_ipv4", udp_disable_domain_unmapping: true
    };
    const structured = mount(InboundEditorDialog, {
      props: {
        open: true, existingTags: [], kind: "profile",
        resource: { id: "profile-inbound-0", mode: "structured", config: legacy, updatedAt: "2026-08-25T00:00:00Z" }
      }
    });
    await structured.get("form").trigger("submit");
    const migrated = structured.emitted("submit")?.[0]?.[0] as { config: Record<string, unknown>; routing: Record<string, unknown> };
    expect(migrated.routing).toEqual({ sniff: true, sniffTimeout: "1s" });
    for (const field of ["sniff", "sniff_override_destination", "sniff_timeout", "domain_strategy", "udp_disable_domain_unmapping"]) {
      expect(migrated.config).not.toHaveProperty(field);
    }

    const custom = mount(InboundEditorDialog, { props: { open: true, resource: null, existingTags: [] } });
    await custom.get('input[type="radio"][value="custom"]').setValue(true);
    const source = JSON.stringify(legacy, null, 2);
    await custom.get('textarea[aria-label="入站 JSON"]').setValue(source);
    await custom.get("form").trigger("submit");
    expect(custom.emitted("submit")?.[0]?.[0]).toEqual({ mode: "custom", config: legacy });
  });

  it("只增删结构化编辑器管理的 sniff 路由规则", () => {
    const initial = draft([{ type: "mixed", tag: "mixed-in" }]);
    const manual = { inbound: "manual-in", action: "sniff", timeout: "2s" };
    const terminal = { action: "route", outbound: "direct" };
    initial.config.route = { rules: [manual, terminal] };
    initial.buffers.route = `{"future":9007199254740993,"rules":[${JSON.stringify(manual)},${JSON.stringify(terminal)}]}`;
    initial.editorState = { route: { version: 1, insertionPosition: 1, disabledRules: [] } };

    const enabled = applyInboundRouteIntents(initial, [{ tag: "mixed-in", sniff: true }]);
    expect(enabled.routeText).toBeDefined();
    expect((JSON.parse(enabled.routeText!) as { rules: unknown[] }).rules).toEqual([
      { inbound: "mixed-in", action: "sniff", timeout: "300ms" }, manual, terminal
    ]);
    expect(enabled.routeText).toContain("9007199254740993");
    expect(enabled.editorState.managedSniffRules).toHaveLength(1);
    expect(enabled.routeEditorState?.insertionPosition).toBe(2);

    const withManaged: Draft = {
      ...initial,
      config: { ...initial.config, route: JSON.parse(enabled.routeText!) },
      buffers: { ...initial.buffers, route: enabled.routeText! },
      editorState: { ...initial.editorState, inbounds: enabled.editorState, route: enabled.routeEditorState }
    };
    const disabled = applyInboundRouteIntents(withManaged, [{ previousTag: "mixed-in", tag: "mixed-in", sniff: false }]);
    expect((JSON.parse(disabled.routeText!) as { rules: unknown[] }).rules).toEqual([manual, terminal]);
    expect(disabled.routeText).toContain("9007199254740993");
    expect(disabled.editorState.managedSniffRules).toEqual([]);

    const manualOnly = applyInboundRouteIntents(initial, [{ previousTag: "manual-in", tag: "manual-in", sniff: false }]);
    expect(manualOnly.routeText).toBeUndefined();
    expect(initial.config.route).toEqual({ rules: [manual, terminal] });
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
    const source = {
      type: "mixed", tag: "mixed-in", listen: "0.0.0.0", listen_port: 19090,
      sniff: true, sniff_override_destination: true, sniff_timeout: "750ms", future: { keep: true }
    };
    const summary = { id, mode: "structured", type: "mixed", tag: "mixed-in", endpoint: "0.0.0.0:19090", fieldCount: 9, updatedAt: "2026-08-25T02:00:00Z" };
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
    const payload = wrapper.emitted("save")?.[0]?.[0] as { text: string; routeText?: string; editorState: Record<string, unknown> };
    const saved = JSON.parse(payload.text) as Array<Record<string, unknown>>;
    expect(saved).toHaveLength(2);
    expect(saved[1]).toEqual({
      type: "mixed", tag: "mixed-in-2", listen: "0.0.0.0", listen_port: 19090, future: { keep: true }
    });
    expect((JSON.parse(payload.routeText!) as { rules: unknown[] }).rules[0]).toEqual({
      inbound: "mixed-in-2", action: "sniff", timeout: "750ms"
    });
    expect(payload.editorState).toMatchObject({ version: 1, managedSniffRules: [{ tag: "mixed-in-2" }] });
    expect(JSON.stringify(saved)).not.toContain(id);
    expect(JSON.stringify(saved)).not.toContain("sourceId");
    source.listen_port = 19091;
    expect(saved[1]?.listen_port).toBe(19090);
  });
});
