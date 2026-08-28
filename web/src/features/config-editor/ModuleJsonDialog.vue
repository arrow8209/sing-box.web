<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { applyEdits, format } from "jsonc-parser";
import AppDialog from "@/components/AppDialog.vue";
import JsonEditor from "@/components/JsonEditor.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { validateModuleShape } from "./json";
import { configSectionFor, type ConfigModule } from "./module-map";

const props = defineProps<{
  open: boolean;
  module: ConfigModule;
  text: string;
  configured: boolean;
  error?: string;
}>();
const emit = defineEmits<{ close: []; save: [text: string]; buffer: [text: string]; clear: [] }>();
const buffer = ref(props.text);
const section = computed(() => configSectionFor(props.module));
const validation = computed(() => validateModuleShape(buffer.value, props.module, section.value.type));
const displayedError = computed(() => validation.value?.message ?? props.error);

watch(() => [props.open, props.text] as const, ([open, text]) => {
  if (open) buffer.value = text;
});

function pretty(): void {
  if (validation.value || !buffer.value.trim()) return;
  buffer.value = applyEdits(buffer.value, format(buffer.value, undefined, { insertSpaces: true, tabSize: 2, eol: "\n" }));
  emit("buffer", buffer.value);
}

function save(): void {
  if (!validation.value) emit("save", buffer.value);
}
</script>

<template>
  <AppDialog :open="open" :title="`编辑${section.label}`" :description="`完整维护 config.${section.key}`" initial-focus="textarea" @close="emit('close')">
    <div class="module-meta">
      <span class="module-meta-icon"><AppIcon :name="section.icon" :size="15" /></span>
      <div><strong>{{ section.label }}</strong><code>{{ section.key }}</code><small>{{ section.description }}</small></div>
      <span class="type-badge">{{ section.type }}</span>
    </div>
    <div class="editor-shell">
      <div class="editor-toolbar"><div><strong>JSON</strong><span>顶层配置片段</span></div><button type="button" class="format-button" :disabled="Boolean(validation) || !buffer.trim()" @click="pretty"><AppIcon name="sparkles" :size="13" />格式化</button></div>
      <JsonEditor v-model="buffer" :label="`${section.label} JSON`" :error="displayedError" @update:model-value="emit('buffer', buffer)" />
    </div>
    <div class="info-callout"><AppIcon name="shield" :size="15" /><span>编辑器只检查 JSON 容器类型；字段、平台与版本兼容性请在部署前使用目标核心执行 <code>sing-box check</code>。</span></div>
    <template #footer>
      <button v-if="configured" type="button" class="clear-button" @click="emit('clear')"><AppIcon name="trash" :size="14" />清空模块</button>
      <span class="footer-spacer" />
      <button type="button" class="secondary" @click="emit('close')">取消</button>
      <button type="button" data-save-module :disabled="Boolean(validation)" @click="save"><AppIcon name="save" :size="14" />保存片段</button>
    </template>
  </AppDialog>
</template>

<style scoped>
.module-meta { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 11px; margin-bottom: 12px; padding: 11px 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }
.module-meta-icon { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 8px; color: var(--primary); background: var(--primary-soft); }
.module-meta strong, .module-meta code, .module-meta small { display: block; }
.module-meta strong { font-size: 11px; }
.module-meta code { margin-top: 1px; color: var(--primary); font-size: 9px; }
.module-meta small { margin-top: 2px; color: var(--text-faint); font-size: 9px; }
.type-badge { display: inline-flex; min-height: 22px; align-items: center; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface); font-size: 9px; font-weight: 660; }
.editor-shell { overflow: hidden; border: 1px solid var(--line-strong); border-radius: 10px; background: var(--surface-soft); }
.editor-toolbar { display: flex; min-height: 38px; align-items: center; justify-content: space-between; gap: 12px; padding: 6px 8px 6px 11px; border-bottom: 1px solid var(--line); }
.editor-toolbar > div { display: flex; align-items: center; gap: 8px; color: var(--text-faint); font-size: 9px; }
.editor-toolbar strong { color: var(--primary); font-size: 9px; letter-spacing: .4px; }
.format-button { display: inline-flex; min-height: 27px; align-items: center; gap: 5px; padding: 4px 7px; color: var(--text-soft); background: var(--surface); font-size: 9px; }
.editor-shell :deep(.json-editor) { gap: 0; }
.editor-shell :deep(textarea) { min-height: min(48vh, 420px); border: 0; border-radius: 0; background: transparent; }
.editor-shell :deep(.json-editor__error) { margin: 0; padding: 8px 11px; border-top: 1px solid color-mix(in srgb, var(--red) 28%, var(--line)); background: var(--red-soft); font-size: 10px; }
.info-callout { display: flex; gap: 9px; margin-top: 12px; padding: 10px 11px; border: 1px solid color-mix(in srgb, var(--primary) 14%, var(--line)); border-radius: 9px; color: var(--text-soft); background: var(--primary-soft); font-size: 10px; }
.info-callout :deep(.icon) { flex: 0 0 auto; color: var(--primary); }
.info-callout code { color: var(--primary-strong); }
.clear-button { display: inline-flex; align-items: center; gap: 6px; color: var(--red); background: var(--red-soft); }
.footer-spacer { flex: 1; }
[data-save-module] { display: inline-flex; align-items: center; gap: 6px; }

@media (max-width: 560px) {
  .module-meta { grid-template-columns: auto minmax(0, 1fr); }
  .type-badge { display: none; }
  .footer-spacer { display: none; }
  .clear-button { width: 100%; justify-content: center; }
}
</style>
