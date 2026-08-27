<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import { useCoreStore } from "./store";
import CoreVersionPanel from "./CoreVersionPanel.vue";

const core = useCoreStore();
const submitting = ref(false);
const stopping = ref(false);
const checkingLatest = ref(true);
const operationActive = computed(() => Boolean(core.update.value?.stage && !["idle", "completed", "failed", "cancelled"].includes(core.update.value.stage)));

onMounted(() => {
  core.startPolling();
  void refreshLatest();
});
onBeforeUnmount(core.stopPolling);

async function refreshLatest(): Promise<void> {
  checkingLatest.value = true;
  await core.refreshLatest();
  checkingLatest.value = false;
}

async function downloadLatest(version: string): Promise<void> {
  submitting.value = true;
  try {
    await core.startUpdate(version);
  } finally {
    submitting.value = false;
  }
}

async function stopDownload(): Promise<void> {
  stopping.value = true;
  try {
    await core.cancelUpdate();
  } finally {
    stopping.value = false;
  }
}
</script>

<template>
  <section class="core-management-page">
    <header class="page-intro">
      <h1>核心管理</h1>
      <p>查看 sing-box 版本，并按需下载最新稳定版本。</p>
    </header>

    <p v-if="core.error.value" class="core-error" role="alert"><AppIcon name="info" :size="15" />{{ core.error.value }}</p>

    <CoreVersionPanel
      :status="core.status.value"
      :latest="core.latest.value"
      :update="core.update.value"
      :checking="checkingLatest"
      :busy="submitting"
      :stopping="stopping"
      :update-available="core.updateAvailable.value"
      @download="downloadLatest"
      @refresh="refreshLatest"
      @stop="stopDownload"
    />

    <p v-if="core.status.value?.lastError && !operationActive" class="core-runtime-error" role="alert"><AppIcon name="info" :size="14" /><span><strong>最近一次核心错误</strong>{{ core.status.value.lastError }}</span></p>
    <p class="core-page-note"><AppIcon name="info" :size="14" /><span>点击下载后会立即开始；安装前会校验文件摘要和现有配置，运行中的核心会在成功后自动恢复。</span></p>
  </section>
</template>

<style scoped>
.core-management-page { display: grid; max-width: 1040px; gap: 14px; }
.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.4px; }
.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }
.core-error, .core-runtime-error { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: 10px; }
.core-runtime-error span, .core-runtime-error strong { display: block; }
.core-runtime-error strong { margin-bottom: 2px; font-size: 10px; }
.core-page-note { display: flex; align-items: center; gap: 7px; margin: -2px 3px 0; color: var(--text-faint); font-size: 9px; }
.core-page-note :deep(.icon) { flex: 0 0 auto; color: var(--primary); }
</style>
