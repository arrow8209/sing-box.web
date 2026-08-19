import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import CollectionJSONEditor from "./CollectionJSONEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

function makeDraft(): Draft {
  return {
    profileId,
    revision: 1,
    content: {
      config: { services: [{ type: "echo", tag: "e1" }, { type: "custom", tag: "c1" }] },
      editorBuffers: {},
    },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

describe("CollectionJSONEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("emits immutable arrays and edits a row via full JSON", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft();
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(CollectionJSONEditor, {
      props: { module: "services", modelValue: draft.draft.content.config.services as unknown[] },
    });

    await wrapper.get('button[name="编辑 JSON 0"]').trigger("click");
    const textarea = wrapper.get("textarea");
    await textarea.setValue('{"type":"echo","tag":"e1","extra":{"k":2}}');

    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(config.services).toEqual([
      { type: "echo", tag: "e1", extra: { k: 2 } },
      { type: "custom", tag: "c1" },
    ]);
  });

  it("retains invalid object text in an index-scoped buffer", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft();
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");
    const setEditorBuffer = vi.spyOn(draft, "setEditorBuffer");

    const wrapper = mount(CollectionJSONEditor, {
      props: { module: "services", modelValue: draft.draft.content.config.services as unknown[] },
    });

    await wrapper.get('button[name="编辑 JSON 1"]').trigger("click");
    await wrapper.get("textarea").setValue('{"type":"broken"');

    expect(replaceConfig).not.toHaveBeenCalled();
    expect(setEditorBuffer).toHaveBeenCalledWith("services/1", '{"type":"broken"');
    expect(draft.draft?.content.editorBuffers["services/1"]?.valid).toBe(false);
  });

  it("reorder remaps affected buffer keys in the same mutation", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft();
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(CollectionJSONEditor, {
      props: { module: "services", modelValue: draft.draft.content.config.services as unknown[] },
    });

    await wrapper.get('button[name="上移行 1"]').trigger("click");
    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(config.services).toEqual([
      { type: "custom", tag: "c1" },
      { type: "echo", tag: "e1" },
    ]);
  });

  it("deletes a row and remaps buffer keys", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft();
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(CollectionJSONEditor, {
      props: { module: "services", modelValue: draft.draft.content.config.services as unknown[] },
    });

    await wrapper.get('button[name="删除行 0"]').trigger("click");
    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(config.services).toEqual([{ type: "custom", tag: "c1" }]);
  });
});
