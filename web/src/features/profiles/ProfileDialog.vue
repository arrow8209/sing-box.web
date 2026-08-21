<script setup lang="ts">
import { computed, ref, watch } from "vue";

const props = withDefaults(defineProps<{ open: boolean; title: string; initialName?: string; busy?: boolean }>(), { initialName: "", busy: false });
const emit = defineEmits<{ submit: [name: string]; close: [] }>();
const name = ref(props.initialName);
watch(() => props.open, (open) => { if (open) name.value = props.initialName; });
const valid = computed(() => name.value.trim().length > 0);
function submit(): void { if (valid.value) emit("submit", name.value.trim()); }
</script>

<template>
  <div v-if="open" class="backdrop" role="presentation" @click.self="emit('close')">
    <form class="dialog" role="dialog" aria-modal="true" :aria-label="title" @submit.prevent="submit">
      <h2>{{ title }}</h2>
      <label>名称<input v-model="name" autofocus maxlength="80" /></label>
      <div class="actions"><button type="button" class="secondary" @click="emit('close')">取消</button><button :disabled="!valid || busy">确认</button></div>
    </form>
  </div>
</template>

<style scoped>
.backdrop { position: fixed; z-index: 30; inset: 0; display: grid; place-items: center; padding: 20px; background: rgb(10 18 33 / 45%); }
.dialog { width: min(420px, 100%); padding: 22px; border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-lg); } h2 { margin: 0 0 18px; font-size: 18px; } label { display: grid; gap: 7px; color: var(--text-soft); font-weight: 600; } input { width: 100%; } .actions { display: flex; justify-content: end; gap: 8px; margin-top: 22px; } .secondary { color: var(--text); background: var(--surface-soft); }
</style>
