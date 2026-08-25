<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { RouterLink } from "vue-router";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { inboundTypes, uniqueInboundTag } from "./model";
import { useInboundStore } from "./store";

const props = withDefaults(defineProps<{ open: boolean; profileName: string; currentTags: string[]; busy?: boolean }>(), { busy: false });
const emit = defineEmits<{ close: []; import: [ids: string[]] }>();
const store = useInboundStore();
const selected = ref<string[]>([]);
const error = ref("");
const rows = computed(() => store.resources.value.map((item) => ({
  ...item,
  icon: item.mode === "custom" ? "code" : inboundTypes.find((candidate) => candidate.type === item.type)?.icon ?? "inbound",
  nextTag: uniqueInboundTag(item.tag, props.currentTags)
})));

watch(() => props.open, (open) => {
  if (!open) return;
  selected.value = [];
  error.value = "";
  void refresh();
}, { immediate: true });

async function refresh(): Promise<void> {
  try { await store.refresh(); } catch (cause) { error.value = cause instanceof Error ? cause.message : "无法加载入站配置"; }
}
function toggle(id: string, checked: boolean): void {
  selected.value = checked ? [...selected.value, id] : selected.value.filter((item) => item !== id);
}
function submit(): void { if (selected.value.length && !props.busy) emit("import", [...selected.value]); }
</script>

<template>
  <AppDialog class="inbound-import-dialog" :open="open" title="导入入站配置" :description="`复制到“${profileName}”`" :close-disabled="busy" @close="emit('close')">
    <form v-if="rows.length" id="inbound-import-form" class="import-form" @submit.prevent="submit">
      <div class="import-banner"><AppIcon name="copy" :size="20" /><div><strong>导入会创建独立副本</strong><span>只复制当前配置内容，不保存来源 ID；导入后两边的编辑和删除均不联动。</span></div></div>
      <p v-if="error" class="load-error" role="alert">{{ error }} <button type="button" @click="refresh">重试</button></p>
      <div class="import-list">
        <label v-for="item in rows" :key="item.id" class="import-option"><input type="checkbox" :value="item.id" :checked="selected.includes(item.id)" @change="toggle(item.id, ($event.target as HTMLInputElement).checked)" /><span class="import-tile"><span class="entry-icon" :class="{ custom: item.mode === 'custom', tun: item.type === 'tun' }"><AppIcon :name="item.icon" :size="18" /></span><span class="import-copy"><strong>{{ item.tag }}</strong><small>{{ item.endpoint }}</small><span><b>{{ item.type }}</b><em v-if="item.nextTag !== item.tag">将改名为 {{ item.nextTag }}</em></span></span><AppIcon class="check" name="check" :size="17" /></span></label>
      </div>
    </form>
    <div v-else class="import-empty" :aria-busy="store.loading.value"><AppIcon name="inbound" :size="34" /><strong>{{ store.loading.value ? "正在加载入站配置" : "还没有可导入的入站配置" }}</strong><span v-if="!store.loading.value">请先到一级“入站配置”中新建项目。</span><RouterLink v-if="!store.loading.value" to="/inbounds" @click="emit('close')"><AppIcon name="plus" :size="14" />前往新建</RouterLink><p v-if="error" role="alert">{{ error }}</p></div>
    <template #footer><span v-if="rows.length" class="footer-note">可同时选择多个项目</span><button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button><button v-if="rows.length" class="primary-button" type="submit" form="inbound-import-form" :disabled="!selected.length || busy"><AppIcon name="download" :size="14" />{{ busy ? "正在导入" : `导入所选 ${selected.length} 项` }}</button></template>
  </AppDialog>
</template>

<style scoped>
:global(.inbound-import-dialog) { width: min(780px, calc(100vw - 32px)); }.import-form { display: grid; gap: 14px; }.import-banner { display: flex; align-items: center; gap: 11px; padding: 11px 12px; border: 1px solid color-mix(in srgb, var(--primary) 24%, var(--line)); border-radius: 11px; color: var(--primary); background: var(--primary-soft); }.import-banner strong, .import-banner span { display: block; }.import-banner strong { color: var(--text); font-size: 10px; }.import-banner span { margin-top: 2px; color: var(--text-soft); font-size: 8px; }.import-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 9px; }.import-option { min-width: 0; }.import-option > input { position: absolute; width: 1px; height: 1px; overflow: hidden; opacity: 0; }.import-tile { display: grid; min-height: 82px; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 9px; padding: 10px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface-soft); cursor: pointer; transition: 150ms ease; }.import-tile:hover { border-color: color-mix(in srgb, var(--primary) 30%, var(--line)); transform: translateY(-1px); }.entry-icon { display: grid; width: 35px; height: 35px; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.entry-icon.custom { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.entry-icon.tun { color: var(--cyan); background: var(--cyan-soft); }.import-copy { min-width: 0; }.import-copy strong, .import-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.import-copy strong { font-size: 10px; }.import-copy small { margin-top: 2px; color: var(--text-faint); font-size: 8px; }.import-copy > span { display: flex; align-items: center; gap: 5px; margin-top: 5px; }.import-copy b, .import-copy em { padding: 2px 6px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 700 8px ui-monospace, monospace; font-style: normal; }.import-copy em { overflow: hidden; color: var(--orange); background: var(--orange-soft); text-overflow: ellipsis; white-space: nowrap; }.check { color: transparent; }.import-option > input:checked + .import-tile { border-color: color-mix(in srgb, var(--primary) 48%, var(--line)); background: var(--primary-soft); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 8%, transparent); }.import-option > input:checked + .import-tile .check { color: var(--primary); }.import-option > input:focus-visible + .import-tile { outline: 3px solid color-mix(in srgb, var(--primary) 22%, transparent); outline-offset: 1px; }.import-empty { display: grid; min-height: 270px; place-items: center; align-content: center; gap: 8px; color: var(--text-faint); text-align: center; }.import-empty > .icon { color: var(--primary); }.import-empty strong { color: var(--text); font-size: 12px; }.import-empty span, .import-empty p { margin: 0; font-size: 9px; }.import-empty p { color: var(--red); }.import-empty a { display: inline-flex; min-height: 34px; align-items: center; gap: 5px; margin-top: 7px; padding: 6px 10px; border-radius: 8px; color: #fff; background: var(--primary); font-size: 9px; font-weight: 700; text-decoration: none; }.load-error { display: flex; align-items: center; justify-content: space-between; margin: 0; padding: 8px 10px; border-radius: 8px; color: var(--red); background: var(--red-soft); font-size: 9px; }.load-error button { min-height: 27px; padding: 3px 7px; color: var(--red); background: var(--surface); font-size: 8px; }.footer-note { margin-right: auto; color: var(--text-faint); font-size: 9px; }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: 10px; }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 680px) { :global(.inbound-import-dialog) { width: 100%; }.import-list { grid-template-columns: 1fr; }.footer-note { display: none; } }
</style>
