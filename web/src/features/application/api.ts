import { request } from "@/api/client";
import type { ApplyResult, CoreStatus, PreviewResult, ValidationResult } from "./types";

export interface ApplicationApi {
  preview(profileId: string, revision: number): Promise<PreviewResult>;
  validate(profileId: string, revision: number): Promise<ValidationResult>;
  apply(profileId: string, body: { revision: number; confirmedConfigDigest?: string }): Promise<ApplyResult>;
  core(): Promise<CoreStatus>;
}

export const applicationApi: ApplicationApi = {
  preview: async (id, revision) => (await request<PreviewResult>(`/profiles/${encodeURIComponent(id)}/preview`, { method: "POST", body: { revision } })).data,
  validate: async (id, revision) => (await request<ValidationResult>(`/profiles/${encodeURIComponent(id)}/validate`, { method: "POST", body: { revision } })).data,
  apply: async (id, body) => (await request<ApplyResult>(`/profiles/${encodeURIComponent(id)}/apply`, { method: "POST", body })).data,
  core: async () => (await request<CoreStatus>("/core")).data
};
