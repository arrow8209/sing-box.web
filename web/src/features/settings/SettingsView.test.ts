import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createRouter, createMemoryHistory } from "vue-router";
import SettingsView from "./SettingsView.vue";
import { useAuthStore } from "../../stores/auth";

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/login", component: { template: "<div />" } }],
  });
}

describe("SettingsView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    localStorage.clear();
  });

  it("supports theme preference in localStorage", async () => {
    const router = makeRouter();
    router.push("/login");
    await router.isReady();
    const wrapper = mount(SettingsView, {
      global: { plugins: [router] },
    });
    await wrapper.get('input[aria-label="深色模式"]').setValue(true);
    expect(localStorage.getItem("singbox-theme")).toBe("dark");

    await wrapper.get('input[aria-label="深色模式"]').setValue(false);
    expect(localStorage.getItem("singbox-theme")).toBe("light");
  });

  it("changes the password with 新密码/确认新密码 and routes to login on success", async () => {
    const auth = useAuthStore();
    auth.session = {
      username: "admin",
      csrfToken: "csrf-1",
      expiresAt: "2030-01-01T00:00:00Z",
    };
    const changePassword = vi.spyOn(auth, "changePassword").mockResolvedValue(undefined);
    const router = makeRouter();
    router.push("/login");
    await router.isReady();
    const replace = vi.spyOn(router, "replace");

    const wrapper = mount(SettingsView, {
      global: { plugins: [router] },
    });

    await wrapper.get('input[aria-label="新密码"]').setValue("new-password-123");
    await wrapper.get('input[aria-label="确认新密码"]').setValue("new-password-123");
    await wrapper.get('button[name="修改管理员密码"]').trigger("click");

    expect(changePassword).toHaveBeenCalledWith("new-password-123");
    expect(replace).toHaveBeenCalledWith("/login");
    expect(auth.session).toBeNull();
  });

  it("rejects mismatched password confirmation", async () => {
    const auth = useAuthStore();
    const changePassword = vi.spyOn(auth, "changePassword");
    const router = makeRouter();
    router.push("/login");
    await router.isReady();

    const wrapper = mount(SettingsView, {
      global: { plugins: [router] },
    });
    await wrapper.get('input[aria-label="新密码"]').setValue("new-password-123");
    await wrapper.get('input[aria-label="确认新密码"]').setValue("different-456");
    await wrapper.get('button[name="修改管理员密码"]').trigger("click");

    expect(changePassword).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("两次输入的密码不一致");
  });
});
