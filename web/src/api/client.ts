import type { APIError } from "./types";

export interface APIResponse<T> {
  data: T;
  etag: string | null;
}

export class APIClientError extends Error {
  constructor(
    public readonly status: number,
    public readonly detail: APIError,
  ) {
    super(detail.message);
  }
}

type UnauthorizedListener = () => void;

interface ErrorEnvelope {
  error: APIError;
}

function apiPath(path: string): string {
  if (path === "/api/v1" || path.startsWith("/api/v1/")) {
    return path;
  }
  return `/api/v1/${path.replace(/^\/+/, "")}`;
}

export class APIClient {
  private csrfToken: string | null = null;
  private listeners = new Set<UnauthorizedListener>();

  setCSRFToken(token: string): void {
    this.csrfToken = token;
  }

  clearCSRFToken(): void {
    this.csrfToken = null;
  }

  on(listener: UnauthorizedListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private emitUnauthorized(): void {
    for (const listener of this.listeners) {
      listener();
    }
  }

  async request<T>(path: string, init: RequestInit = {}): Promise<APIResponse<T>> {
    const headers = new Headers(init.headers);
    if (!headers.has("Content-Type") && init.body !== undefined) {
      headers.set("Content-Type", "application/json");
    }
    const method = (init.method ?? "GET").toUpperCase();
    if (method !== "GET" && this.csrfToken !== null) {
      headers.set("X-CSRF-Token", this.csrfToken);
    }

    const response = await fetch(apiPath(path), {
      ...init,
      method,
      headers,
      credentials: "same-origin",
    });

    const etag = response.headers.get("ETag");

    if (!response.ok) {
      let detail: APIError = { code: "http_error", message: `请求失败（${response.status}）` };
      try {
        const body = (await response.json()) as ErrorEnvelope;
        if (body && body.error) {
          detail = body.error;
        }
      } catch {
        // Non-JSON error body: keep the fallback detail.
      }
      if (response.status === 401) {
        this.clearCSRFToken();
        this.emitUnauthorized();
      }
      throw new APIClientError(response.status, detail);
    }

    if (response.status === 204) {
      return { data: null as T, etag };
    }
    const data = (await response.json()) as T;
    return { data, etag };
  }
}

export const apiClient = new APIClient();
