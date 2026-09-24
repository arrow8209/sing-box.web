import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import InboundConfigPanel from "@/features/config-editor/InboundConfigPanel.vue";
import InboundEditorDialog from "@/features/inbounds/InboundEditorDialog.vue";
import InboundImportDialog from "@/features/inbounds/InboundImportDialog.vue";
import { serializeInboundInput } from "@/features/config-editor/inbound-input";
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
  it("添加独立入站时保留原始大整数 JSON token", () => {
    const saved = draft([{ type: "mixed", tag: "original", future: 9007199254740993 }]);
    saved.buffers.inbounds = '[{"type":"mixed","tag":"original","future":9007199254740993}]';
    const text = serializeInboundInput(saved, [saved.config.inbounds instanceof Array ? saved.config.inbounds[0] as { type: string; tag: string } : { type: "mixed", tag: "original" }, { type: "http", tag: "new" }]);
    expect(text).toContain("9007199254740993");
    expect(text).toContain('"tag":"new"');
  });
  it("新建 Mixed 使用服务端默认端口，编辑既有入站保留原端口", async () => {
    const wrapper = mount(InboundEditorDialog, {
      props: { open: true, resource: null, existingTags: [], defaultMixedPort: 50019 }
    });
    expect((wrapper.get('input[type="number"]').element as HTMLInputElement).value).toBe("50019");
    await wrapper.get('input[type="radio"][value="http"]').setValue(true);
    await wrapper.get('input[type="radio"][value="mixed"]').setValue(true);
    await wrapper.get("form").trigger("submit");
    expect(wrapper.emitted("submit")?.[0]?.[0]).toMatchObject({ config: { type: "mixed", listen_port: 50019 } });
    await wrapper.setProps({
      resource: { id: "existing", mode: "structured", config: { type: "mixed", tag: "existing", listen: "127.0.0.1", listen_port: 19090 }, updatedAt: "2026-09-11T00:00:00Z" }
    });
    expect((wrapper.get('input[type="number"]').element as HTMLInputElement).value).toBe("19090");
    wrapper.unmount();
  });

  it("校验单个完整对象、保留未知字段并生成唯一 tag", () => {
    const result = validateInboundJson(`{"type":" naive ","tag":" custom-in ","future":{"large":900},"token":"keep"}`);
    expect(result.valid).toBe(true);
    if (!result.valid) return;
    expect(result.config).toEqual({ type: "naive", tag: "custom-in", future: { large: 900 }, token: "keep" });
    expect(validateInboundJson(`[{"type":"mixed","tag":"x"}]`).valid).toBe(false);
    expect(validateInboundJson(`{"type":"mixed","tag":"x"}`, ["x"])).toEqual({ valid: false, message: "入站名称已存在：x" });
    expect(uniqueInboundTag("mixed-in", ["mixed-in", "mixed-in-2"])).toBe("mixed-in-3");
  });

  it("结构化编辑器不显示或生成已移除的旧嗅探参数", async () => {
    const wrapper = mount(InboundEditorDialog, {
      props: { open: true, resource: null, existingTags: [] }
    });
    expect(wrapper.findAll(".type-grid > label")).toHaveLength(6);
    await wrapper.get('input[type="radio"][value="http"]').setValue(true);
    expect(wrapper.text()).not.toContain("协议嗅探");
    expect(wrapper.text()).not.toContain("覆盖目标地址");
    await wrapper.get('input[name="tag"]').setValue("http-lan");
    await wrapper.get('input[required]:not([name="tag"])').setValue("0.0.0.0");
    await wrapper.get('input[type="number"]').setValue("18080");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as { config: Record<string, unknown>; routing?: unknown };
    expect(submitted).toMatchObject({
      mode: "structured",
      config: { type: "http", tag: "http-lan", listen: "0.0.0.0", listen_port: 18080 }
    });
    expect(submitted).not.toHaveProperty("routing");
    expect(submitted.config).not.toHaveProperty("sniff");
    expect(submitted.config).not.toHaveProperty("sniff_override_destination");
  });

  it("档案结构化编辑移除旧嗅探字段，完整 JSON 则原样提交", async () => {
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
    expect(structured.text()).not.toContain("协议嗅探");
    expect(structured.text()).not.toContain("覆盖目标地址");
    await structured.get("form").trigger("submit");
    const migrated = structured.emitted("submit")?.[0]?.[0] as { config: Record<string, unknown>; routing?: unknown };
    expect(migrated).not.toHaveProperty("routing");
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

  it("导入入站保留来源关联，重复 tag 自动改名且来源 ID 不进入运行 JSON", async () => {
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
    const payload = wrapper.emitted("save")?.[0]?.[0] as { text: string; references: { inbounds: Array<{ tag: string; resourceId: string; sourceTag: string; position: number }> }; routeText?: string };
    const saved = JSON.parse(payload.text) as Array<Record<string, unknown>>;
    expect(saved).toHaveLength(1);
    expect(saved[0]?.tag).toBe("mixed-in");
    expect(payload.references.inbounds).toEqual([{ tag: "mixed-in-2", resourceId: id, sourceTag: "mixed-in", position: 1 }]);
    expect(payload.routeText).toBeUndefined();
    expect(JSON.stringify(saved)).not.toContain(id);
    expect(JSON.stringify(saved)).not.toContain("sourceId");
    source.listen_port = 19091;
    expect(payload.references.inbounds[0]?.resourceId).toBe(id);
  });

  it("旧入站可明确关联源资源并保留本地名称，解除关联保留已同步内容", async () => {
    const id = "e".repeat(32);
    const source = { type: "http", tag: "shared-http", listen: "127.0.0.1", listen_port: 19100 };
    const summary = { id, mode: "structured", type: "http", tag: source.tag, endpoint: "127.0.0.1:19100", fieldCount: 4, updatedAt: "2026-09-23T00:00:00Z" };
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => new Response(JSON.stringify(String(input).endsWith(`/inbound-resources/${id}`)
      ? { ...summary, config: source } : [summary]), { headers: { "Content-Type": "application/json" } })));
    const original = draft([{ type: "mixed", tag: "local-in", listen_port: 19090 }]);
    const wrapper = mount(InboundConfigPanel, { props: { draft: original, profileName: "测试配置" } });
    await wrapper.get('[aria-label="关联源资源 local-in"]').trigger("click");
    await flushPromises();
    const dialog = wrapper.findComponent(InboundImportDialog);
    expect(dialog.props("targetTag")).toBe("local-in");
    expect(dialog.find('input[type="radio"]').exists()).toBe(true);
    dialog.vm.$emit("import", [id]);
    await flushPromises();
    const linked = wrapper.emitted("save")![0]![0] as { text: string; references: NonNullable<Draft["references"]> };
    expect(JSON.parse(linked.text)).toEqual([]);
    expect(linked.references.inbounds).toEqual([{ tag: "local-in", resourceId: id, sourceTag: "shared-http", position: 0 }]);
    const candidate = { profileId: original.profileId, revision: original.revision, digest: "digest", resourceDigests: {}, items: [{ module: "inbounds" as const, tag: "local-in", type: "http", independent: false,
      sourceId: id, sourceName: "shared-http", text: JSON.stringify({ ...source, tag: "local-in" }), routeText: "{}", inboundStateText: '{"version":1,"managedSniffRules":[]}', token: "signed" }] };
    await wrapper.setProps({ serverResolution: true, candidate, draft: { ...original, schemaVersion: 2, references: linked.references, config: { inbounds: [] }, buffers: { inbounds: linked.text }, editorState: {} } });
    expect(wrapper.find(".entry-copy em").text()).toBe("已关联源资源");
    await wrapper.get('[aria-label="编辑 local-in"]').trigger("click");
    expect(wrapper.findComponent(InboundEditorDialog).props("open")).toBe(false);
    await wrapper.get('[aria-label="解除关联 local-in"]').trigger("click");
    expect(wrapper.emitted("freeze")?.[0]?.[0]).toMatchObject({ action: "detach", tag: "local-in", text: JSON.stringify({ ...source, tag: "local-in" }), token: "signed" });
    wrapper.unmount();
  });
});
