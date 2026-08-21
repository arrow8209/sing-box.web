<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { profileApi } from "@/features/profiles/api";
import type { ProfileSummary } from "@/features/profiles/types";
import { ruleResourceApi } from "@/features/rule-resources/api";
import type { RuleResource } from "@/features/rule-resources/types";
import { useCoreStore } from "@/features/core/store";

const core = useCoreStore();
const profiles = ref<ProfileSummary[]>([]);
const rules = ref<RuleResource[]>([]);
const error = ref("");
const profileName = computed(() => profiles.value.find((item) => item.id === core.status.value?.currentProfileId)?.name || core.status.value?.currentProfileId || "—");
const needsApply = computed(() => profiles.value.filter((item) => item.needsApply).length);
const recentRule = computed(() => [...rules.value].filter((item) => item.lastSuccessAt).sort((a, b) => String(b.lastSuccessAt).localeCompare(String(a.lastSuccessAt)))[0]);
async function refreshSummaries(): Promise<void> {
  try { [profiles.value, rules.value] = await Promise.all([profileApi.list(), ruleResourceApi.list()]); error.value = ""; }
  catch (cause) { error.value = cause instanceof Error ? cause.message : "无法读取仪表盘摘要"; }
}
onMounted(() => { core.startPolling(); void refreshSummaries(); });
onBeforeUnmount(core.stopPolling);
</script>

<template>
  <section class="page"><header><h1>仪表盘</h1><p>以下内容均来自服务端当前状态。</p></header>
    <p v-if="error" class="error">{{ error }}</p>
    <div class="grid">
      <section class="card"><h2>档案</h2><strong>{{ profiles.length }}</strong><p>共 {{ profiles.length }} 个档案，{{ needsApply }} 个需要应用</p></section>
      <section class="card"><h2>规则资源</h2><p>最近成功：{{ recentRule?.lastSuccessAt || "暂无" }}</p><p>下次更新：{{ recentRule?.nextUpdateAt || "暂无" }}</p></section>
      <section class="card"><h2>核心</h2><p>版本：{{ core.status.value?.installedVersion || "未安装" }}</p><p>状态：{{ core.status.value?.state || "正在读取" }}</p><p v-if="core.status.value?.pid">PID {{ core.status.value.pid }}</p></section>
      <section class="card"><h2>当前运行</h2><p>当前档案：{{ profileName }}</p><p>最近错误：{{ core.status.value?.lastError || "无" }}</p></section>
    </div>
    <section class="phase-two"><h2>二期尚未接入</h2><p>运行流量、连接、策略组、测速和完整日志将在二期提供。</p></section>
  </section>
</template>

<style scoped>
.page { display: grid; gap: 18px; } header h1 { margin: 0; } header p { color: var(--text-soft); } .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; } .card, .phase-two { padding: 20px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); } h2 { margin: 0 0 12px; font-size: 16px; } strong { font-size: 30px; } p { margin: 7px 0; overflow-wrap: anywhere; } .error { color: var(--danger); } .phase-two { color: var(--text-soft); } @media (max-width: 640px) { .grid { grid-template-columns: 1fr; } }
</style>
