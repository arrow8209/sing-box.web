import { defineStore } from "pinia";
import { apiClient, APIClientError } from "../api/client";
import { useProfilesStore } from "./profiles";
import type { APIError, Draft } from "../api/types";

export interface DraftConflictState {
  localContent: Draft["content"];
  serverRevision: number;
}

type DraftStatus = "idle" | "loading" | "dirty" | "saving" | "saved" | "offline" | "conflict" | "error";

interface DraftState {
  draft: Draft | null;
  etag: string | null;
  status: DraftStatus;
  lastError: APIError | null;
  conflict: DraftConflictState | null;
}

const SAVE_DEBOUNCE_MS = 600;

const UNSAFE_BUFFER_PATH_SEGMENTS = new Set(["", ".", "..", "__proto__", "prototype", "constructor"]);

let saveTimer: ReturnType<typeof setTimeout> | null = null;

function cloneDraft(draft: Draft): Draft {
  return JSON.parse(JSON.stringify(draft)) as Draft;
}

function configPathValue(
  config: Record<string, unknown>,
  bufferKey: string,
): { exists: boolean; value?: unknown } {
  const segments = bufferKey.split("/");
  if (segments.some((segment) => UNSAFE_BUFFER_PATH_SEGMENTS.has(segment))) {
    return { exists: false };
  }
  let current: unknown = config;
  for (const segment of segments) {
    if (Array.isArray(current)) {
      if (!/^(0|[1-9]\d*)$/.test(segment)) {
        return { exists: false };
      }
      const index = Number(segment);
      if (!Number.isSafeInteger(index) || index >= current.length) {
        return { exists: false };
      }
      current = current[index];
      continue;
    }
    if (current === null || typeof current !== "object" || !Object.hasOwn(current, segment)) {
      return { exists: false };
    }
    current = (current as Record<string, unknown>)[segment];
  }
  return { exists: current !== undefined, value: current };
}

