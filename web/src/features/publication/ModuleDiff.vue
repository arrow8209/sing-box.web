<script lang="ts">
import { isSensitiveKey } from "../config/modules";

export interface ModuleDiffEntry {
  key: string;
  status: "added" | "changed" | "removed" | "unchanged";
  sensitiveNote?: string;
}

function stableJSON(value: unknown): string {
  const seen = new WeakSet<object>();
  function serialize(current: unknown): string {
    if (current === null || typeof current !== "object") {
      return JSON.stringify(current);
    }
    if (seen.has(current as object)) {
      return '"<circular>"';
    }
    seen.add(current as object);
    if (Array.isArray(current)) {
      return `[${current.map(serialize).join(",")}]`;
    }
    const keys = Object.keys(current as Record<string, unknown>).sort();
    return `{${keys
      .map((key) => `${JSON.stringify(key)}:${serialize((current as Record<string, unknown>)[key])}`)
      .join(",")}}`;
  }
  return serialize(value);
}

function containsSensitiveDiff(draftValue: unknown, publishedValue: unknown): boolean {
  if (draftValue === null || typeof draftValue !== "object" || publishedValue === null || typeof publishedValue !== "object") {
    return false;
  }
  for (const [key, value] of Object.entries(draftValue as Record<string, unknown>)) {
    if (isSensitiveKey(key)) {
      const other = (publishedValue as Record<string, unknown>)[key];
      if (stableJSON(value) !== stableJSON(other)) {
        return true;
      }
    }
    if (value !== null && typeof value === "object") {
      if (containsSensitiveDiff(value, (publishedValue as Record<string, unknown>)[key])) {
        return true;
      }
    }
  }
  return false;
}

export function compareModules(
  draft: Record<string, unknown>,
  published: Record<string, unknown>,
): ModuleDiffEntry[] {
  const keys = new Set([...Object.keys(draft), ...Object.keys(published)]);
  const entries: ModuleDiffEntry[] = [];
  for (const key of [...keys].sort()) {
    const draftValue = draft[key];
    const publishedValue = published[key];
    if (draftValue === undefined) {
      entries.push({ key, status: "removed" });
      continue;
    }
    if (publishedValue === undefined) {
      entries.push({ key, status: "added" });
      continue;
    }
    if (stableJSON(draftValue) === stableJSON(publishedValue)) {
      entries.push({ key, status: "unchanged" });
      continue;
    }
    if (containsSensitiveDiff(draftValue, publishedValue)) {
      entries.push({ key, status: "changed", sensitiveNote: "已修改敏感字段" });
    } else {
      entries.push({ key, status: "changed" });
    }
  }
  return entries;
}
</script>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  draft: Record<string, unknown>;
  published: Record<string, unknown>;
}>();

const entries = computed(() => compareModules(props.draft, props.published));

const statusText: Record<string, string> = {
  added: "新增",
  changed: "已修改",
  removed: "已移除",
  unchanged: "未变化",
};
</script>

<template>
  <ul class="module-diff" aria-label="模块差异">
    <li v-for="entry in entries" :key="entry.key" class="diff-row" :class="entry.status">
      <span class="diff-key">{{ entry.key }}</span>
      <span class="diff-status">{{ statusText[entry.status] }}</span>
      <span v-if="entry.sensitiveNote" class="diff-sensitive">{{ entry.sensitiveNote }}</span>
    </li>
  </ul>
</template>

<style scoped>
.module-diff {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.diff-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  font-size: 13px;
}

.diff-key {
  font-family: ui-monospace, "SF Mono", monospace;
  font-weight: 600;
}

.diff-status {
  color: var(--text-soft);
}

.diff-row.changed .diff-status {
  color: var(--orange);
}

.diff-row.added .diff-status {
  color: var(--green);
}

.diff-row.removed .diff-status {
  color: var(--red);
}

.diff-sensitive {
  color: var(--violet);
  font-size: 12px;
}
</style>
