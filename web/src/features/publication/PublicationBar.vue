<script setup lang="ts">
import { computed, ref } from "vue";
import { APIClientError } from "../../api/client";
import { useDraftStore } from "../../stores/draft";
import { usePublicationStore } from "../../stores/publication";
import { useProfilesStore } from "../../stores/profiles";
import ModuleDiff from "./ModuleDiff.vue";
import ValidationIssueList from "../config/components/ValidationIssueList.vue";

const draft = useDraftStore();
const publication = usePublicationStore();
const profiles = useProfilesStore();

const announcement = ref("");
const emit = defineEmits<{
  goToField: [path: string];
}>();

const stateLabel = computed(() => {
  switch (draft.status) {
    case "dirty":
    case "saving":
    case "offline":
      return "草稿保存中";
    case "saved":
      return "草稿已保存";
    case "conflict":
      return "有冲突";
    default:
      return "与磁盘一致";
  }
});

const pendingChanges = computed(() => {
  const profile = profiles.active;
  return profile !== null && draft.draft !== null && draft.draft.revision !== profile.publishedRevision;
});

const errorMessage = computed(
  () => publication.lastError?.stderr || publication.lastError?.message || "",
);

const validationIssues = computed(() => {
  if (publication.lastError?.fieldIssues !== undefined) {
    return publication.lastError.fieldIssues;
  }
  return publication.lastValidation?.valid === false ? publication.lastValidation.issues : [];
});

const publishedConfig = ref<Record<string, unknown> | null>(null);

async function validate(): Promise<void> {
  announcement.value = "";
  try {
    const result = await publication.validate();
    if (result.valid) {
      announcement.value = "校验通过";
    } else {
      announcement.value = `校验失败，共 ${result.issues.length} 项`;
    }
  } catch {
    announcement.value = "";
  }
}

async function publish(): Promise<void> {
  announcement.value = "";
  try {
    await publication.publish();
    announcement.value = "正式配置已写入";
    publishedConfig.value = null;
  } catch (error) {
    if (
      error instanceof APIClientError &&
      error.detail.code === "external_drift" &&
      window.confirm("检测到正式配置已被外部修改。确认覆盖受管发布目录中的外部改动？")
    ) {
      try {
        await publication.publish(true);
        announcement.value = "正式配置已写入";
        publishedConfig.value = null;
      } catch {
        announcement.value = "";
      }
    }
  }
}

async function showDiff(): Promise<void> {
  const profile = profiles.active;
  if (profile === null) {
    return;
  }
  const response = await fetch(
    `/api/v1/profiles/${profile.id}/export?mode=redacted&source=published`,
    { credentials: "same-origin" },
  );
  if (!response.ok) {
    return;
  }
  publishedConfig.value = (await response.json()) as Record<string, unknown>;
}

async function copyRedacted(): Promise<void> {
  await publication.copyRedacted();
  announcement.value = "脱敏配置已复制";
}

async function downloadFull(): Promise<void> {
  await publication.downloadFull(
    () =>
      window.confirm("完整配置包含密码、Token、Secret、UUID 和私钥，确认下载？")
        ? Promise.resolve(true)
        : Promise.resolve(false),
  );
}
</script>

<template>
  <div class="publication-bar">
    <div class="publication-state">
      <span role="status">{{ announcement || stateLabel }}</span>
      <span v-if="pendingChanges" class="pending-flag">有待发布修改</span>
    </div>
    <div class="publication-actions">
      <button class="btn" name="仅校验" type="button" :disabled="publication.validating" @click="validate">
        仅校验
      </button>
      <button
        class="btn btn-primary"
        name="校验并写入"
        type="button"
        :disabled="!publication.canPublish || publication.publishing"
        :title="publication.publishBlockedReason"
        @click="publish"
      >
        校验并写入
      </button>
      <button class="btn" name="复制脱敏配置" type="button" @click="copyRedacted">复制脱敏配置</button>
      <button class="btn" name="下载完整配置" type="button" @click="downloadFull">下载完整配置</button>
      <button class="btn" name="显示模块差异" type="button" @click="showDiff">显示模块差异</button>
    </div>

    <p v-if="errorMessage" class="publication-error" role="alert">{{ errorMessage }}</p>

    <ValidationIssueList
      v-if="validationIssues.length > 0"
      :issues="validationIssues"
      @go-to-field="emit('goToField', $event)"
    />

    <ModuleDiff
      v-if="publishedConfig !== null && draft.draft !== null"
      :draft="draft.draft.content.config"
      :published="publishedConfig"
    />
  </div>
</template>

<style scoped>
.publication-bar {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 0;
  border-bottom: 1px solid var(--line);
}

.publication-state {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text-soft);
  font-size: 13px;
}

.pending-flag {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--orange-soft);
  color: var(--orange);
  font-size: 11px;
}

.publication-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.publication-error {
  margin: 0;
  color: var(--red);
  font-size: 13px;
}
</style>
