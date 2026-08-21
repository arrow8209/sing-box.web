<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { useCoreStore } from "./store";
import CoreStatusCard from "./CoreStatusCard.vue";
import CoreUpdateDialog from "./CoreUpdateDialog.vue";

const core = useCoreStore();
const dialogOpen = ref(false);
const submitting = ref(false);
onMounted(() => { core.startPolling(); void core.refreshLatest(); });
onBeforeUnmount(core.stopPolling);
async function openUpdate(): Promise<void> { await core.refreshLatest(); dialogOpen.value = true; }
async function confirm(version: string): Promise<void> { submitting.value = true; await core.startUpdate(version); submitting.value = false; dialogOpen.value = false; }
const stageLabels: Record<string, string> = { checking_release: "检查版本", downloading: "下载", verifying_checksum: "摘要校验", verifying_binary: "程序验证", checking_config: "配置预检", stopping_core: "停止核心", installing: "安装", restarting_core: "重启", completed: "完成", failed: "失败" };
</script>

<template>
  <section class="page"><header><div><h1>核心管理</h1><p>仅显示服务端返回的核心进程状态。</p></div><button data-open-update type="button" :disabled="submitting" @click="openUpdate">手动更新</button></header>
    <p v-if="core.error" class="error">{{ core.error }}</p>
    <section v-if="core.status.value?.state === 'not_installed'" class="missing"><h2>尚未安装 sing-box</h2><p>安装核心后仍可继续编辑档案和规则资源。</p><button data-download-core type="button" @click="openUpdate">手动下载</button></section>
    <CoreStatusCard :status="core.status.value" />
    <section v-if="core.update.value && core.update.value.stage" class="operation"><h2>更新操作</h2><p>{{ stageLabels[core.update.value.stage] ?? core.update.value.stage }}{{ core.update.value.targetVersion ? `：${core.update.value.targetVersion}` : "" }}</p><p v-if="core.update.value.message">{{ core.update.value.message }}</p></section>
    <CoreUpdateDialog :open="dialogOpen" :current-version="core.status.value?.installedVersion ?? ''" :latest="core.latest.value" :busy="submitting" @close="dialogOpen = false" @confirm="confirm" />
  </section>
</template>

<style scoped>
.page { display: grid; gap: 18px; max-width: 860px; } header { display: flex; justify-content: space-between; align-items: start; gap: 16px; } h1, h2 { margin: 0; } header p { margin-bottom: 0; color: var(--text-soft); } .missing, .operation { padding: 20px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); } .missing h2, .operation h2 { margin-bottom: 8px; font-size: 17px; } .error { color: var(--danger); }
</style>
