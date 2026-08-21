<script setup lang="ts">
import type { CoreStatus } from "./types";
defineProps<{ status: CoreStatus | null }>();
function short(value: string): string { return value ? `${value.slice(0, 12)}${value.length > 12 ? "…" : ""}` : "—"; }
</script>

<template>
  <section class="card" aria-label="核心状态">
    <h2>核心状态</h2>
    <dl v-if="status">
      <dt>状态</dt><dd>{{ status.state }}</dd>
      <dt>版本</dt><dd>{{ status.installedVersion || "未安装" }}</dd>
      <dt v-if="status.installedDigest">摘要</dt><dd v-if="status.installedDigest" :title="status.installedDigest">{{ short(status.installedDigest) }}</dd>
      <dt v-if="status.pid">PID</dt><dd>PID {{ status.pid }}</dd>
      <dt v-if="status.currentProfileId">当前档案</dt><dd>{{ status.currentProfileId }}</dd>
      <dt v-if="status.startedAt">启动时间</dt><dd>{{ status.startedAt }}</dd>
      <dt v-if="status.lastError">最近错误</dt><dd class="error">{{ status.lastError }}</dd>
    </dl>
    <p v-else>正在读取核心状态…</p>
  </section>
</template>

<style scoped>
.card { padding: 20px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); } h2 { margin: 0 0 14px; font-size: 16px; } dl { display: grid; grid-template-columns: 100px 1fr; gap: 9px 14px; margin: 0; } dt { color: var(--text-faint); } dd { margin: 0; overflow-wrap: anywhere; } .error { color: var(--danger); }
</style>
