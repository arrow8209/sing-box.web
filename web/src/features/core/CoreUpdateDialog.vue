<script setup lang="ts">
import AppIcon from "@/layout/AppIcon.vue";
import type { LatestRelease } from "./types";
defineProps<{ open: boolean; currentVersion: string; latest: LatestRelease | null; busy: boolean }>();
const emit = defineEmits<{ close: []; confirm: [version: string] }>();
</script>

<template>
  <div v-if="open" class="backdrop" role="presentation" @click.self="emit('close')">
    <section class="dialog" role="dialog" aria-modal="true" aria-labelledby="update-title">
      <header><span><AppIcon name="download" :size="20" /></span><div><h2 id="update-title">下载最新版本</h2><p>确认要下载的核心版本</p></div></header>
      <div class="version-change">
        <div><span>当前版本</span><strong>{{ currentVersion || "尚未安装" }}</strong></div>
        <AppIcon name="chevron" :size="17" />
        <div class="target"><span>最新版本</span><strong>{{ latest?.version || "正在获取" }}</strong></div>
      </div>
      <p class="notice"><AppIcon name="info" :size="16" /><span>确认后开始下载最新稳定版，现有核心配置不会改变。</span></p>
      <div class="actions"><button class="cancel" type="button" @click="emit('close')">取消</button><button data-confirm-update type="button" :disabled="!latest || busy" @click="latest && emit('confirm', latest.version)"><AppIcon name="download" :size="15" />{{ busy ? "正在准备…" : "确认下载" }}</button></div>
    </section>
  </div>
</template>

<style scoped>
.backdrop { position: fixed; z-index: 30; inset: 0; display: grid; place-items: center; padding: 16px; background: rgb(10 18 33 / 52%); backdrop-filter: blur(2px); }
.dialog { width: min(450px, 100%); padding: 22px; border: 1px solid var(--line); border-radius: var(--radius-lg); color: var(--text); background: var(--surface); box-shadow: var(--shadow-lg); }
.dialog > header { display: flex; align-items: center; gap: 11px; }
.dialog > header > span { display: grid; width: 40px; height: 40px; flex: 0 0 auto; place-items: center; border-radius: 12px; color: var(--primary); background: var(--primary-soft); }
.dialog header h2, .dialog header p { margin: 0; }
.dialog header h2 { font-size: 17px; }
.dialog header p { margin-top: 2px; color: var(--text-faint); font-size: 10px; }
.version-change { display: grid; grid-template-columns: 1fr 28px 1fr; align-items: center; gap: 5px; margin-top: 20px; }
.version-change > div { min-width: 0; padding: 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }
.version-change > .icon { color: var(--text-faint); }
.version-change .target { border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); background: var(--primary-soft); }
.version-change span, .version-change strong { display: block; }
.version-change span { color: var(--text-faint); font-size: 9px; }
.version-change strong { overflow: hidden; margin-top: 4px; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.version-change .target strong { color: var(--primary-strong); }
.notice { display: flex; align-items: flex-start; gap: 8px; margin: 14px 0 0; padding: 10px 11px; border-radius: 9px; color: var(--primary); background: var(--primary-soft); font-size: 9px; }
.notice span { color: var(--text-soft); }
.actions { display: flex; justify-content: end; gap: 8px; margin-top: 21px; }
.actions button { display: inline-flex; min-height: 36px; align-items: center; justify-content: center; gap: 6px; padding: 7px 13px; font-size: 10px; }
.actions .cancel { color: var(--text); background: var(--surface-soft); }

@media (max-width: 540px) {
  .backdrop { align-items: end; padding: 0; }
  .dialog { width: 100%; padding: 20px 16px calc(20px + env(safe-area-inset-bottom)); border-radius: 20px 20px 0 0; }
  .actions button { flex: 1; }
}
</style>
