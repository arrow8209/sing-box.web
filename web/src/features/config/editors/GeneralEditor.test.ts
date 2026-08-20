import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import GeneralEditor from "./GeneralEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

const profileId = "a".repeat(32);

function makeDraft(log: Record<string, unknown>): Draft {
  return {
    profileId,
    revision: 1,
    content: { config: { log }, editorBuffers: {} },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

describe("GeneralEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("edits log.disabled, level, output and timestamp while keeping unknown fields", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      level: "info",
      future_option: { enabled: true },
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(GeneralEditor);
    const level = wrapper.get('select[aria-label="日志级别"]');
    expect((level.element as HTMLSelectElement).value).toBe("info");
    await level.setValue("debug");

    expect(replaceConfig).toHaveBeenCalled();
    const nextConfig = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((nextConfig.log as Record<string, unknown>).level).toBe("debug");
    expect((nextConfig.log as Record<string, unknown>).future_option).toEqual({ enabled: true });

    const disabled = wrapper.get('input[aria-label="禁用日志"]');
    await disabled.setValue(true);
    expect((replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>).log).toEqual(
      expect.objectContaining({ disabled: true }),
    );

    const output = wrapper.get('input[aria-label="日志输出路径"]');
    await output.setValue("/var/log/sb.log");
    const lastOutput = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((lastOutput.log as Record<string, unknown>).output).toBe("/var/log/sb.log");
    expect((lastOutput.log as Record<string, unknown>).future_option).toEqual({ enabled: true });

    const timestamp = wrapper.get('input[aria-label="日志时间戳"]');
    await timestamp.setValue(true);
    let last = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((last.log as Record<string, unknown>).timestamp).toBe(true);

    await timestamp.setValue(false);
    last = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((last.log as Record<string, unknown>).timestamp).toBe(false);
  });

  it("deletes the optional output field when emptied", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({ level: "info", output: "console" });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(GeneralEditor);
    const output = wrapper.get('input[aria-label="日志输出路径"]');
    await output.setValue("");

    const nextConfig = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(nextConfig.log).toEqual({ level: "info" });
  });

  it("marks each structured log control with its JSON Pointer", () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({ level: "info" });
    draft.status = "saved";

    const wrapper = mount(GeneralEditor);
    expect(wrapper.get('input[aria-label="禁用日志"]').attributes("data-json-path")).toBe("/log/disabled");
    expect(wrapper.get('select[aria-label="日志级别"]').attributes("data-json-path")).toBe("/log/level");
    expect(wrapper.get('input[aria-label="日志输出路径"]').attributes("data-json-path")).toBe("/log/output");
    expect(wrapper.get('input[aria-label="日志时间戳"]').attributes("data-json-path")).toBe("/log/timestamp");
  });
});
