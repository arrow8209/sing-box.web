<script setup lang="ts">
import { reactive, watch } from "vue";
import type { RuleResource, RuleResourceInput } from "./types";
const props = defineProps<{ open: boolean; resource?: RuleResource | null }>();
const emit = defineEmits<{ close: []; submit: [input: RuleResourceInput] }>();
const form = reactive<RuleResourceInput>({ name: "", url: "", format: "binary", updateInterval: "", disabled: false });
function reset(): void { Object.assign(form, props.resource ? { name: props.resource.name, url: props.resource.url, format: props.resource.format, updateInterval: props.resource.updateInterval, disabled: props.resource.disabled } : { name: "", url: "", format: "binary", updateInterval: "", disabled: false }); }
watch(() => [props.open, props.resource] as const, reset, { immediate: true });
function submit(): void { emit("submit", { ...form }); }
</script>
<template>
  <div v-if="open" class="backdrop" role="presentation"><form class="dialog" @submit.prevent="submit"><header><h2>{{ resource ? "编辑规则资源" : "新增规则资源" }}</h2><button type="button" aria-label="关闭" @click="emit('close')">×</button></header><label>名称<input v-model.trim="form.name" required /></label><label>下载地址<input v-model.trim="form.url" type="url" required placeholder="https://…" /></label><label>格式<select v-model="form.format"><option value="binary">binary (.srs)</option><option value="source">source (.json)</option></select></label><label>更新周期<input v-model.trim="form.updateInterval" placeholder="例如 6h；留空使用手动更新" /></label><label class="checkbox"><input v-model="form.disabled" type="checkbox" /> 禁用此资源</label><p v-if="resource" class="hint">固定路径：<code>{{ resource.localPath }}</code>（由服务端管理，只读）</p><footer><button type="button" @click="emit('close')">取消</button><button type="submit">保存</button></footer></form></div>
</template>
<style scoped>
.backdrop { position: fixed; z-index: 30; inset: 0; display: grid; place-items: center; padding: 16px; background: rgb(10 18 30 / .48); }.dialog { display: grid; width: min(500px, 100%); gap: 13px; padding: 20px; border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-lg); }.dialog header, footer { display: flex; justify-content: space-between; align-items: center; gap: 10px; }.dialog h2 { margin: 0; }.dialog label { display: grid; gap: 5px; font-weight: 600; }.dialog input, .dialog select { width: 100%; }.checkbox { display: flex !important; align-items: center; }.checkbox input { width: auto; }.hint { margin: 0; color: var(--text-soft); font-size: 13px; }.dialog footer { justify-content: end; }
</style>
