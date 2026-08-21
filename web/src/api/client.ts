import { ApiError, type ApiErrorBody, type ApiResponse, type RequestOptions } from "./types";

export { ApiError };

let csrfToken = "";
let unauthorizedHandler: (() => void) | undefined;

export function setUnauthorizedHandler(handler: (() => void) | undefined): void {
  unauthorizedHandler = handler;
}

export function clearCSRFToken(): void {
  csrfToken = "";
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<ApiResponse<T>> {
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (options.body !== undefined) headers.set("Content-Type", "application/json");
  if (!/^(GET|HEAD)$/.test(options.method ?? "GET") && csrfToken) headers.set("X-CSRF-Token", csrfToken);
  const response = await fetch(`/api/v1${path}`, {
    method: options.method ?? "GET",
    credentials: "same-origin",
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  });
  const payload = response.status === 204 ? undefined : await response.json();
  if (!response.ok) {
    if (response.status === 401) {
      clearCSRFToken();
      unauthorizedHandler?.();
    }
    throw new ApiError(response.status, (payload?.error ?? payload) as ApiErrorBody);
  }
  if (payload?.csrfToken) csrfToken = payload.csrfToken;
  return { data: payload as T, etag: response.headers.get("ETag") ?? undefined };
}
