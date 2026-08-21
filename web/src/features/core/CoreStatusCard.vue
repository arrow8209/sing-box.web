<script setup lang="ts">
import AppIcon from "@/layout/AppIcon.vue";
import type { CoreStatus } from "./types";
defineProps<{ status: CoreStatus | null }>();
function short(value: string): string { return value ? `${value.slice(0, 12)}${value.length > 12 ? "…" : ""}` : "—"; }
</script>

<template>
  <section class="core-status-card" aria-label="核心状态">
    <header><span class="status-icon"><AppIcon name="cpu" /></span><div><h2>核心状态</h2><p>由 Web 服务作为唯一父进程管理</p></div><span v-if="status" class="state" :class="{ running: status.state === 'running', warning: status.state !== 'running' }">{{ status.state }}</span></header>
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
.core-status-card { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.core-status-card header { display: flex; align-items: center; gap: 10px; padding: 15px 16px; border-bottom: 1px solid var(--line); }.status-icon { display: grid; width: 34px; height: 34px; place-items: center; border-radius: 10px; color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.core-status-card header div { min-width: 0; flex: 1; }.core-status-card h2 { margin: 0; font-size: 14px; }.core-status-card header p { margin: 2px 0 0; color: var(--text-faint); font-size: 10px; }.state { display: inline-flex; align-items: center; gap: 5px; padding: 4px 8px; border-radius: 999px; color: var(--orange); background: var(--orange-soft); font-size: 10px; font-weight: 670; }.state::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }.state.running { color: var(--green); background: var(--green-soft); } dl { display: grid; grid-template-columns: 110px 1fr; gap: 0; margin: 0; padding: 5px 16px 12px; } dt, dd { min-height: 39px; padding: 10px 0; border-bottom: 1px solid var(--line); } dt:last-of-type, dd:last-child { border-bottom: 0; } dt { color: var(--text-faint); font-size: 11px; } dd { margin: 0; overflow-wrap: anywhere; font-size: 11px; font-weight: 620; } .error { color: var(--red); }
</style>
