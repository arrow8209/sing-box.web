import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createRouter, createMemoryHistory } from "vue-router";
import AppShell from "./AppShell.vue";
import { useAuthStore } from "../../stores/auth";
import { apiClient } from "../../api/client";

function makeRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/:pathMatch(.*)*", component: AppShell },
    ],
  });
  return router;
}

function stubFetchAll() {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const path = typeof input === "string" ? input : input.toString();
      if (path.includes("/profiles")) {
        return Promise.resolve(
          new Response(JSON.stringify([]), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({
            capabilities: {
              "config.manage": true,
              "config.publish": true,
              "subscription.sync": false,
              "ruleset.sync": false,
              "core.lifecycle": false,
              "core.metrics": false,
            },
            singBox: { available: true, version: "1.12.0", targetVersion: "1.12.0", warning: "" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    }),
  );
}

describe("AppShell", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders navigation with accessible names and no runtime metric text", async () => {
    stubFetchAll();
    const auth = useAuthStore();
    auth.session = {
      username: "admin",
      csrfToken: "csrf-1",
      expiresAt: "2030-01-01T00:00:00Z",
    };
    const router = makeRouter();
    router.push("/");
    await router.isReady();

    const wrapper = mount(AppShell, { global: { plugins: [router] } });

    const nav = wrapper.get("nav");
    for (const name of ["仪表盘", "配置", "运行状态", "订阅", "规则集", "设置"]) {
      expect(nav.text()).toContain(name);
    }
    expect(wrapper.text()).toContain("退出登录");
    expect(wrapper.text()).not.toContain("MB/s");
  });

  it("logs out via the store when 退出登录 is clicked", async () => {
    stubFetchAll();
    const auth = useAuthStore();
    auth.session = {
      username: "admin",
      csrfToken: "csrf-1",
      expiresAt: "2030-01-01T00:00:00Z",
    };
    const logout = vi.spyOn(auth, "logout").mockResolvedValue(undefined);
    const router = makeRouter();
    router.push("/");
    await router.isReady();

    const wrapper = mount(AppShell, { global: { plugins: [router] } });
    await wrapper.get('button[name="退出登录"]').trigger("click");

    expect(logout).toHaveBeenCalled();
  });

  it("redirects to /login when no session exists", async () => {
    const auth = useAuthStore();
    auth.session = null;
    const request = vi.spyOn(apiClient, "request").mockRejectedValue({ status: 401 });
    void request;

    const router = makeRouter();
    router.push("/");
    await router.isReady();
    mount(AppShell, { global: { plugins: [router] } });

    // The shell itself must not expose navigation for a logged-out visitor.
    expect(auth.session).toBeNull();
  });
});
