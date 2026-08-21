import { mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import LoginView from "@/features/auth/LoginView.vue";
import router, { resetRouterAuthForTests } from "@/router";
import { useAuthStore } from "@/features/auth/store";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function mockAPI(routes: Record<string, { status: number; body: unknown }>): void {
  vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
    const route = routes[String(input)];
    const result = route ?? { status: 404, body: { error: { code: "not_found", message: "not found" } } };
    return Promise.resolve(jsonResponse(result.body, result.status));
  }));
}

describe("认证", () => {
  beforeEach(() => {
    resetRouterAuthForTests();
    useAuthStore().reset();
  });

  afterEach(async () => {
    useAuthStore().reset();
    await router.replace("/login");
  });

  it("submits only the password and never renders a username field", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({ csrfToken: "csrf-a" }))
      .mockResolvedValueOnce(jsonResponse({ valid: true, expiresAt: "2026-08-22T00:00:00Z", csrfToken: "csrf-a" }));
    vi.stubGlobal("fetch", fetchMock);
    const wrapper = mount(LoginView, { global: { plugins: [router] } });
    await wrapper.get("input[type=password]").setValue("correct horse battery staple");
    await wrapper.get("form").trigger("submit");
    expect(wrapper.find('input[name="username"]').exists()).toBe(false);
    const [, request] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(JSON.parse(String(request.body))).toEqual({ password: "correct horse battery staple" });
  });

  it("redirects an invalid session to login", async () => {
    mockAPI({ "/api/v1/auth/session": { status: 200, body: { valid: false, expiresAt: null } } });
    await router.push("/profiles");
    await router.isReady();
    expect(router.currentRoute.value.name).toBe("login");
  });
});
