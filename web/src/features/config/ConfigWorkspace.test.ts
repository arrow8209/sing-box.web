import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createRouter, createMemoryHistory } from "vue-router";
import ConfigWorkspace from "./ConfigWorkspace.vue";
import { useDraftStore } from "../../stores/draft";
import type { Draft } from "../../api/types";

const profileId = "a".repeat(32);

const draftBody: Draft = {
  profileId,
  revision: 1,
  content: {
    config: { log: { level: "info" }, dns: { servers: [] } },
    editorBuffers: {},
  },
  savedAt: "2026-08-18T10:00:00Z",
};

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: "/config/:profileId?/:module?",
        component: ConfigWorkspace,
      },
    ],
  });
}

async function mountWorkspace(module = "general") {
  const router = makeRouter();
  router.push(`/config/${profileId}/${module}`);
  await router.isReady();
  const wrapper = mount(ConfigWorkspace, {
    global: { plugins: [router] },
  });
  return { wrapper, router };
}

describe("ConfigWorkspace", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("renders a tablist with the module short labels", async () => {
    const draft = useDraftStore();
    draft.draft = draftBody;
    draft.etag = '"draft-1"';
    draft.status = "saved";

    const { wrapper } = await mountWorkspace();
    const tabs = wrapper.findAll('[role="tab"]');
    expect(tabs.length).toBeGreaterThanOrEqual(8);
    const labels = tabs.map((tab) => tab.text());
    expect(labels).toContain("概览");
    expect(labels).toContain("DNS");
    expect(labels).toContain("入站");
    expect(labels).toContain("出站");
    expect(labels).toContain("路由");
    expect(labels).toContain("高级");
  });

  it("opens the raw editor through 编辑<模块> JSON with accessible name <模块> JSON", async () => {
    const draft = useDraftStore();
    draft.draft = draftBody;
    draft.etag = '"draft-1"';
    draft.status = "saved";

    const { wrapper } = await mountWorkspace();
    await wrapper.get('button[name="编辑概览 JSON"]').trigger("click");

    const raw = wrapper.get('section[aria-label="概览 JSON"]');
    expect(raw.get("textarea").isVisible()).toBe(true);
  });

  it("navigates between module tabs through the router", async () => {
    const draft = useDraftStore();
    draft.draft = draftBody;
    draft.etag = '"draft-1"';
    draft.status = "saved";

    const { wrapper, router } = await mountWorkspace();
    const dnsTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text() === "DNS");
    await dnsTab!.trigger("click");
    await flushPromises();

    expect(router.currentRoute.value.params.module).toBe("dns");
  });

  it("shows a disabled-state banner when the draft is read-only", async () => {
    const draft = useDraftStore();
    draft.status = "error";
    draft.lastError = { code: "draft_read_only", message: "草稿 JSON 损坏，需要管理员在服务器修复" };

    const { wrapper } = await mountWorkspace();
    expect(wrapper.text()).toContain("草稿 JSON 损坏，需要管理员在服务器修复");
    expect(wrapper.text()).toContain("草稿只读，编辑已禁用");
    // 编辑控件不渲染，避免浏览器侧误改损坏草稿。
    expect(wrapper.find("textarea").exists()).toBe(false);
  });
});
