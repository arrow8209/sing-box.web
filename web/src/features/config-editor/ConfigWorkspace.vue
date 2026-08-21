<script setup lang="ts">
import { computed, ref } from "vue";
import { useProfileStore } from "@/features/profiles/store";
import type { Draft } from "@/features/profiles/types";
import ConfigTabs from "./ConfigTabs.vue";
import ModuleJsonDialog from "./ModuleJsonDialog.vue";
import OutboundJsonEditor from "./OutboundJsonEditor.vue";
import OverviewPanel from "./OverviewPanel.vue";
import ReferenceErrors from "./ReferenceErrors.vue";
import StructuredModulePanel from "./StructuredModulePanel.vue";
import { configTabs, moduleLabels, type ConfigModule, type ConfigTabId } from "./module-map";
import { saveModuleBuffer } from "./paths";

const store = useProfileStore();
const active = ref<ConfigTabId>("overview");
const dialogModule = ref<ConfigModule | null>(null);
const errors = ref<Record<string, string>>({});
const draft = computed(() => store.draft.value);
const activeModules = computed(() => configTabs.find((tab) => tab.id === active.value)?.modules ?? []);
function applyResult(next: Draft, module: string, error?: string): void { store.mutate((current) => Object.assign(current, next)); if (error) errors.value = { ...errors.value, [module]: error }; else { const copy = { ...errors.value }; delete copy[module]; errors.value = copy; } }
function persist(module: ConfigModule, text: string): boolean { if (!draft.value) return false; const result = saveModuleBuffer(draft.value, module, text); applyResult(result.draft, module, result.error?.message); return !result.error; }
function save(module: ConfigModule, text: string): void { if (persist(module, text)) dialogModule.value = null; }
function fromStructured(next: Draft, error?: string): void { applyResult(next, "log", error); }
</script>
<template>
  <section class="workspace"><header><div><h1>配置工作区</h1><p>草稿会自动保存。模块 JSON 和未识别字段按原始文本保留。</p></div><span v-if="store.saveState.value === 'saving'">保存中</span><span v-else-if="store.saveState.value === 'saved'">已保存</span></header><template v-if="draft"><ConfigTabs :active="active" @select="active = $event" /><OverviewPanel v-if="active === 'overview'" :draft="draft" :error="errors.log" @update="fromStructured" @edit-json="dialogModule = $event" /><OutboundJsonEditor v-else-if="active === 'outbounds'" :text="draft.buffers.outbounds ?? '[]'" :error="errors.outbounds" @buffer="persist('outbounds', $event)" @save="save('outbounds', $event)" /><div v-else class="modules"><StructuredModulePanel v-for="module in activeModules" :key="module" :draft="draft" :module="module" :error="errors[module]" @edit-json="dialogModule = $event" /></div><ReferenceErrors :errors="errors" /><ModuleJsonDialog v-if="dialogModule" :open="true" :title="`编辑 ${moduleLabels[dialogModule]} JSON`" :text="draft.buffers[dialogModule] ?? ''" :error="errors[dialogModule]" @close="dialogModule = null" @buffer="persist(dialogModule!, $event)" @save="save(dialogModule!, $event)" /></template><p v-else class="empty">请先在“配置档案”中选择一个可用档案。</p></section>
</template>
<style scoped>
.workspace { max-width: 1100px; }.workspace > header { display: flex; justify-content: space-between; gap: 16px; margin-bottom: 20px; }.workspace h1 { margin: 0; font-size: 24px; }.workspace header p, .empty { color: var(--text-soft); }.workspace > header span { height: fit-content; padding: 5px 9px; border-radius: 999px; color: var(--primary-strong); background: var(--primary-soft); }.modules { display: grid; gap: 14px; margin-top: 18px; }.tabs + .panel, .tabs + .outbound-editor { margin-top: 18px; }.empty { padding: 24px; border: 1px dashed var(--line-strong); border-radius: var(--radius); }
</style>