export const useDraftStore = defineStore("draft", {
  state: (): DraftState => ({
    draft: null,
    etag: null,
    status: "idle",
    lastError: null,
    conflict: null,
  }),
  actions: {
    async load(profileId: string): Promise<void> {
      const profiles = useProfilesStore();
      const profile = profiles.items.find((item) => item.id === profileId);
      if (profile?.draftReadOnly) {
        this.draft = null;
        this.etag = null;
        this.status = "error";
        this.lastError = {
          code: "draft_read_only",
          message: profile.draftError || "草稿只读，需要管理员在服务器修复",
        };
        return;
      }
      this.status = "loading";
      try {
        const { data, etag } = await apiClient.request<Draft>(`/profiles/${profileId}/draft`);
        this.draft = data;
        this.etag = etag;
        this.status = "saved";
        this.lastError = null;
        this.conflict = null;
      } catch (error) {
        this.status = "error";
        this.lastError =
          error instanceof APIClientError
            ? error.detail
            : { code: "network_error", message: "草稿载入失败" };
        throw error;
      }
    },
    replaceConfig(config: Record<string, unknown>, bufferRemap?: Record<string, string>): void {
      if (this.status === "error") {
        // 损坏草稿拒绝本地变更：绝不能从浏览器状态修复服务器文件。
        return;
      }
      if (this.draft === null) {
        return;
      }
      const next = cloneDraft(this.draft);
      const buffers = { ...next.content.editorBuffers };
      // 结构化变更同步再生所有已应用缓冲区的格式化文本，避免 canonical/buffer 分叉。
      for (const [module, buffer] of Object.entries(buffers)) {
        if (buffer.valid && buffer.applied) {
          const text = this.formatModule(module, config);
          if (text === undefined) {
            delete buffers[module];
          } else {
            buffers[module] = { ...buffer, text };
          }
        }
      }
      // 集合行重排/删除时同步迁移索引缓冲区键（module/0、module/1 …）。
      if (bufferRemap !== undefined) {
        for (const [from, to] of Object.entries(bufferRemap)) {
          const buffer = buffers[from];
          if (buffer !== undefined) {
            buffers[to] = buffer;
          }
          delete buffers[from];
        }
      }
      next.content = { ...next.content, config, editorBuffers: buffers };
      this.draft = next;
      this.status = "dirty";
      this.scheduleSave();
    },
    formatModule(module: string, config: Record<string, unknown>): string | undefined {
      // 缓冲区键与后端一致，使用顶层模块键（如 log、dns、experimental）；
      // 索引键（如 outbounds/0）按集合行格式化。
      const segments = module.split("/");
      const canonical = configPathValue(config, module);
      if (!canonical.exists) {
        return undefined;
      }
      if (segments.length === 1) {
        return JSON.stringify({ [module]: canonical.value }, null, 2);
      }
      return JSON.stringify(canonical.value, null, 2);
    },
    /** 单次变更同时更新规范配置与缓冲区（集合行编辑/重排/删除用，避免中间状态分叉）。 */
    updateConfigWithBuffers(
      config: Record<string, unknown>,
      buffers: Record<string, { text: string; valid: boolean; applied: boolean }>,
    ): void {
      if (this.status === "error" || this.draft === null) {
        return;
      }
      const next = cloneDraft(this.draft);
      next.content = { config, editorBuffers: { ...next.content.editorBuffers, ...buffers } };
      this.draft = next;
      this.status = "dirty";
      this.scheduleSave();
    },
    /** 用户选择放弃未应用 JSON：以规范配置重新生成缓冲区文本并标记有效已应用。 */
    discardUnappliedBuffer(module: string): void {
      if (this.draft === null || this.status === "error") {
        return;
      }
      const next = cloneDraft(this.draft);
      const buffers = { ...next.content.editorBuffers };
      const text = this.formatModule(module, next.content.config);
      if (text === undefined) {
        delete buffers[module];
      } else {
        buffers[module] = { text, valid: true, applied: true };
      }
      next.content = { ...next.content, editorBuffers: buffers };
      this.draft = next;
      this.status = "dirty";
      this.scheduleSave();
    },
    setEditorBuffer(module: string, text: string): void {
      if (this.status === "error" || this.draft === null) {
        return;
      }
      const next = cloneDraft(this.draft);
      const buffers = { ...next.content.editorBuffers };
      const previous = buffers[module];
      buffers[module] = {
        text,
        valid: false,
        applied: previous?.applied ?? false,
      };
      next.content = { ...next.content, editorBuffers: buffers };
      this.draft = next;
      this.status = "dirty";
      this.scheduleSave();
    },
    applyEditorBuffer(module: string, text: string): void {
      if (this.status === "error" || this.draft === null) {
        return;
      }
      const next = cloneDraft(this.draft);
      const buffers = { ...next.content.editorBuffers };
      buffers[module] = {
        text,
        valid: true,
        applied: true,
      };
      next.content = { ...next.content, editorBuffers: buffers };
      this.draft = next;
      this.status = "dirty";
      this.scheduleSave();
    },
    async save(): Promise<void> {
      if (this.draft === null || this.etag === null) {
        return;
      }
      if (this.status === "conflict" || this.status === "error") {
        return;
      }
      const profiles = useProfilesStore();
      const profile = profiles.active;
      if (profile?.draftReadOnly) {
        this.status = "error";
        this.lastError = {
          code: "draft_read_only",
          message: profile.draftError || "草稿只读，需要管理员在服务器修复",
        };
        return;
      }
      const profileId = this.draft.profileId;
      const content = cloneDraft(this.draft).content;
      this.status = "saving";
      try {
        const { data, etag } = await apiClient.request<Draft>(`/profiles/${profileId}/draft`, {
          method: "PUT",
          headers: { "If-Match": this.etag },
          body: JSON.stringify(content),
        });
        this.draft = data;
        this.etag = etag;
        this.status = "saved";
        this.lastError = null;
      } catch (error) {
        if (error instanceof APIClientError && error.status === 409) {
          this.status = "conflict";
          this.conflict = {
            localContent: content,
            serverRevision: error.detail.currentRevision ?? 0,
          };
          this.lastError = error.detail;
          return;
        }
        this.status = "offline";
        this.lastError =
          error instanceof APIClientError
            ? error.detail
            : { code: "network_error", message: "草稿保存失败" };
      }
    },
    async reloadAfterConflict(): Promise<void> {
      if (this.conflict === null || this.draft === null) {
        return;
      }
      const profileId = this.draft.profileId;
      this.conflict = null;
      await this.load(profileId);
    },
    async overwriteAfterConflict(): Promise<void> {
      if (this.conflict === null || this.draft === null) {
        return;
      }
      const profileId = this.draft.profileId;
      const localContent = cloneDraft(this.draft);
      localContent.content = this.conflict.localContent;
      const latest = await apiClient.request<Draft>(`/profiles/${profileId}/draft`);
      const headers: Record<string, string> = {};
      if (latest.etag !== null) {
        headers["If-Match"] = latest.etag;
      }
      const { data, etag } = await apiClient.request<Draft>(`/profiles/${profileId}/draft`, {
        method: "PUT",
        headers,
        body: JSON.stringify(this.conflict.localContent),
      });
      this.draft = data;
      this.etag = etag;
      this.status = "saved";
      this.conflict = null;
      this.lastError = null;
    },
    scheduleSave(): void {
      if (saveTimer !== null) {
        clearTimeout(saveTimer);
      }
      saveTimer = setTimeout(() => {
        saveTimer = null;
        void this.save();
      }, SAVE_DEBOUNCE_MS);
    },
  },
});
