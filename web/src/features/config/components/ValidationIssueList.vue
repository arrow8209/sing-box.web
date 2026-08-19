<script setup lang="ts">
import { useRouter } from "vue-router";
import { jsonPointerToModule } from "../modules";
import type { ValidationIssue } from "../../../api/types";

const props = defineProps<{
  issues: ValidationIssue[];
  profileId: string;
}>();

const router = useRouter();

async function goToField(issue: ValidationIssue): Promise<void> {
  const module = jsonPointerToModule(issue.path);
  if (module === null) {
    return;
  }
  await router.push(`/config/${props.profileId}/${module}`);
  // 聚焦最近的 data-json-path 元素。
  await new Promise((resolve) => setTimeout(resolve, 0));
  const target = document.querySelector<HTMLElement>(
    `[data-json-path="${CSS.escape(issue.path)}"]`,
  );
  target?.focus();
}
</script>

<template>
  <ul class="issue-list" aria-label="校验问题">
    <li v-for="(issue, index) in issues" :key="index" class="issue-item">
      <span class="issue-stage">{{ issue.stage }}</span>
      <button class="issue-link" type="button" :name="`转到字段 ${issue.path}`" @click="goToField(issue)">
        {{ issue.message }}
      </button>
      <code class="issue-path">{{ issue.path }}</code>
    </li>
  </ul>
</template>

<style scoped>
.issue-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.issue-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
}

.issue-stage {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--surface-tint);
  color: var(--primary);
  font-size: 11px;
  white-space: nowrap;
}

.issue-link {
  border: 0;
  background: transparent;
  padding: 0;
  color: var(--red);
  cursor: pointer;
  font: inherit;
  text-align: left;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.issue-path {
  color: var(--text-faint);
  font-size: 11px;
}
</style>
