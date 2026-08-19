import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import RawModuleEditor from "./RawModuleEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

const draftWithLog: Draft = {
  profileId,
  revision: 1,
  content: {
    config: {
      log: { level: "info" },
      dns: { servers: [{ tag: "s" }] },
    },
    editorBuffers: {},
  },
  savedAt: "2026-08-18T10:00:00Z",
};

function mountEditor(module = "log", draft: Draft = draftWithLog) {
  const store = useDraftStore();
  store.draft = draft;
  store.etag = '"draft-1"';
  store.status = "saved";
  return mount(RawModuleEditor, {
    props: { module },
  });
}

describe("RawModuleEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("invalid text emits an unapplied buffer without touching the canonical config", async () => {
    const draft = useDraftStore();
    draft.draft = draftWithLog;
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const setEditorBuffer = vi.spyOn(draft, "setEditorBuffer");

    const wrapper = mount(RawModuleEditor, { props: { module: "log" } });
    const textarea = wrapper.get("textarea");
    await textarea.setValue('{"log": {"level":');

    expect(replaceConfig).not.toHaveBeenCalled();
    expect(setEditorBuffer).toHaveBeenCalledWith("log", '{"log": {"level":');
    // 缓冲区进入未应用状态。
    const buffer = draft.draft?.content.editorBuffers.log;
    expect(buffer?.valid).toBe(false);
    expect(buffer?.applied).toBe(false);
  });

  it("valid JSON emits the new canonical config then a valid applied buffer", async () => {
    const draft = useDraftStore();
    draft.draft = draftWithLog;
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const applyEditorBuffer = vi.spyOn(draft, "applyEditorBuffer");

    const wrapper = mount(RawModuleEditor, { props: { module: "log" } });
    const textarea = wrapper.get("textarea");
    await textarea.setValue('{"log":{"level":"warn","future":1}}');

    expect(replaceConfig).toHaveBeenCalledWith(
      expect.objectContaining({ "log": { level: "warn", future: 1 } }),
    );
    expect(applyEditorBuffer).toHaveBeenCalledWith("log", '{"log":{"level":"warn","future":1}}');
    // Other top-level modules survive.
    const nextConfig = replaceConfig.mock.calls[0][0] as Record<string, unknown>;
    expect(nextConfig.dns).toEqual({ servers: [{ tag: "s" }] });
  });

  it("masks a sensitive module behind a redacted preview until explicit reveal", async () => {
    const secretDraft: Draft = {
      profileId,
      revision: 1,
      content: {
        config: { experimental: { clash_api: { secret: "known-secret" } } },
        editorBuffers: {},
      },
      savedAt: "2026-08-18T10:00:00Z",
    };
    const wrapper = mountEditor("experimental", secretDraft);

    expect(wrapper.text()).toContain("***REDACTED***");
    expect(wrapper.text()).not.toContain("known-secret");
    expect(wrapper.find("textarea").exists()).toBe(false);
    expect(wrapper.find("textarea[disabled]").exists()).toBe(false);

    const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
    await wrapper.get('button[name="显示完整 JSON 并编辑"]').trigger("click");
    expect(confirm).toHaveBeenCalledTimes(1);

    const textarea = wrapper.get("textarea");
    expect(textarea.element.value).toContain("known-secret");
    expect(textarea.attributes("disabled")).toBeUndefined();

    // Re-mount (switch modules) masks again.
    const again = mountEditor("experimental", secretDraft);
    expect(again.text()).toContain("***REDACTED***");
    expect(again.text()).not.toContain("known-secret");
    expect(again.find("textarea").exists()).toBe(false);
  });

  it("hides an invalid secret buffer entirely behind the reveal gate", async () => {
    const secretDraft: Draft = {
      profileId,
      revision: 1,
      content: {
        config: { experimental: {} },
        editorBuffers: {
          experimental: {
            text: '{"experimental":{"clash_api":{"secret":"leaked-fragment"',
            valid: false,
            applied: false,
          },
        },
      },
      savedAt: "2026-08-18T10:00:00Z",
    };
    const wrapper = mountEditor("experimental", secretDraft);

    expect(wrapper.text()).toContain("***REDACTED***");
    expect(wrapper.text()).not.toContain("leaked-fragment");
    expect(wrapper.find("textarea").exists()).toBe(false);
  });
});
