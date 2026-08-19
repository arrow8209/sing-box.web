import { afterEach, describe, expect, it, vi } from "vitest";
import { APIClient, APIClientError } from "./client";

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

function stubFetch(response: Response) {
  const fetchMock = vi.fn().mockResolvedValue(response);
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function stubFetchEach(factory: () => Response) {
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(factory()));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function getFetch() {
  return vi.mocked(fetch);
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("APIClient", () => {
  it("sets credentials same-origin and never sets Origin on login", async () => {
    const client = new APIClient();
    const fetchMock = stubFetch(jsonResponse(200, { username: "admin", csrfToken: "t", expiresAt: "2030-01-01T00:00:00Z" }));

    await client.request("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username: "admin", password: "secret" }),
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const init = getFetch().mock.calls[0][1] as RequestInit;
    expect(init.credentials).toBe("same-origin");
    expect(init.headers).toBeDefined();
    const headers = init.headers as Headers;
    expect(headers.has("Origin")).toBe(false);
  });

  it("adds the CSRF token only on non-GET API calls", async () => {
    const client = new APIClient();
    client.setCSRFToken("csrf-1");
    stubFetchEach(() => jsonResponse(200, {}));

    await client.request("/profiles", { method: "GET" });
    await client.request("/profiles", { method: "POST", body: "{}" });

    const calls = getFetch().mock.calls;
    const getInit = calls[0][1] as RequestInit;
    const postInit = calls[1][1] as RequestInit;
    expect((getInit.headers as Headers).has("X-CSRF-Token")).toBe(false);
    expect((postInit.headers as Headers).get("X-CSRF-Token")).toBe("csrf-1");
  });

  it("returns the ETag alongside draft data", async () => {
    const client = new APIClient();
    stubFetch(jsonResponse(200, { profileId: "p1" }, { ETag: '"draft-3"' }));

    const result = await client.request<{ profileId: string }>("/profiles/p1/draft");

    expect(result.data.profileId).toBe("p1");
    expect(result.etag).toBe('"draft-3"');
  });

  it("throws a typed APIClientError with code and current revision", async () => {
    const client = new APIClient();
    stubFetch(
      jsonResponse(409, {
        error: { code: "draft_conflict", message: "草稿已被其他页面更新", currentRevision: 2 },
      }),
    );

    const attempt = client.request("/profiles/p1/draft", { method: "PUT", body: "{}" });
    await expect(attempt).rejects.toBeInstanceOf(APIClientError);
    await attempt.catch((error: unknown) => {
      expect(error).toBeInstanceOf(APIClientError);
      const typed = error as APIClientError;
      expect(typed.status).toBe(409);
      expect(typed.detail.code).toBe("draft_conflict");
      expect(typed.detail.currentRevision).toBe(2);
      expect(typed.message).toBe("草稿已被其他页面更新");
    });
  });

  it("clears in-memory auth and emits one unauthorized event on 401", async () => {
    const client = new APIClient();
    client.setCSRFToken("csrf-9");
    stubFetchEach(() => jsonResponse(401, { error: { code: "unauthorized", message: "未认证" } }));

    const events: string[] = [];
    const unsubscribe = client.on(() => events.push("unauthorized"));

    await expect(client.request("/profiles")).rejects.toBeInstanceOf(APIClientError);
    expect(events).toEqual(["unauthorized"]);
    unsubscribe();

    // After a 401 the client must not keep sending a stale CSRF token.
    const nextFetch = stubFetch(jsonResponse(200, {}));
    await client.request("/profiles", { method: "GET" });
    const init = nextFetch.mock.calls[0][1] as RequestInit;
    expect((init.headers as Headers).has("X-CSRF-Token")).toBe(false);
  });

  it("emits unauthorized only once for a single 401", async () => {
    const client = new APIClient();
    stubFetch(jsonResponse(401, { error: { code: "unauthorized", message: "未认证" } }));

    const events: string[] = [];
    client.on(() => events.push("unauthorized"));

    await expect(client.request("/profiles")).rejects.toBeInstanceOf(APIClientError);
    expect(events).toHaveLength(1);
  });
});
