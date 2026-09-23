import { describe, expect, it, vi } from "vitest";
import { ReferenceSyncError, synchronizeReferences, type ReferenceSources } from "@/features/application/references";
import { detachEditedInboundResources } from "@/features/config-editor/inbound";
import type { Draft } from "@/features/profiles/types";
import type { SubscriptionDetail } from "@/features/subscriptions/types";

const subscriptionId = "a".repeat(32);
const inboundId = "b".repeat(32);

function subscriptionDraft(): Draft {
  const text = '[{"type":"selector","tag":"选择","outbounds":["test-ssh"],"interrupt_exist_connections":true},{"type":"ssh","tag":"test-ssh","server":"old.example.test","detour":"removed-group"},{"type":"direct","tag":"direct","future":9007199254740993}]';
  return {
    profileId: "c".repeat(32), revision: 4, savedAt: "2026-09-01T00:00:00Z", bindings: [],
    buffers: { outbounds: text, log: '{"level":"info"}' }, config: { outbounds: JSON.parse(text), log: { level: "info" } },
    editorState: { outbounds: {
      version: 1, generatedTags: ["test-ssh"], groups: [{
        id: "group-a", type: "selector", tag: "选择", hidden: false, include: "", exclude: "", icon: "",
        references: [{ kind: "subscription", subscriptionId, tag: "测试订阅", resolvedTags: ["test-ssh"] }],
        runtimeTags: ["test-ssh"]
      }]
    } }
  };
}

function latestSubscription(): SubscriptionDetail {
  return { id: subscriptionId, name: "测试订阅", updatedAt: "2026-09-23T00:00:00Z", outbounds: [
    { type: "ssh", tag: "test-ssh", server: "new.example.test" },
    { type: "trojan", tag: "new-node", server: "new-node.example.test", password: "test-only" }
  ] };
}

function sources(): ReferenceSources {
  return { subscription: vi.fn(async () => latestSubscription()), inbound: vi.fn() };
}

function inboundDraft(): Draft {
  const text = '[{"type":"mixed","tag":"local-in","listen":"127.0.0.1","listen_port":19090,"tcp_fast_open":true},{"type":"mixed","tag":"independent","listen_port":19092,"future":9007199254740993}]';
  return {
    profileId: "c".repeat(32), revision: 4, savedAt: "2026-09-01T00:00:00Z", bindings: [],
    buffers: { inbounds: text, route: '{"future":9007199254740995,"rules":[{"action":"route","outbound":"direct"}]}' },
    config: { inbounds: JSON.parse(text), route: { rules: [{ action: "route", outbound: "direct" }] } },
    editorState: { inbounds: { version: 1, managedSniffRules: [], resourceBindings: [{ tag: "local-in", resourceId: inboundId, sourceTag: "source-in" }] } }
  };
}

