<script setup lang="ts">
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { LatestRelease } from "./types";
withDefaults(defineProps<{ open: boolean; currentVersion: string; latest: LatestRelease | null; checking?: boolean; busy: boolean }>(), { checking: false });
const emit = defineEmits<{ close: []; confirm: [version: string] }>();
</script>

<template>
  <AppDialog class="core-update-dialog" :open="open" title="确认手动更新" description="下载并安装官方最新稳定版本" :close-disabled="busy" @close="emit('close')">
    <div class="version-change">
      <div><span>当前版本</span><strong>{{ currentVersion || "尚未安装" }}</strong></div>
      <AppIcon name="chevron" :size="17" />
      <div class="target"><span>目标稳定版</span><strong>{{ latest?.version || "正在获取" }}</strong></div>
    </div>
    <p class="notice"><AppIcon name="info" :size="16" /><span>确认后会下载并校验官方核心；如核心正在运行，安装期间会短暂停止并在成功后自动恢复。</span></p>
    <template #footer>
      <button class="cancel" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button data-confirm-update type="button" :disabled="!latest || checking || busy" @click="latest && !checking && emit('confirm', latest.version)"><AppIcon :name="checking ? 'rotate' : 'download'" :size="15" />{{ checking ? "正在检查版本…" : busy ? "正在提交…" : currentVersion ? "确认更新" : "确认下载" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.app-dialog.core-update-dialog) { width: min(450px, calc(100vw - 32px)); }
.version-change { display: grid; grid-template-columns: 1fr 28px 1fr; align-items: center; gap: 5px; }
.version-change > div { min-width: 0; padding: 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }
.version-change > :deep(.icon) { color: var(--text-faint); }
.version-change .target { border-color: color-mix(in srgb, var(--primary) 24%, var(--line)); background: var(--primary-soft); }
.version-change span, .version-change strong { display: block; }
.version-change span { color: var(--text-faint); font-size: 9px; }
.version-change strong { overflow: hidden; margin-top: 4px; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.version-change .target strong { color: var(--primary-strong); }
.notice { display: flex; align-items: flex-start; gap: 8px; margin: 14px 0 0; padding: 10px 11px; border-radius: 9px; color: var(--primary); background: var(--primary-soft); font-size: 9px; }
.notice span { color: var(--text-soft); }
.cancel, [data-confirm-update] { display: inline-flex; min-height: 36px; align-items: center; justify-content: center; gap: 6px; padding: 7px 13px; font-size: 10px; }
.cancel { color: var(--text); background: var(--surface-soft); }

@media (max-width: 540px) {
  :global(.app-dialog.core-update-dialog) { width: 100%; }
  .cancel, [data-confirm-update] { flex: 1; }
}
</style>
