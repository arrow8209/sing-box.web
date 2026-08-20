<script setup lang="ts">
import type { ValidationIssue } from "../../../api/types";

defineProps<{
  issues: ValidationIssue[];
}>();

const emit = defineEmits<{
  goToField: [path: string];
}>();
</script>

<template>
  <ul class="issue-list" aria-label="校验问题">
    <li v-for="(issue, index) in issues" :key="index" class="issue-item">
      <span class="issue-stage">{{ issue.stage }}</span>
      <a
        class="issue-link"
        href="#"
        role="link"
        :aria-label="`转到字段 ${issue.path}`"
        @click.prevent="emit('goToField', issue.path)"
      >
        {{ issue.message }}
      </a>
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
