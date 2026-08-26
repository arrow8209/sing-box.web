<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useProfileStore } from "@/features/profiles/store";
import { useApplicationStore } from "./store";

const props = withDefaults(defineProps<{ bufferErrors?: string[] }>(), { bufferErrors: () => [] });
const emit = defineEmits<{ preview: []; validate: []; apply: [] }>();
const profiles = useProfileStore();
const app = useApplicationStore();
const disabled = computed(() => !app.canApply.value || props.bufferErrors.length > 0);

onMounted(() => { void app.refreshCore().catch(() => undefined); });
function apply(): void { if (!disabled.value) emit("apply"); }
</script>
<template>
  <div class="application-actions" role="group" aria-label="配置操作">
    <button data-preview type="button" class="secondary" :disabled="profiles.saveState.value !== 'saved'" @click="emit('preview')">预览</button>
    <button data-validate type="button" class="secondary" :disabled="profiles.saveState.value !== 'saved'" @click="emit('validate')">检查</button>
    <button data-apply type="button" :disabled="disabled" @click="apply">应用</button>
  </div>
</template>
<style scoped>
.application-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }
button { min-height: 34px; padding: 6px 10px; font-size: 10px; }
.secondary { color: var(--text); background: var(--surface-soft); }
</style>
