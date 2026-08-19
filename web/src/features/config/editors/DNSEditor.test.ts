import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import DNSEditor from "./DNSEditor.vue";
import { useDraftStore } from "../../../stores/draft";
import type { Draft } from "../../../api/types";

interface RuleEntry {
  query?: string[];
  server?: string;
}

const profileId = "a".repeat(32);

function makeDraft(dns: Record<string, unknown>): Draft {
  return {
    profileId,
    revision: 1,
    content: { config: { dns }, editorBuffers: {} },
    savedAt: "2026-08-18T10:00:00Z",
  };
}

describe("DNSEditor", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("adds, edits, removes and reorders dns.servers while keeping unknown fields", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      servers: [{ tag: "s1", address: "1.1.1.1" }],
      future_option: { enabled: true },
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(DNSEditor);
    const serverInputs = () => wrapper.findAll('input[aria-label="服务器地址"]');
    expect(serverInputs()).toHaveLength(1);

    await serverInputs()[0].setValue("8.8.8.8");
    let config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.dns as Record<string, unknown>).servers).toEqual([
      { tag: "s1", address: "8.8.8.8" },
    ]);

    await wrapper.get('button[name="添加服务器"]').trigger("click");
    expect(replaceConfig.mock.calls.at(-1)?.[0]).toBeDefined();
    config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.dns as Record<string, unknown>).servers).toHaveLength(2);

    await wrapper.get('button[name="删除服务器 1"]').trigger("click");
    config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect((config.dns as Record<string, unknown>).servers).toHaveLength(1);

    await wrapper.get('button[name="上移服务器 0"]').trigger("click");
    config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    // Reorder keeps the unknown sibling.
    expect((config.dns as Record<string, unknown>).future_option).toEqual({ enabled: true });
  });

  it("edits dns.rules with reorder and unknown-field preservation", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      rules: [{ query: ["a.com"], server: "s1" }, { query: ["b.com"], server: "s2" }],
      unknown_rule_field: true,
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(DNSEditor);
    const ruleInputs = () => wrapper.findAll('input[aria-label="规则查询"]');
    expect(ruleInputs()).toHaveLength(2);

    await ruleInputs()[0].setValue("c.com");
    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    const rules = (config.dns as Record<string, unknown>).rules as RuleEntry[];
    expect(rules[0]).toEqual({ query: ["c.com"], server: "s1" });
    expect((config.dns as Record<string, unknown>).unknown_rule_field).toBe(true);

    await wrapper.get('button[name="上移规则 1"]').trigger("click");
    const reordered = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    const reorderedRules = (reordered.dns as Record<string, unknown>).rules as RuleEntry[];
    expect(reorderedRules[0]).toEqual({ query: ["b.com"], server: "s2" });
    expect(reorderedRules[1]).toEqual({ query: ["c.com"], server: "s1" });
    expect((reordered.dns as Record<string, unknown>).unknown_rule_field).toBe(true);
  });

  it("edits disable_cache, disable_expire, independent_cache and final", async () => {
    const draft = useDraftStore();
    draft.draft = makeDraft({
      disable_cache: false,
      final: "s1",
      future_cache_option: 3,
    });
    draft.status = "saved";
    const replaceConfig = vi.spyOn(draft, "replaceConfig");

    const wrapper = mount(DNSEditor);

    await wrapper.get('input[aria-label="禁用缓存"]').setValue(true);
    await wrapper.get('input[aria-label="禁用过期"]').setValue(true);
    await wrapper.get('input[aria-label="独立缓存"]').setValue(true);

    const config = replaceConfig.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(config.dns).toEqual(
      expect.objectContaining({
        disable_cache: true,
        disable_expire: true,
        independent_cache: true,
        future_cache_option: 3,
      }),
    );
  });
});
