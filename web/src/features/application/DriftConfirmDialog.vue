<script setup lang="ts">
import AppDialog from "@/components/AppDialog.vue";

withDefaults(defineProps<{ open: boolean; busy?: boolean }>(), { busy: false });
const emit = defineEmits<{ close: []; confirm: [] }>();
</script>

<template>
  <AppDialog :open="open" title="检测到运行配置已被外部修改" description="继续前需要确认覆盖当前 config/。" :close-disabled="busy" @close="emit('close')">
    <p class="drift-note">外部修改不会导入当前草稿。确认后会再次校验现场摘要，并用已通过检查的配置替换运行目录。</p>
    <template #footer>
      <button class="secondary" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button data-confirm-drift type="button" :disabled="busy" @click="emit('confirm')">覆盖并应用</button>
    </template>
  </AppDialog>
</template>

<style scoped>
.drift-note { margin: 0; color: var(--text-soft); line-height: 1.7; }
.secondary { color: var(--text); background: var(--surface-soft); }
</style>
