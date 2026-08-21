<script setup lang="ts">
import { computed, onMounted } from "vue";
import type { Draft, RuleBinding } from "@/features/profiles/types";
import RuleBindingEditor from "@/features/rule-resources/RuleBindingEditor.vue";
import { useRuleResourceStore } from "@/features/rule-resources/store";
import { moduleLabels, type ConfigModule } from "./module-map";
const props = defineProps<{ draft: Draft; module: ConfigModule; error?: string }>();
const emit = defineEmits<{ editJson: [module: ConfigModule]; updateBindings: [bindings: RuleBinding[]] }>();
const ruleResources = useRuleResourceStore();
onMounted(() => { if (props.module === "route" && !ruleResources.resources.value.length) void ruleResources.refresh(); });
const value = computed(() => props.draft.config[props.module]);
const present = computed(() => value.value !== undefined || props.draft.buffers[props.module] !== undefined);
const summary = computed(() => Array.isArray(value.value) ? `${value.value.length} 个项目` : present.value ? "已配置" : "未配置");
const hasRemoteRuleSet = computed(() => {
  if (props.module !== "route" || !value.value || typeof value.value !== "object") return false;
  const route = value.value as Record<string, unknown>;
  return Array.isArray(route.rule_set) && route.rule_set.some((item) => typeof item === "object" && item !== null && (item as Record<string, unknown>).type === "remote");
});
</script>
<template>
  <section class="panel"><header><div><h2>{{ moduleLabels[module] }}</h2><p>{{ summary }}</p></div><button type="button" @click="emit('editJson', module)">编辑 JSON</button></header><template v-if="Array.isArray(value)"><ul><li v-for="(item, index) in value" :key="index"><strong>{{ typeof item === 'object' && item ? (item as Record<string, unknown>).tag ?? `项目 ${index + 1}` : `项目 ${index + 1}` }}</strong><span v-if="typeof item === 'object' && item">{{ (item as Record<string, unknown>).type }}</span></li></ul></template><p v-else class="hint">此模块提供完整 JSON 编辑；未知字段会原样保留。</p><p v-if="hasRemoteRuleSet" class="error" role="alert">原生 remote 规则集会阻止应用。请建立 Web 规则资源并绑定，或将该 remote 项改为 local/inline JSON。</p><RuleBindingEditor v-if="module === 'route'" :bindings="draft.bindings" :resources="ruleResources.resources.value" @change="emit('updateBindings', $event)" /><p v-if="error" class="error" role="alert">{{ error }}</p></section>
</template>
<style scoped>
.panel { padding: 18px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }.panel header { display: flex; align-items: start; justify-content: space-between; gap: 12px; }.panel h2, .panel p { margin: 0; }.panel p, li span { color: var(--text-soft); }.panel ul { display: grid; gap: 7px; padding: 0; list-style: none; }.panel li { display: flex; justify-content: space-between; gap: 8px; padding: 9px; border-radius: var(--radius-sm); background: var(--surface-soft); }.panel .error { margin-top: 12px; color: var(--red); }
</style>
