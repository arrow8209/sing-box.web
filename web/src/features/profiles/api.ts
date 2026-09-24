import { request, type ApiError } from "@/api/client";
import type { AutoReloadOutcome, Draft, DraftInput, FreezeOperation, ProfileSummary } from "./types";
import { useCapabilitiesStore } from "@/features/capabilities/store";

interface DraftSaveResponse extends Draft {
  autoReload?: AutoReloadOutcome;
}

export interface ProfileApi {
  list(): Promise<ProfileSummary[]>;
  create(name: string): Promise<ProfileSummary>;
  duplicate(id: string): Promise<ProfileSummary>;
  rename(id: string, name: string): Promise<ProfileSummary>;
  remove(id: string, confirmName: string, stopCurrent: boolean): Promise<void>;
  getDraft(id: string): Promise<{ data: Draft; etag: string }>;
  saveDraft(id: string, etag: string, draft: Draft): Promise<{ data: Draft; etag: string; autoReload?: AutoReloadOutcome }>;
  freeze?(id: string, etag: string, operation: FreezeOperation): Promise<{ data: Draft; etag: string; autoReload?: AutoReloadOutcome }>;
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
  async duplicate(id) {
    return (await request<ProfileSummary>(`/profiles/${encodeURIComponent(id)}/copy`, { method: "POST" })).data;
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
    const useV2 = useCapabilitiesStore().capabilities.value?.serverReferenceResolution === true;
    const input: DraftInput = { buffers: draft.buffers, editorState: draft.editorState ?? {}, bindings: draft.bindings,
      ...(useV2 ? { schemaVersion: 2, ...(draft.references ? { references: draft.references } : {}) } : {}) };
    const response = await request<DraftSaveResponse>(`/profiles/${encodeURIComponent(id)}/draft`, {
      method: "PUT",
      headers: { "If-Match": etag },
      body: input
    });
    const { autoReload, ...data } = response.data;
    return { data, etag: requiredETag(response.etag), autoReload };
  },
  async freeze(id, etag, operation) {
    const response = await request<DraftSaveResponse>(`/profiles/${encodeURIComponent(id)}/draft`, {
      method: "PUT", headers: { "If-Match": etag }, body: { freeze: operation, schemaVersion: 2 }
    });
    const { autoReload, ...data } = response.data;
    return { data, etag: requiredETag(response.etag), autoReload };
  }
};

export function isApiError(error: unknown): error is ApiError {
  return typeof error === "object" && error !== null && "status" in error && "body" in error;
}
