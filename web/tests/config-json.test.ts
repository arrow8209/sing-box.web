import { reactive } from "vue";
import { describe, expect, it } from "vitest";
import { applyModuleBuffer, setJsonPath } from "@/features/config-editor/json";
import type { Draft } from "@/features/profiles/types";

function draftFixture(revision = 1, config: Record<string, unknown> = {}): Draft {
  return { profileId: "profile-a", revision, config, buffers: {}, bindings: [], savedAt: "2026-08-21T00:00:00Z" };
}

describe("配置 JSON 缓冲", () => {
  it("只改变一个路径，未知字段和大整数文本不丢失", () => {
    const input = '{"level":"info","future":{"large":9007199254740993,"nested":[1,"x",true]}}';
    const output = setJsonPath(input, ["level"], "debug");
    expect(output).toContain('"level": "debug"');
    expect(output).toContain('"large":9007199254740993');
    expect(output).toContain('"nested":[1,"x",true]');
  });

  it("保留无效文本和最近一次有效对象", () => {
    const draft = draftFixture(1, { outbounds: [{ type: "direct", tag: "direct" }] });
    const result = applyModuleBuffer(draft, "outbounds", '[{"type":');
    expect(result.draft.buffers.outbounds).toBe('[{"type":');
    expect(result.draft.config.outbounds).toEqual([{ type: "direct", tag: "direct" }]);
    expect(result.error?.fieldPath).toBe("outbounds");
  });

  it("可以更新 Vue 响应式草稿而不触发代理克隆错误", () => {
    const draft = reactive(draftFixture(1, { inbounds: [{ type: "mixed", tag: "mixed-in" }] }));
    draft.buffers.inbounds = `[{"type":"mixed","tag":"mixed-in"}]`;
    const result = applyModuleBuffer(draft, "inbounds", `[{"type":"tun","tag":"tun-in"}]`);
    expect(result.error).toBeUndefined();
    expect(result.draft.config.inbounds).toEqual([{ type: "tun", tag: "tun-in" }]);
    expect(draft.config.inbounds).toEqual([{ type: "mixed", tag: "mixed-in" }]);
  });
});
