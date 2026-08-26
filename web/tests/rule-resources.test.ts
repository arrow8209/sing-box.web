import { mount } from "@vue/test-utils";
import { describe, expect, it, vi } from "vitest";
import RuleResourceDialog from "@/features/rule-resources/RuleResourceDialog.vue";
import { createRuleResourceStore } from "@/features/rule-resources/store";
import type { RuleCatalog, RuleResource, RuleResourceInput } from "@/features/rule-resources/types";

const resource: RuleResource = {
  id: "a".repeat(32), sourceType: "remote", name: "GeoSite CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "6h", disabled: false,
  localPath: "data/rulesets/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.srs", lastDigest: "b".repeat(64)
};

describe("规则资源", () => {
  it("创建输入永远不包含服务端只读 localPath", async () => {
    const wrapper = mount(RuleResourceDialog, { props: { open: true } });
    await wrapper.get('input[placeholder="例如 geosite-cn"]').setValue("GeoSite CN");
    await wrapper.get('input[placeholder="https://example.com/rules.srs"]').setValue("https://example.test/cn.srs");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as RuleResourceInput;
    expect(submitted).toEqual({ sourceType: "remote", name: "GeoSite CN", url: "https://example.test/cn.srs", format: "binary", updateInterval: "24h", disabled: false });
    expect(wrapper.get(".fixed-format").text()).toContain("Binary");
    expect(wrapper.find('input[value="binary"]').exists()).toBe(false);
    expect(wrapper.text()).not.toContain("Source");
    expect(JSON.stringify(submitted)).not.toContain("localPath");
  });

  it("手工规则在提交前校验 JSON，并只发送受管内容", async () => {
    const wrapper = mount(RuleResourceDialog, { props: { open: true } });
    await wrapper.get('input[value="manual"]').setValue();
    await wrapper.get('input[placeholder="例如 geosite-cn"]').setValue("E2E 私有规则");
    await wrapper.get("textarea").setValue(`{"version":3,"rules":[]}`);
    expect(wrapper.text()).toContain("rules 必须至少包含一个规则块");
    expect(wrapper.get('button[type="submit"]').attributes("disabled")).toBeDefined();

    await wrapper.get("textarea").setValue(`{"version":3,"rules":[{"domain_suffix":["example.com","example.org"]}]}`);
    expect(wrapper.text()).toContain("JSON 有效 · 1 个规则块");
    await wrapper.get("form").trigger("submit");
    const submitted = wrapper.emitted("submit")?.[0]?.[0] as RuleResourceInput;
    expect(submitted).toEqual({
      sourceType: "manual",
      name: "E2E 私有规则",
      url: "",
      format: "source",
      updateInterval: "",
      disabled: false,
      content: { version: 3, rules: [{ domain_suffix: ["example.com", "example.org"] }] }
    });
    expect(wrapper.text()).toContain("手工规则固定使用 Source JSON");
    expect(JSON.stringify(submitted)).not.toContain("localPath");
  });

  it("更新期间锁定同一资源，并刷新服务端周期状态", async () => {
    let release: (() => void) | undefined;
    const api = {
      list: vi.fn(async () => [resource]), get: vi.fn(), create: vi.fn(), patch: vi.fn(), remove: vi.fn(), references: vi.fn(), catalog: vi.fn(), refreshCatalog: vi.fn(), previewCatalog: vi.fn(), importCatalog: vi.fn(),
      update: vi.fn(async () => await new Promise<{ changed: boolean; digest: string; message: string }>((resolve) => { release = () => resolve({ changed: true, digest: resource.lastDigest!, message: "规则资源已更新，请点击“应用" }); }))
    };
    const store = createRuleResourceStore(api); await store.refresh(); const pending = store.update(resource.id);
    expect(store.updatingId.value).toBe(resource.id); release?.(); await pending;
    expect(store.updatingId.value).toBeNull(); expect(api.list).toHaveBeenCalledTimes(2);
  });

  it("批量更新最多并发五项，并汇总变化与失败", async () => {
    const enabled = Array.from({ length: 8 }, (_, index): RuleResource => ({
      ...resource,
      id: `remote-${index}`,
      name: `Remote ${index}`,
      lastDigest: `${index}`.repeat(64).slice(0, 64)
    }));
    const excluded: RuleResource[] = [
      { ...resource, id: "disabled", disabled: true },
      { ...resource, id: "manual", sourceType: "manual", url: "", format: "source", updateInterval: "" }
    ];
    const items = [...enabled, ...excluded];
    let active = 0;
    let maximum = 0;
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    const api = {
      list: vi.fn(async () => items), get: vi.fn(), create: vi.fn(), patch: vi.fn(), remove: vi.fn(), references: vi.fn(), catalog: vi.fn(), refreshCatalog: vi.fn(), previewCatalog: vi.fn(), importCatalog: vi.fn(),
      update: vi.fn(async (id: string) => {
        active++;
        maximum = Math.max(maximum, active);
        await gate;
        active--;
        const index = Number(id.split("-").at(-1));
        if (index === 7) throw new Error("offline");
        return { changed: index % 2 === 0, digest: "a".repeat(64), message: "done" };
      })
    };
    const store = createRuleResourceStore(api);
    const pending = store.updateAll(items, 5);
    await vi.waitFor(() => expect(api.update).toHaveBeenCalledTimes(5));
    expect(maximum).toBe(5);
    release();
    await expect(pending).resolves.toEqual({ total: 8, changed: 4, unchanged: 3, failed: 1 });
    expect(api.update).toHaveBeenCalledTimes(8);
    expect(api.list).toHaveBeenCalledOnce();
    expect(store.updatingIds.value).toEqual([]);
  });

  it("首次加载目录会自动刷新，并把导入结果加入资源列表", async () => {
    const unavailable: RuleCatalog = { available: false, source: "Ruleset-Hub", geosite: "https://example.test/geosite/", geoip: "https://example.test/geoip/", total: 0, geositeCount: 0, geoipCount: 0, items: [] };
    const available: RuleCatalog = {
      available: true,
      source: "https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full.json",
      geosite: "https://example.test/geosite/",
      geoip: "https://example.test/geoip/",
      digest: "a".repeat(64),
      total: 1,
      geositeCount: 1,
      geoipCount: 0,
      items: [{ key: "geosite:cn", name: "cn", type: "geosite", count: 111868 }]
    };
    const imported = { ...resource, id: "c".repeat(32), catalogKey: "geosite:cn" };
    const api = {
      list: vi.fn(async () => []), get: vi.fn(), create: vi.fn(), patch: vi.fn(), update: vi.fn(), remove: vi.fn(), references: vi.fn(), previewCatalog: vi.fn(),
      catalog: vi.fn(async () => unavailable),
      refreshCatalog: vi.fn(async () => available),
      importCatalog: vi.fn(async () => ({ resource: imported, update: { changed: true, digest: "b".repeat(64), message: "done" } }))
    };
    const store = createRuleResourceStore(api);
    await expect(store.loadCatalog()).resolves.toEqual(available);
    expect(api.catalog).toHaveBeenCalledOnce();
    expect(api.refreshCatalog).toHaveBeenCalledOnce();
    await store.importCatalog("geosite:cn");
    expect(api.importCatalog).toHaveBeenCalledWith("geosite:cn", "24h");
    expect(store.resources.value).toEqual([imported]);
  });
});