describe("配置引用同步", () => {
  it("读取最新订阅，移除旧 detour 并更新策略组成员，保留非受管 JSON token", async () => {
    const draft = subscriptionDraft();
    const original = structuredClone(draft);
    const api = sources();
    const result = await synchronizeReferences(draft, api);
    const items = JSON.parse(result.buffers.outbounds!) as Array<Record<string, unknown>>;
    expect(api.subscription).toHaveBeenCalledExactlyOnceWith(subscriptionId);
    expect(items.find((item) => item.tag === "test-ssh")).toEqual({ type: "ssh", tag: "test-ssh", server: "new.example.test" });
    expect(items.find((item) => item.tag === "选择")?.outbounds).toEqual(["test-ssh", "new-node"]);
    expect(result.buffers.outbounds).toContain("9007199254740993");
    expect(items.find((item) => item.tag === "direct")).toBeDefined();
    expect(result.buffers.log).toBe(original.buffers.log);
    expect(draft).toEqual(original);
    expect(await synchronizeReferences(result, api)).toEqual(result);
  });

  it("订阅删除的节点不再保留为旧副本，重复引用只读取一次源", async () => {
    const draft = subscriptionDraft();
    const state = draft.editorState!.outbounds as { groups: Array<Record<string, unknown>> };
    const group = state.groups[0]!;
    (group.references as unknown[]).push({ kind: "subscription", subscriptionId, tag: "测试订阅", resolvedTags: ["test-ssh"] });
    const api = sources();
    vi.mocked(api.subscription).mockResolvedValue({ ...latestSubscription(), outbounds: [{ type: "direct", tag: "replacement" }] });
    const result = await synchronizeReferences(draft, api);
    const items = JSON.parse(result.buffers.outbounds!) as Array<Record<string, unknown>>;
    expect(api.subscription).toHaveBeenCalledOnce();
    expect(items.some((item) => item.tag === "test-ssh")).toBe(false);
    expect(items.find((item) => item.tag === "选择")?.outbounds).toEqual(["replacement"]);
  });

  it("指定订阅节点消失时阻止使用旧节点继续检测", async () => {
    const draft = subscriptionDraft();
    const state = draft.editorState!.outbounds as { groups: Array<{ references: unknown[] }> };
    state.groups[0]!.references = [{ kind: "subscription-node", subscriptionId, nodeTag: "removed-node", resolvedTags: ["test-ssh"] }];
    await expect(synchronizeReferences(draft, sources())).rejects.toThrow("订阅节点“removed-node”已不存在");
  });

  it("非法出站 JSON 或不一致的引用信息不会被重建覆盖", async () => {
    const draft = subscriptionDraft();
    const api = sources();
    draft.buffers.outbounds = '[{"tag":';
    await expect(synchronizeReferences(draft, api)).rejects.toBeInstanceOf(ReferenceSyncError);
    expect(draft.buffers.outbounds).toBe('[{"tag":');
    expect(api.subscription).not.toHaveBeenCalled();

    const changed = subscriptionDraft();
    changed.buffers.outbounds = changed.buffers.outbounds!.replace('["test-ssh"]', '["direct"]');
    await expect(synchronizeReferences(changed, api)).rejects.toThrow("JSON 与引用信息不一致");
    expect(api.subscription).not.toHaveBeenCalled();
  });

  it("没有来源关联的旧出站和入站不会被猜测或替换", async () => {
    const draft = { ...subscriptionDraft(), ...inboundDraft(), editorState: undefined };
    const api = sources();
    expect(await synchronizeReferences(draft, api)).toEqual(draft);
    expect(api.subscription).not.toHaveBeenCalled();
    expect(api.inbound).not.toHaveBeenCalled();
  });

  it.each(["inbounds", "outbounds"] as const)("%s 名称重复时不猜测应该更新哪个条目", async (module) => {
    const draft = module === "inbounds" ? inboundDraft() : subscriptionDraft();
    const items = JSON.parse(draft.buffers[module]!);
    items.push(structuredClone(items[0]));
    draft.buffers[module] = JSON.stringify(items);
    const api = sources();
    await expect(synchronizeReferences(draft, api)).rejects.toThrow("名称重复");
    expect(api.inbound).not.toHaveBeenCalled();
    expect(api.subscription).not.toHaveBeenCalled();
  });

  it("关联入站同步最新字段并保留本地别名，兼容旧 sniff 迁移", async () => {
    const draft = inboundDraft();
    const api = sources();
    vi.mocked(api.inbound).mockResolvedValue({ id: inboundId, mode: "structured", updatedAt: "2026-09-23T00:00:00Z", config: {
      type: "http", tag: "renamed-source", listen: "127.0.0.1", listen_port: 19091, sniff: true, sniff_timeout: "750ms", future: { keep: true }
    } });
    const result = await synchronizeReferences(draft, api);
    const items = JSON.parse(result.buffers.inbounds!) as Array<Record<string, unknown>>;
    expect(items[0]).toEqual({ type: "http", tag: "local-in", listen: "127.0.0.1", listen_port: 19091, future: { keep: true } });
    expect(result.buffers.inbounds).not.toContain(inboundId);
    expect(result.buffers.inbounds).toContain("9007199254740993");
    expect(result.buffers.route).toContain("9007199254740995");
    expect(JSON.parse(result.buffers.route!).rules[0]).toEqual({ inbound: "local-in", action: "sniff", timeout: "750ms" });
    expect(result.editorState?.inbounds).toMatchObject({ resourceBindings: [{ tag: "local-in", resourceId: inboundId, sourceTag: "renamed-source" }] });
    expect(await synchronizeReferences(result, api)).toEqual(result);
  });

  it("任一关联源读取失败时不返回部分更新", async () => {
    const outbound = subscriptionDraft();
    const inbound = inboundDraft();
    const draft = { ...outbound, buffers: { ...outbound.buffers, ...inbound.buffers }, config: { ...outbound.config, ...inbound.config }, editorState: { ...outbound.editorState, ...inbound.editorState } };
    const original = structuredClone(draft);
    const api = sources();
    vi.mocked(api.inbound).mockRejectedValue(new Error("入站资源不存在"));
    await expect(synchronizeReferences(draft, api)).rejects.toThrow("请重新关联或解除关联");
    expect(draft).toEqual(original);
  });

  it("已移除入站的旧关联不会复活入站或读取源资源", async () => {
    const draft = inboundDraft();
    draft.buffers.inbounds = "[]";
    draft.config.inbounds = [];
    const api = sources();
    const result = await synchronizeReferences(draft, api);
    expect(api.inbound).not.toHaveBeenCalled();
    expect(result.buffers.inbounds).toBe("[]");
    expect(result.editorState?.inbounds).not.toHaveProperty("resourceBindings");
  });

  it("直接编辑 JSON 只解除被修改入站的关联", () => {
    const previous = inboundDraft();
    const changed = structuredClone(previous);
    (changed.config.inbounds as Array<Record<string, unknown>>)[0]!.listen_port = 19100;
    expect(detachEditedInboundResources(previous, changed)).not.toHaveProperty("resourceBindings");
    expect(detachEditedInboundResources(previous, structuredClone(previous))).toMatchObject({ resourceBindings: [{ resourceId: inboundId }] });
  });
});
