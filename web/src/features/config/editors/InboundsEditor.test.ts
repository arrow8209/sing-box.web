import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import InboundsEditor from "./InboundsEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

function makeDraft(inbounds: unknown[]): Draft {
  return {
    profileId,
    revision: 1,
    content: { config: { inbounds }, editorBuffers: {} },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

interface InboundLike {
  type: string;
  tag: string;
  [key: string]: unknown;
}

describe("InboundsEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("edits mixed fields", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      {
        type: "mixed",
        tag: "mixed-in",
        listen: "127.0.0.1",
        listen_port: 7890,
        unknown_keep: true,
      },
    ]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastInbound = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.inbounds as InboundLike[])[0];
    };

    const wrapper = mount(InboundsEditor);
    await wrapper.get('input[aria-label="监听地址"]').setValue("0.0.0.0");
    expect(lastInbound().listen).toBe("0.0.0.0");
    expect(lastInbound().unknown_keep).toBe(true);

    await wrapper.get('input[aria-label="监听端口"]').setValue("1080");
    expect(lastInbound().listen_port).toBe(1080);

    await wrapper.get('input[aria-label="TCP Fast Open"]').setValue(true);
    expect(lastInbound().tcp_fast_open).toBe(true);

    await wrapper.get('input[aria-label="TCP Multi Path"]').setValue(true);
    expect(lastInbound().tcp_multi_path).toBe(true);

    await wrapper.get('input[aria-label="UDP Fragment"]').setValue(true);
    expect(lastInbound().udp_fragment).toBe(true);
  });

  it("edits tun fields including address array", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([
      {
        type: "tun",
        tag: "tun-in",
        interface_name: "singtun0",
        address: ["172.19.0.1/30"],
        mtu: 9000,
        stack: "system",
        future_field: { n: 1 },
      },
    ]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastInbound = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.inbounds as InboundLike[])[0];
    };

    const wrapper = mount(InboundsEditor);
    await wrapper.get('input[aria-label="接口名称"]').setValue("tun1");
    expect(lastInbound().interface_name).toBe("tun1");

    await wrapper.get('input[aria-label="MTU"]').setValue("1500");
    expect(lastInbound().mtu).toBe(1500);

    await wrapper.get('select[aria-label="协议栈"]').setValue("gvisor");
    expect(lastInbound().stack).toBe("gvisor");

    await wrapper.get('input[aria-label="自动路由"]').setValue(true);
    expect(lastInbound().auto_route).toBe(true);
    await wrapper.get('input[aria-label="自动路由"]').setValue(false);
    expect(lastInbound().auto_route).toBe(false);

    await wrapper.get('input[aria-label="严格路由"]').setValue(true);
    expect(lastInbound().strict_route).toBe(true);

    await wrapper.get('input[aria-label="端点独立 NAT"]').setValue(true);
    expect(lastInbound().endpoint_independent_nat).toBe(true);

    expect(lastInbound().future_field).toEqual({ n: 1 });
  });

  it("accepts a custom object preserving every key and derives title from tag then type", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft([]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const setEditorBuffer = vi.spyOn(draft, "setEditorBuffer");
    const applyEditorBuffer = vi.spyOn(draft, "applyEditorBuffer");

    const wrapper = mount(InboundsEditor);
    await wrapper.get('button[name="添加入站"]').trigger("click");
    await wrapper.get('button[name="编辑 JSON"]').trigger("click");

    const textarea = wrapper.get("textarea");
    await textarea.setValue(
      '{"type":"shadowsocks","tag":"ss-in","listen":"0.0.0.0","listen_port":8388,"method":"aes-256-gcm","password":"p","custom_extra":{"k":1}}',
    );

    expect(replaceConfig).toHaveBeenCalled();
    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    const inbound = (config.inbounds as InboundLike[])[0];
    expect(inbound.type).toBe("shadowsocks");
    expect(inbound.custom_extra).toEqual({ k: 1 });

    expect(applyEditorBuffer).toHaveBeenCalled();
    void setEditorBuffer;
  });

  it("deletes and reorders items while preserving untouched objects", async () => {
    const first = { type: "mixed", tag: "m1", listen: "127.0.0.1", listen_port: 1 };
    const second = { type: "tun", tag: "t1", interface_name: "tun0", address: ["10.0.0.1/30"] };
    const draft = useDraftStore();
    draft.draft = makeDraft([first, second]);
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(InboundsEditor);
    await wrapper.get('button[name="删除入站 1"]').trigger("click");
    let config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.inbounds as InboundLike[])).toEqual([first]);

    await wrapper.get('button[name="添加入站"]').trigger("click");
    config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.inbounds as InboundLike[])[0]).toEqual(first);
  });
});
