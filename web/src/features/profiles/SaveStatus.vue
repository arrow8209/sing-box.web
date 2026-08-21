<script setup lang="ts">
import type { SaveState } from "./types";
import StatusBadge from "@/components/StatusBadge.vue";
defineProps<{ state: SaveState; needsApply?: boolean }>();
const labels: Record<SaveState, string> = { idle: "", dirty: "有未保存修改", saving: "正在保存草稿", saved: "草稿已保存", offline: "离线，等待恢复", conflict: "草稿版本冲突", error: "保存失败，可重试" };
</script>

<template><p v-if="state !== 'idle'" class="status" aria-live="polite"><StatusBadge :label="labels[state]" :tone="state === 'saved' ? 'success' : ['offline', 'conflict', 'error'].includes(state) ? 'danger' : 'neutral'" /><StatusBadge v-if="needsApply" label="需要应用" tone="warning" /></p></template>

<style scoped>
.status { display: flex; align-items: center; gap: 8px; margin: 0; color: var(--text-soft); font-size: 13px; }.saved { color: var(--green); }.offline, .conflict, .error { color: var(--red); }.apply { padding: 2px 7px; border-radius: 99px; background: var(--orange-soft); color: #a45d08; font-weight: 700; }
</style>
