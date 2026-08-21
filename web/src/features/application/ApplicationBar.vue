<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useProfileStore } from "@/features/profiles/store";
import { stageLabels, type ApplicationError } from "./types";
import { useApplicationStore } from "./store";
const props = withDefaults(defineProps<{ bufferErrors?: string[] }>(), { bufferErrors: () => [] });
const emit = defineEmits<{ preview: []; validate: []; apply: []; error: [ApplicationError] }>();
const profiles = useProfileStore(); const app = useApplicationStore();
const reason = computed(() => { if (!profiles.activeProfileId.value) return "请先选择档案"; if (profiles.saveState.value !== "saved") return "请先等待草稿保存"; if (props.bufferErrors.length) return "请先修复编辑器错误"; if (app.core.value?.state === "not_installed") return "尚未安装 sing-box 核心"; if (app.core.value?.operation.active) return "已有核心操作正在进行"; return ""; });
const disabled = computed(() => Boolean(reason.value) || app.applying.value);
onMounted(() => { void app.refreshCore().catch(() => undefined); });
function apply(): void { if (!disabled.value) emit("apply"); }
</script>
<template><section class="application-bar"><div><strong>应用配置</strong><small v-if="reason">{{ reason }}</small><small v-else-if="app.core.value">核心：{{ app.core.value.state }}</small></div><div class="actions"><button data-preview type="button" class="secondary" :disabled="profiles.saveState.value !== 'saved'" @click="emit('preview')">预览</button><button data-validate type="button" class="secondary" :disabled="profiles.saveState.value !== 'saved'" @click="emit('validate')">检查</button><button data-apply type="button" :disabled="disabled" @click="apply">应用</button></div><p v-if="app.applying" class="stage">{{ stageLabels[app.core.value?.operation.stage ?? 'snapshot'] ?? "正在应用" }}</p><p v-if="app.reconnecting" class="stage">正在重新读取服务器真实状态</p></section></template>
<style scoped>.application-bar { display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; gap:12px; margin:18px 0; padding:12px 14px; border:1px solid var(--line); border-radius:var(--radius); background:var(--surface); }.application-bar div:first-child { display:grid; gap:3px; }.application-bar small,.stage { color:var(--text-soft); }.actions { display:flex; gap:8px; }.secondary { color:var(--text); background:var(--surface-soft); }.stage { width:100%; margin:0; }</style>
