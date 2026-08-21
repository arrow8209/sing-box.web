<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = withDefaults(defineProps<{
  open: boolean;
  title: string;
  closeDisabled?: boolean;
  initialFocus?: string;
}>(), { closeDisabled: false, initialFocus: "[autofocus], input, textarea, select, button" });
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLDialogElement>();
let trigger: HTMLElement | null = null;

async function show(): Promise<void> {
  trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  if (!dialog.value?.open) dialog.value?.showModal();
  await nextTick();
  const target = dialog.value?.querySelector<HTMLElement>(props.initialFocus);
  target?.focus();
}
function hide(): void {
  if (dialog.value?.open) dialog.value.close();
}
function requestClose(): void {
  if (!props.closeDisabled) emit("close");
}
function onCancel(event: Event): void {
  event.preventDefault();
  requestClose();
}
function onClosed(): void {
  trigger?.focus();
  if (props.open) emit("close");
}
watch(() => props.open, (open) => { if (open) void show(); else hide(); });
onMounted(() => { if (props.open) void show(); });
onBeforeUnmount(() => { if (dialog.value?.open) dialog.value.close(); });
</script>

<template>
  <dialog ref="dialog" class="app-dialog" :aria-label="title" @cancel="onCancel" @close="onClosed">
    <div class="app-dialog__content">
      <header class="app-dialog__header"><h2>{{ title }}</h2><button v-if="!closeDisabled" type="button" class="icon-button" aria-label="关闭对话框" @click="requestClose">×</button></header>
      <slot />
      <footer v-if="$slots.footer" class="app-dialog__footer"><slot name="footer" /></footer>
    </div>
  </dialog>
</template>
