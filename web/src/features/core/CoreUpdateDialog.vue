<script setup lang="ts">
import type { LatestRelease } from "./types";
defineProps<{ open: boolean; currentVersion: string; latest: LatestRelease | null; busy: boolean }>();
const emit = defineEmits<{ close: []; confirm: [version: string] }>();
</script>

<template>
  <div v-if="open" class="backdrop" role="presentation">
    <section class="dialog" role="dialog" aria-modal="true" aria-labelledby="update-title">
      <h2 id="update-title">确认手动更新</h2>
      <p>当前版本：{{ currentVersion || "尚未安装" }}</p>
      <p>目标稳定版：{{ latest?.version || "正在获取" }}</p>
      <p class="notice">不会自动更新。确认后才会下载并安装该稳定版本。</p>
      <div class="actions"><button type="button" @click="emit('close')">取消</button><button data-confirm-update type="button" :disabled="!latest || busy" @click="latest && emit('confirm', latest.version)">{{ busy ? "正在提交" : "确认更新" }}</button></div>
    </section>
  </div>
</template>

<style scoped>
.backdrop { position: fixed; z-index: 30; inset: 0; display: grid; place-items: center; padding: 16px; background: rgb(0 0 0 / .42); } .dialog { width: min(460px, 100%); padding: 22px; border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-lg); } h2 { margin-top: 0; } .notice { color: var(--text-soft); } .actions { display: flex; justify-content: end; gap: 10px; margin-top: 22px; } .actions button:first-child { background: var(--surface-soft); color: var(--text); }
</style>
