import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import AdvancedEditor from "./AdvancedEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

function makeDraft(experimental: Record<string, unknown>): Draft {
  return {
    profileId,
    revision: 1,
    content: { config: { experimental }, editorBuffers: {} },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

describe("AdvancedEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("edits clash_api controller, UI, detour, default mode, CORS and private-network fields", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      clash_api: {
        external_controller: "127.0.0.1:9090",
        secret: "masked-secret",
        default_mode: "rule",
      },
      future_option: { enabled: true },
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(AdvancedEditor);
    const lastConfig = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.experimental as Record<string, unknown>).clash_api as Record<string, unknown>;
    };

    await wrapper.get('input[aria-label="Clash API 外部控制器"]').setValue("127.0.0.1:9091");
    expect(lastConfig().external_controller).toBe("127.0.0.1:9091");

    await wrapper.get('input[aria-label="Clash API 默认模式"]').setValue("global");
    expect(lastConfig().default_mode).toBe("global");

    await wrapper.get('input[aria-label="Clash API 外部 UI"]').setValue("ui");
    expect(lastConfig().external_ui).toBe("ui");

    await wrapper.get('input[aria-label="Clash API 外部 UI 下载地址"]').setValue("https://example.test/ui");
    expect(lastConfig().external_ui_download_url).toBe("https://example.test/ui");

    await wrapper.get('input[aria-label="Clash API 允许 CORS"]').setValue(true);
    expect(lastConfig().cors).toBe(true);

    await wrapper.get('input[aria-label="Clash API 允许私网访问"]').setValue(true);
    expect(lastConfig().allow_lan).toBe(true);

    await wrapper.get('input[aria-label="Clash API 出站"]').setValue("proxy");
    expect(lastConfig().detour).toBe("proxy");

    const fullConfig = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((fullConfig.experimental as Record<string, unknown>).future_option).toEqual({
      enabled: true,
    });
  });

  it("masks the Clash API secret and reveals it temporarily", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({ clash_api: { secret: "masked-secret" } });
    draft.status = "saved";

    const wrapper = mount(AdvancedEditor);
    const secretInput = wrapper.get('input[aria-label="Clash API Secret"]');
    expect(secretInput.attributes("type")).toBe("password");
    expect((secretInput.element as HTMLInputElement).value).toBe("masked-secret");

    await wrapper.get('button[name="临时显示 Clash API Secret"]').trigger("click");
    expect(wrapper.get('input[aria-label="Clash API Secret"]').attributes("type")).toBe("text");
  });

  it("edits cache_file enabled, path, cache id, store fake IP and RDRC timeout", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      cache_file: {
        enabled: true,
        path: "/var/lib/sing-box/cache.db",
        cache_id: "cache-1",
        store_fakeip: true,
      },
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const lastCache = () => {
      const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
      return (config.experimental as Record<string, unknown>).cache_file as Record<string, unknown>;
    };

    const wrapper = mount(AdvancedEditor);

    await wrapper.get('input[aria-label="启用缓存文件"]').setValue(false);
    expect(lastCache().enabled).toBe(false);

    await wrapper.get('input[aria-label="缓存文件路径"]').setValue("/tmp/cache.db");
    expect(lastCache().path).toBe("/tmp/cache.db");

    await wrapper.get('input[aria-label="缓存 ID"]').setValue("cache-2");
    expect(lastCache().cache_id).toBe("cache-2");

    await wrapper.get('input[aria-label="缓存存储 FakeIP"]').setValue(false);
    expect(lastCache().store_fakeip).toBe(false);
  });

  it("deletes an emptied optional secret field", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({ clash_api: { secret: "masked-secret" } });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(AdvancedEditor);
    await wrapper.get('input[aria-label="Clash API Secret"]').setValue("");

    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.experimental as Record<string, unknown>).clash_api).toEqual({});
  });
});
