<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import { useCoreStore } from "./store";
import CoreUpdateDialog from "./CoreUpdateDialog.vue";
import CoreVersionPanel from "./CoreVersionPanel.vue";

const core = useCoreStore();
const dialogOpen = ref(false);
const submitting = ref(false);
const checkingLatest = ref(true);
onMounted(() => { core.startPolling(); void refreshLatest(); });
onBeforeUnmount(core.stopPolling);
async function refreshLatest(): Promise<void> {
  checkingLatest.value = true;
  await core.refreshLatest();
  checkingLatest.value = false;
}
function openDownload(): void { dialogOpen.value = true; }
async function confirm(version: string): Promise<void> {
  submitting.value = true;
  await core.startUpdate(version);
  submitting.value = false;
  if (!core.error.value) dialogOpen.value = false;
}
const stageLabels: Record<string, string> = { checking_release: "检查版本", downloading: "下载", verifying_checksum: "摘要校验", verifying_binary: "程序验证", checking_config: "配置预检", stopping_core: "停止核心", installing: "安装", restarting_core: "重启", completed: "完成", failed: "失败" };
</script>

<template>
  <section class="core-page"><header class="page-intro"><div><h1>核心管理</h1><p>查看 sing-box 核心版本，并按需下载最新稳定版。</p></div></header>
    <p v-if="core.error.value" class="error">{{ core.error.value }}</p>
    <CoreVersionPanel :status="core.status.value" :latest="core.latest.value" :checking="checkingLatest" :busy="submitting" :update-available="core.updateAvailable.value" @download="openDownload" @refresh="refreshLatest" />
    <section v-if="core.update.value && core.update.value.stage" class="operation" aria-live="polite"><span><AppIcon name="download" :size="17" /></span><div><h2>下载进度</h2><p>{{ stageLabels[core.update.value.stage] ?? core.update.value.stage }}{{ core.update.value.targetVersion ? ` · ${core.update.value.targetVersion}` : "" }}</p><small v-if="core.update.value.message">{{ core.update.value.message }}</small></div></section>
    <CoreUpdateDialog :open="dialogOpen" :current-version="core.status.value?.installedVersion ?? ''" :latest="core.latest.value" :busy="submitting" @close="dialogOpen = false" @confirm="confirm" />
  </section>
</template>

<style scoped>
.core-page { display: grid; gap: 14px; max-width: 980px; }
.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.4px; }
.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }
.operation { display: flex; align-items: center; gap: 11px; padding: 14px 16px; border: 1px solid color-mix(in srgb, var(--primary) 18%, var(--line)); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.operation > span { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }
.operation h2, .operation p, .operation small { margin: 0; }
.operation h2 { font-size: 11px; }
.operation p { margin-top: 2px; color: var(--text-soft); font-size: 10px; }
.operation small { display: block; margin-top: 2px; color: var(--text-faint); font-size: 9px; }
.error { margin: 0; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); }
</style>
