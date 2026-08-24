import { flushPromises, mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocked = vi.hoisted(() => ({ store: null as any }));

vi.mock("@/features/rule-resources/store", async () => {
  const { ref } = await vi.importActual<typeof import("vue")>("vue");
  const catalog = ref<any>(null);
  const resources = ref<any[]>([]);
  const store = {
    catalog,
    resources,
    catalogLoading: ref(false),
    loadCatalog: vi.fn(async () => catalog.value),
    previewCatalog: vi.fn(),
    importCatalog: vi.fn()
  };
  mocked.store = store;
  return { useRuleResourceStore: () => store };
});

import RuleCatalogDialog from "@/features/rule-resources/RuleCatalogDialog.vue";

function catalogItems(): Array<{ key: string; name: string; type: "geosite" | "geoip"; count: number }> {
  return [
    ...Array.from({ length: 30 }, (_, index) => ({ key: `geosite:site-${String(index).padStart(3, "0")}`, name: `site-${String(index).padStart(3, "0")}`, type: "geosite" as const, count: index + 1 })),
    ...Array.from({ length: 30 }, (_, index) => ({ key: `geoip:ip-${String(index).padStart(3, "0")}`, name: `ip-${String(index).padStart(3, "0")}`, type: "geoip" as const, count: index + 1 }))
  ];
}

describe("规则集目录", () => {
  beforeEach(() => {
    const items = catalogItems();
    mocked.store.catalog.value = { available: true, source: "Ruleset-Hub", geosite: "https://example.test/geosite/", geoip: "https://example.test/geoip/", total: 60, geositeCount: 30, geoipCount: 30, items };
    mocked.store.resources.value = [];
    mocked.store.catalogLoading.value = false;
    mocked.store.loadCatalog.mockReset().mockResolvedValue(mocked.store.catalog.value);
    mocked.store.previewCatalog.mockReset().mockResolvedValue({ key: items[0].key, ruleBlocks: 1, ruleCount: 2, truncated: false, content: { version: 3, rules: [{ domain_suffix: ["example.com"] }] } });
    mocked.store.importCatalog.mockReset().mockImplementation(async (key: string, format: "source" | "binary") => {
      const imported = { id: "a".repeat(32), catalogKey: key, sourceType: "remote", name: "site-000-geosite", url: `https://example.test/site-000.${format === "source" ? "json" : "srs"}`, format, updateInterval: "24h", disabled: false, localPath: `data/rulesets/${"a".repeat(32)}.${format === "source" ? "json" : "srs"}` };
      mocked.store.resources.value = [...mocked.store.resources.value, imported];
      return { resource: imported, update: { changed: true, digest: "b".repeat(64), message: "done" } };
    });
  });

  it("每页展示 27 项，搜索和分类后回到第一页", async () => {
    const wrapper = mount(RuleCatalogDialog, { props: { open: false } });
    await wrapper.setProps({ open: true });
    await flushPromises();
    expect(mocked.store.loadCatalog).toHaveBeenCalledWith(false);
    expect(wrapper.findAll(".catalog-card")).toHaveLength(27);
    expect(wrapper.find(".catalog-card").text()).toContain("规则数量：1");
    expect(wrapper.find(".catalog-card__type").text()).toBe("geosite");
    expect(wrapper.find(".add-config-button").text()).toBe("添加到配置");

    await wrapper.findAll(".pagination button")[1].trigger("click");
    expect(wrapper.find(".catalog-card").text()).toContain("site-027");
    await wrapper.get(".catalog-search input").setValue("site-029");
    expect(wrapper.findAll(".catalog-card")).toHaveLength(1);
    expect(wrapper.find(".catalog-card").text()).toContain("site-029");

    await wrapper.get(".catalog-search input").setValue("");
    await wrapper.findAll(".catalog-filters button")[2].trigger("click");
    expect(wrapper.findAll(".catalog-card")).toHaveLength(27);
    expect(wrapper.find(".catalog-card").text()).toContain("ip-000");
    expect(wrapper.text()).toContain("第 1 / 2 页");
  });

  it("可预览真实 Source 结构并在导入后显示已添加状态", async () => {
    const wrapper = mount(RuleCatalogDialog, { props: { open: false } });
    await wrapper.setProps({ open: true });
    await flushPromises();
    await wrapper.find(".preview-button").trigger("click");
    await flushPromises();
    expect(mocked.store.previewCatalog).toHaveBeenCalledWith("geosite:site-000");
    expect(wrapper.find(".preview-editor").text()).toContain("example.com");

    await wrapper.get(".back-button").trigger("click");
    await wrapper.find(".source-action").trigger("click");
    await flushPromises();
    expect(mocked.store.importCatalog).toHaveBeenCalledWith("geosite:site-000", "source");
    expect(wrapper.emitted("notice")?.[0]?.[0]).toContain("site-000 的 source 格式已导入");
    expect(wrapper.find(".source-action").text()).toContain("已添加 Source");

    await wrapper.find(".add-config-button").trigger("click");
    await flushPromises();
    expect(mocked.store.importCatalog).toHaveBeenLastCalledWith("geosite:site-000", "binary");
    expect(wrapper.find(".add-config-button").text()).toBe("已添加到配置");
  });

  it("旧资源只按完整类型地址匹配，不混淆同名 GeoSite 和 GeoIP", async () => {
    mocked.store.catalog.value = {
      available: true,
      source: "Ruleset-Hub",
      geosite: "https://example.test/geosite/",
      geoip: "https://example.test/geoip/",
      total: 2,
      geositeCount: 1,
      geoipCount: 1,
      items: [
        { key: "geosite:cn", name: "cn", type: "geosite", count: 10 },
        { key: "geoip:cn", name: "cn", type: "geoip", count: 20 }
      ]
    };
    mocked.store.resources.value = [{ id: "c".repeat(32), sourceType: "remote", name: "legacy", url: "https://example.test/geosite/cn.json", format: "source", updateInterval: "24h", disabled: false, localPath: "data/rulesets/legacy.json" }];
    const wrapper = mount(RuleCatalogDialog, { props: { open: false } });
    await wrapper.setProps({ open: true });
    await flushPromises();
    const cards = wrapper.findAll(".catalog-card");
    expect(cards[0].find(".source-action").text()).toContain("已添加 Source");
    expect(cards[1].find(".source-action").text()).toContain("添加 Source");
    expect(cards[1].find(".source-action").text()).not.toContain("已添加");
  });
});
