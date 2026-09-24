export interface ApiErrorBody {
  code: string;
  message: string;
  stage?: string;
  fieldPath?: string;
  latestRevision?: number;
  details?: Record<string, unknown>;
}

export class ApiError extends Error {
  constructor(public status: number, public body: ApiErrorBody) {
    super(body.message);
  }
}

export interface Session {
  valid: boolean;
  expiresAt: string | null;
  csrfToken?: string;
}

export interface LoginResult {
  csrfToken: string;
}

export interface RequestOptions {
  signal?: AbortSignal;
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  headers?: HeadersInit;
  body?: unknown;
}

export interface ApiResponse<T> {
  data: T;
  etag?: string;
}
