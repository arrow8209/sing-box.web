<script setup lang="ts">
import { computed } from "vue";
import { useProfilesStore } from "../../stores/profiles";
import { useDraftStore } from "../../stores/draft";
import { useCapabilitiesStore } from "../../stores/capabilities";

const profiles = useProfilesStore();
const draft = useDraftStore();
const capabilities = useCapabilitiesStore();

const active = computed(() => profiles.active);

const draftStateLabel = computed(() => {
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

const validationLabel = computed(() => {
  const validation = active.value?.lastValidation;
  if (validation === null || validation === undefined) {
    return "尚未校验";
  }
  return `${validation.status}（r${validation.revision}）`;
});
</script>

<template>
  <section class="dashboard-view">
    <template v-if="active !== null">
      <div class="card-grid">
        <article class="health-card">
          <h3 class="card-title">配置档案</h3>
          <p class="card-value">{{ active.name }}</p>
          <p class="card-meta">{{ active.slug }} · 草稿 r{{ active.draftRevision }}</p>
        </article>
        <article class="health-card">
          <h3 class="card-title">草稿状态</h3>
          <p class="card-value">{{ draftStateLabel }}</p>
        </article>
        <article class="health-card">
          <h3 class="card-title">最近校验</h3>
          <p class="card-value">{{ validationLabel }}</p>
        </article>
        <article class="health-card">
          <h3 class="card-title">sing-box</h3>
          <p class="card-value">
            {{ capabilities.snapshot?.singBox.available ? capabilities.snapshot.singBox.version : "不可用" }}
          </p>
          <p class="card-meta">目标版本 {{ capabilities.snapshot?.singBox.targetVersion }}</p>
        </article>
      </div>
    </template>
    <p v-else class="empty-state">尚未创建配置档案，请在侧栏新建。</p>
  </section>
</template>

<style scoped>
.dashboard-view {
  padding: 20px;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 14px;
}

.health-card {
  padding: 16px;
  border: 1px solid var(--line);
  border-radius: var(--radius);
  background: var(--surface);
}

.card-title {
  margin: 0 0 8px;
  color: var(--text-soft);
  font-size: 12px;
}

.card-value {
  margin: 0;
  color: var(--text);
  font-size: 16px;
  font-weight: 600;
}

.card-meta {
  margin: 4px 0 0;
  color: var(--text-faint);
  font-size: 12px;
}

.empty-state {
  color: var(--text-soft);
}
</style>
