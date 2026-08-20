import { defineStore } from "pinia";
import { apiClient, APIClientError } from "../api/client";
import { useDraftStore } from "./draft";
import { useProfilesStore } from "./profiles";
import { useCapabilitiesStore } from "./capabilities";
import type { APIError, PublicationReceipt, ValidationResult } from "../api/types";

interface PublicationState {
  lastValidation: ValidationResult | null;
  lastReceipt: PublicationReceipt | null;
  lastError: APIError | null;
  validating: boolean;
  publishing: boolean;
}

export const usePublicationStore = defineStore("publication", {
  state: (): PublicationState => ({
    lastValidation: null,
    lastReceipt: null,
    lastError: null,
    validating: false,
    publishing: false,
  }),
  getters: {
    canPublish(): boolean {
      const capabilities = useCapabilitiesStore();
      return capabilities.can("config.publish");
    },
    publishBlockedReason(): string {
      const capabilities = useCapabilitiesStore();
      return capabilities.snapshot?.singBox.warning ?? "";
    },
  },
  actions: {
    async validate(): Promise<ValidationResult> {
      const profiles = useProfilesStore();
      const profile = profiles.active;
      if (profile === null) {
        throw new Error("未选择配置档案");
      }
      this.validating = true;
      this.lastError = null;
      try {
        const draft = useDraftStore();
        const saved = await draft.save();
        if (
          !saved ||
          draft.status !== "saved" ||
          draft.draft?.profileId !== profile.id ||
          draft.etag === null ||
          profiles.activeId !== profile.id
        ) {
          const detail = draft.lastError ?? {
            code: "draft_not_saved",
            message: "草稿尚未保存，无法校验",
          };
          this.lastError = detail;
          throw new Error(detail.message);
        }
        const { data } = await apiClient.request<ValidationResult>(
          `/profiles/${profile.id}/validate`,
          { method: "POST", body: JSON.stringify({}) },
        );
        this.lastValidation = data;
        return data;
      } catch (error) {
        if (this.lastError === null) {
          this.lastError =
            error instanceof APIClientError
              ? error.detail
              : { code: "network_error", message: "校验请求失败" };
        }
        throw error;
      } finally {
        this.validating = false;
      }
    },
    async publish(confirmExternalOverwrite = false): Promise<PublicationReceipt> {
      const profiles = useProfilesStore();
      const profile = profiles.active;
      if (profile === null) {
        throw new Error("未选择配置档案");
      }
      if (!this.canPublish) {
        throw new Error(this.publishBlockedReason || "配置发布已禁用");
      }
      const draft = useDraftStore();
      this.publishing = true;
      this.lastError = null;
      try {
        const saved = await draft.save();
        if (
          !saved ||
          draft.status !== "saved" ||
          draft.draft?.profileId !== profile.id ||
          draft.etag === null ||
          profiles.activeId !== profile.id
        ) {
          const detail = draft.lastError ?? {
            code: "draft_not_saved",
            message: "草稿尚未保存，无法发布",
          };
          this.lastError = detail;
          throw new Error(detail.message);
        }
        const { data } = await apiClient.request<PublicationReceipt>(
          `/profiles/${profile.id}/publish`,
          {
            method: "POST",
            headers: { "If-Match": draft.etag },
            body: JSON.stringify({ confirmExternalOverwrite }),
          },
        );
        this.lastReceipt = data;
        this.lastValidation = data.validation;
        // 同步档案发布状态。
        const index = profiles.items.findIndex((item) => item.id === profile.id);
        if (index !== -1) {
          profiles.items[index] = {
            ...profiles.items[index],
            publishedRevision: data.revision,
            publishedDigest: data.digest,
            publishedAt: data.publishedAt,
          };
        }
        return data;
      } catch (error) {
        if (this.lastError === null) {
          this.lastError =
            error instanceof APIClientError
              ? error.detail
              : { code: "network_error", message: "发布请求失败" };
        }
        // 失败时绝不自动重试；草稿保持不变。
        throw error;
      } finally {
        this.publishing = false;
      }
    },
    async downloadFull(confirm: () => Promise<boolean>): Promise<void> {
      const profiles = useProfilesStore();
      const profile = profiles.active;
      if (profile === null) {
        return;
      }
      if (!(await confirm())) {
        return;
      }
      const url = `/api/v1/profiles/${profile.id}/export?mode=full&source=published&download=1`;
      const response = await fetch(url, { credentials: "same-origin" });
      if (!response.ok) {
        throw new APIClientError(response.status, {
          code: "export_failed",
          message: "完整配置下载失败",
        });
      }
      const blob = await response.blob();
      const objectUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = objectUrl;
      link.download = `${profile.slug}-config.json`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(objectUrl);
    },
    async copyRedacted(): Promise<void> {
      const profiles = useProfilesStore();
      const profile = profiles.active;
      if (profile === null) {
        return;
      }
      const url = `/api/v1/profiles/${profile.id}/export?mode=redacted&source=published`;
      const response = await fetch(url, { credentials: "same-origin" });
      if (!response.ok) {
        throw new APIClientError(response.status, {
          code: "export_failed",
          message: "脱敏配置获取失败",
        });
      }
      const text = await response.text();
      await navigator.clipboard.writeText(text);
    },
  },
});
