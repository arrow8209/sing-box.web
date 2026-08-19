<script setup lang="ts">
import { computed, ref } from "vue";
import { useProfilesStore } from "../../stores/profiles";
import { useDraftStore } from "../../stores/draft";
import CreateProfileDialog from "./CreateProfileDialog.vue";
import RenameProfileDialog from "./RenameProfileDialog.vue";
import DeleteProfileDialog from "./DeleteProfileDialog.vue";

const profiles = useProfilesStore();
const draft = useDraftStore();

const showCreateDialog = ref(false);
const showRenameDialog = ref(false);
const showDeleteDialog = ref(false);

const saveStatusText = computed(() => {
  switch (draft.status) {
    case "saving":
    case "dirty":
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

async function selectProfile(id: string): Promise<void> {
  await profiles.select(id);
}

async function copyText(text: string): Promise<void> {
  await navigator.clipboard.writeText(text);
}

async function reloadAfterConflict(): Promise<void> {
  await draft.reloadAfterConflict();
}

async function overwriteAfterConflict(): Promise<void> {
  await draft.overwriteAfterConflict();
}
</script>

<template>
  <div class="profile-sidebar">
    <div class="profile-sidebar-header">
      <button class="btn btn-primary btn-sm" name="新建档案" @click="showCreateDialog = true">
        新建档案
      </button>
    </div>

    <p class="sr-only" role="status">{{ saveStatusText }}</p>

    <ul class="profile-list">
      <li v-for="profile in profiles.items" :key="profile.id" class="profile-item">
        <button
          class="profile-entry"
          :class="{ active: profile.id === profiles.activeId }"
          @click="selectProfile(profile.id)"
        >
          <span class="profile-name">{{ profile.name }}</span>
          <span class="profile-slug">{{ profile.slug }}</span>
          <span class="profile-badges">
            <span class="badge">草稿 r{{ profile.draftRevision }}</span>
            <span v-if="profile.publishedRevision !== null" class="badge badge-published">
              已发布 r{{ profile.publishedRevision }}
            </span>
          </span>
        </button>
        <button
          v-if="profile.id === profiles.activeId"
          class="profile-action"
          name="重命名档案"
          @click="showRenameDialog = true"
        >
          重命名档案
        </button>
        <button
          v-if="profile.id === profiles.activeId"
          class="profile-action"
          name="删除档案"
          @click="showDeleteDialog = true"
        >
          删除档案
        </button>
      </li>
    </ul>

    <div v-if="profiles.active?.draftReadOnly" class="draft-repair-message" role="alert">
      {{ profiles.active.draftError }}
    </div>

    <section v-if="profiles.active" class="profile-hints">
      <div class="hint-row">
        <code class="hint-code">{{ profiles.active.directoryHint }}</code>
        <button class="btn btn-sm" name="复制" @click="copyText(profiles.active!.directoryHint)">
          复制
        </button>
      </div>
      <div class="hint-row">
        <code class="hint-code">{{ profiles.active.checkCommand }}</code>
        <button class="btn btn-sm" name="复制" @click="copyText(profiles.active!.checkCommand)">
          复制
        </button>
      </div>
    </section>

    <CreateProfileDialog v-if="showCreateDialog" @close="showCreateDialog = false" />
    <RenameProfileDialog
      v-if="showRenameDialog && profiles.activeId !== null"
      :profile-id="profiles.activeId"
      @close="showRenameDialog = false"
    />
    <DeleteProfileDialog
      v-if="showDeleteDialog && profiles.activeId !== null"
      :profile-id="profiles.activeId"
      @close="showDeleteDialog = false"
    />

    <!-- 草稿冲突 -->
    <div
      v-if="draft.conflict !== null && draft.status === 'conflict'"
      class="modal-backdrop"
      role="dialog"
      aria-label="草稿冲突"
      aria-modal="true"
    >
      <div class="modal-card">
        <h2 class="modal-title">草稿冲突</h2>
        <p class="modal-copy">
          服务器上的草稿修订为 r{{ draft.conflict.serverRevision }}，与本地内容不一致。
        </p>
        <div class="modal-actions">
          <button class="btn" name="重新载入服务器版本" type="button" @click="reloadAfterConflict">
            重新载入服务器版本
          </button>
          <button class="btn btn-primary" name="用本地内容覆盖" type="button" @click="overwriteAfterConflict">
            用本地内容覆盖
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.profile-sidebar {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
}

.profile-sidebar-header {
  display: flex;
  justify-content: flex-end;
}

.profile-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.profile-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.profile-entry {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  text-align: left;
  cursor: pointer;
  font: inherit;
}

.profile-entry.active {
  border-color: var(--primary);
  background: var(--primary-soft);
}

.profile-name {
  font-weight: 600;
}

.profile-slug {
  color: var(--text-faint);
  font-size: 12px;
  font-family: ui-monospace, monospace;
}

.profile-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.badge {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--surface-tint);
  color: var(--primary);
  font-size: 11px;
}

.badge-published {
  background: var(--green-soft);
  color: var(--green);
}

.profile-action {
  border: 0;
  background: transparent;
  color: var(--text-soft);
  cursor: pointer;
  font-size: 12px;
  white-space: nowrap;
}

.draft-repair-message {
  padding: 8px 10px;
  border: 1px solid var(--orange);
  border-radius: var(--radius-sm);
  background: var(--orange-soft);
  color: var(--orange);
  font-size: 13px;
}

.profile-hints {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hint-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hint-code {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 4px 8px;
  border-radius: 6px;
  background: var(--surface-soft);
  font-size: 12px;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgb(10 18 38 / 45%);
}

.modal-card {
  width: min(420px, 100%);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 22px;
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: var(--shadow-lg);
}

.modal-title {
  margin: 0;
  font-size: 17px;
  color: var(--text);
}

.modal-copy {
  margin: 0;
  color: var(--text-soft);
  font-size: 14px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
</style>
