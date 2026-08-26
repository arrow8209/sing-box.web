import { request, type ApiError } from "@/api/client";
import type { Draft, DraftInput, ProfileSummary } from "./types";

export interface ProfileApi {
  list(): Promise<ProfileSummary[]>;
  create(name: string): Promise<ProfileSummary>;
  rename(id: string, name: string): Promise<ProfileSummary>;
  remove(id: string, confirmName: string, stopCurrent: boolean): Promise<void>;
  getDraft(id: string): Promise<{ data: Draft; etag: string }>;
  saveDraft(id: string, etag: string, draft: Draft): Promise<{ data: Draft; etag: string }>;
}

function requiredETag(etag: string | undefined): string {
  if (!etag) throw new Error("服务器未返回草稿 ETag");
  return etag;
}

export const profileApi: ProfileApi = {
  async list() {
    return (await request<ProfileSummary[]>("/profiles")).data;
  },
  async create(name) {
    return (await request<ProfileSummary>("/profiles", { method: "POST", body: { name } })).data;
  },
  async rename(id, name) {
    return (await request<ProfileSummary>(`/profiles/${encodeURIComponent(id)}`, { method: "PATCH", body: { name } })).data;
  },
  async remove(id, confirmName, stopCurrent) {
    await request<void>(`/profiles/${encodeURIComponent(id)}`, { method: "DELETE", body: { confirmName, stopCurrent } });
  },
  async getDraft(id) {
    const response = await request<Draft>(`/profiles/${encodeURIComponent(id)}/draft`);
    return { data: response.data, etag: requiredETag(response.etag) };
  },
  async saveDraft(id, etag, draft) {
    const input: DraftInput = { buffers: draft.buffers, editorState: draft.editorState ?? {}, bindings: draft.bindings };
    const response = await request<Draft>(`/profiles/${encodeURIComponent(id)}/draft`, {
      method: "PUT",
      headers: { "If-Match": etag },
      body: input
    });
    return { data: response.data, etag: requiredETag(response.etag) };
  }
};

export function isApiError(error: unknown): error is ApiError {
  return typeof error === "object" && error !== null && "status" in error && "body" in error;
}
