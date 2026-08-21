<script setup lang="ts">
import { computed, ref, watch } from "vue";

const props = defineProps<{ open: boolean; name: string; requiresCoreDecision?: boolean; busy?: boolean }>();
const emit = defineEmits<{ submit: [stopCurrent: boolean]; close: [] }>();
const confirmation = ref("");
const stopCurrent = ref(false);
watch(() => props.open, (open) => { if (open) { confirmation.value = ""; stopCurrent.value = false; } });
const valid = computed(() => confirmation.value === props.name && (!props.requiresCoreDecision || stopCurrent.value));
</script>

<template>
  <div v-if="open" class="backdrop" role="presentation" @click.self="emit('close')">
    <section class="dialog" role="dialog" aria-modal="true" aria-label="删除档案">
      <h2>删除“{{ name }}”</h2><p>输入完整名称以确认。删除草稿无法恢复。</p>
      <label>确认名称<input v-model="confirmation" autofocus /></label>
      <label v-if="requiresCoreDecision" class="check"><input v-model="stopCurrent" type="checkbox" /> 停止核心并删除当前配置</label>
      <div class="actions"><button class="secondary" @click="emit('close')">取消</button><button class="danger" :disabled="!valid || busy" @click="emit('submit', stopCurrent)">删除</button></div>
    </section>
  </div>
</template>

<style scoped>
.backdrop { position: fixed; z-index: 30; inset: 0; display: grid; place-items: center; padding: 20px; background: rgb(10 18 33 / 45%); }.dialog { width: min(440px, 100%); padding: 22px; border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-lg); } h2 { margin: 0 0 10px; font-size: 18px; } p { color: var(--text-soft); } label { display: grid; gap: 7px; font-weight: 600; } input { width: 100%; }.check { display: flex; margin-top: 16px; color: var(--red); }.check input { width: auto; }.actions { display: flex; justify-content: end; gap: 8px; margin-top: 22px; }.secondary { color: var(--text); background: var(--surface-soft); }.danger { background: var(--red); }
</style>
