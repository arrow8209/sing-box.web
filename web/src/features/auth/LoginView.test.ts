import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import LoginView from "./LoginView.vue";
import { useAuthStore } from "../../stores/auth";

describe("LoginView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("renders a read-only admin username field, a password field and a login button", () => {
    const wrapper = mount(LoginView);

    const username = wrapper.get('input[aria-label="用户名"]');
    expect(username.attributes("value")).toBe("admin");
    expect(username.attributes("readonly")).toBeDefined();

    expect(wrapper.get('input[aria-label="密码"]').attributes("type")).toBe("password");
    expect(wrapper.get('button[name="登录"]').text()).toBe("登录");
  });

  it("logs in with the typed password and clears the input after the attempt", async () => {
    const auth = useAuthStore();
    const login = vi.spyOn(auth, "login").mockResolvedValue(undefined);
    const wrapper = mount(LoginView);

    const password = wrapper.get('input[aria-label="密码"]');
    await password.setValue("hunter2-secret");
    await wrapper.get('button[name="登录"]').trigger("submit");

    expect(login).toHaveBeenCalledWith("hunter2-secret");
    expect((wrapper.get('input[aria-label="密码"]').element as HTMLInputElement).value).toBe("");
  });

  it("clears the password even when the attempt fails", async () => {
    const auth = useAuthStore();
    vi.spyOn(auth, "login").mockRejectedValue(new Error("invalid"));
    const wrapper = mount(LoginView);

    const password = wrapper.get('input[aria-label="密码"]');
    await password.setValue("wrong");
    await wrapper.get('button[name="登录"]').trigger("submit");

    expect((wrapper.get('input[aria-label="密码"]').element as HTMLInputElement).value).toBe("");
  });

  it("does not offer username editing", () => {
    const wrapper = mount(LoginView);
    expect(wrapper.findAll('input[aria-label="用户名"]')).toHaveLength(1);
    expect(wrapper.get('input[aria-label="用户名"]').attributes("readonly")).toBeDefined();
  });
});
