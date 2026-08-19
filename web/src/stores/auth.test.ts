import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiClient, APIClientError } from "../api/client";
import { useAuthStore } from "./auth";
import type { SessionResponse } from "../api/types";

const sessionBody: SessionResponse = {
  username: "admin",
  csrfToken: "csrf-42",
  expiresAt: "2030-01-01T00:00:00Z",
};

describe("useAuthStore", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
    apiClient.clearCSRFToken();
  });

  it("login always sends username admin", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: sessionBody,
      etag: null,
    });
    const auth = useAuthStore();

    await auth.login("hunter2-secret");

    expect(request).toHaveBeenCalledTimes(1);
    const [path, init] = request.mock.calls[0];
    expect(path).toBe("/auth/login");
    expect(JSON.parse((init?.body as string) ?? "{}")).toEqual({
      username: "admin",
      password: "hunter2-secret",
    });
    expect(auth.session?.csrfToken).toBe("csrf-42");
  });

  it("restore fetches the session and updates the client CSRF token", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: sessionBody,
      etag: null,
    });
    const auth = useAuthStore();

    await auth.restore();

    expect(request).toHaveBeenCalledTimes(1);
    expect(request.mock.calls[0][0]).toBe("/auth/session");
    expect(auth.session).toEqual(sessionBody);
    expect(apiClient).toBeDefined();
  });

  it("restore treats 401 as a normal logged-out state", async () => {
    vi.spyOn(apiClient, "request").mockRejectedValue(
      new APIClientError(401, { code: "unauthorized", message: "未认证" }),
    );
    const auth = useAuthStore();

    await auth.restore();

    expect(auth.session).toBeNull();
    expect(auth.loading).toBe(false);
  });

  it("logout clears the session", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ data: null, etag: null });
    const auth = useAuthStore();
    auth.session = sessionBody;

    await auth.logout();

    expect(request).toHaveBeenCalledWith("/auth/logout", expect.objectContaining({ method: "POST" }));
    expect(auth.session).toBeNull();
  });

  it("changePassword posts the new password", async () => {
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({ data: null, etag: null });
    const auth = useAuthStore();
    auth.session = sessionBody;

    await auth.changePassword("new-secret-password");

    expect(request).toHaveBeenCalledWith(
      "/auth/password",
      expect.objectContaining({ method: "PUT", body: JSON.stringify({ password: "new-secret-password" }) }),
    );
  });
});
