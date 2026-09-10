<script setup lang="ts">
import { computed, ref, watch } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { validateSubscriptionJson } from "./json";
import type { SubscriptionDetail, SubscriptionInput } from "./types";

const props = defineProps<{
  open: boolean;
  subscription: SubscriptionDetail | null;
  suggestedName: string;
  busy?: boolean;
}>();
const emit = defineEmits<{ close: []; submit: [input: SubscriptionInput]; delete: [] }>();

const example = `{
  "name": "我的 JSON 订阅",
  "outbounds": [
    {
      "type": "shadowsocks",
      "tag": "香港 01",
      "server": "hk01.example.com",
      "server_port": 8388,
      "method": "2022-blake3-aes-128-gcm",
      "password": "change-me"
    },
    {
      "type": "trojan",
      "tag": "新加坡 01",
      "server": "sg01.example.com",
      "server_port": 443,
      "password": "change-me",
      "tls": {
        "enabled": true,
        "server_name": "sg01.example.com"
      }
    }
  ]
}`;

const source = ref(example);
const validation = computed(() => validateSubscriptionJson(source.value));
const title = computed(() => props.subscription ? "编辑 JSON 订阅" : "导入 JSON 订阅");
const description = computed(() => props.subscription ? `整体编辑 ${props.subscription.name} 的全部节点` : "通过 JSON 一次添加订阅中的所有节点");

watch(() => [props.open, props.subscription] as const, ([open]) => {
  if (!open) return;
  source.value = props.subscription
    ? JSON.stringify({ name: props.subscription.name, outbounds: props.subscription.outbounds }, null, 2)
    : example;
}, { immediate: true });

function submit(): void {
  if (!validation.value.valid) return;
  const name = validation.value.name || props.subscription?.name || props.suggestedName;
  emit("submit", { name, outbounds: validation.value.outbounds });
}

function format(): void {
  if (!validation.value.valid) return;
  source.value = JSON.stringify(validation.value.parsed, null, 2);
}

function insertIndent(event: KeyboardEvent): void {
  if (event.key === "Tab") {
    event.preventDefault();
    const target = event.currentTarget as HTMLTextAreaElement;
    const start = target.selectionStart;
    const end = target.selectionEnd;
    source.value = `${source.value.slice(0, start)}  ${source.value.slice(end)}`;
    requestAnimationFrame(() => { target.selectionStart = target.selectionEnd = start + 2; });
  }
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && validation.value.valid && !props.busy) {
    event.preventDefault();
    submit();
  }
}
</script>

<template>
  <AppDialog class="subscription-json-dialog" :open="open" :title="title" :description="description" :close-disabled="busy" initial-focus="textarea" @close="emit('close')">
    <form id="subscription-json-form" class="subscription-json-form" @submit.prevent="submit">
      <div class="json-intro"><AppIcon name="code" :size="20" /><div><strong>粘贴完整节点 JSON</strong><span>支持完整 sing-box 配置的 <code>outbounds</code>，也支持直接粘贴节点数组。</span></div></div>
      <div class="json-workbench">
        <div class="json-toolbar"><div><b>JSON</b><span>全部订阅节点</span></div><div><button type="button" @click="source = example">示例</button><button type="button" :disabled="!validation.valid" @click="format"><AppIcon name="sparkles" :size="13" />格式化</button></div></div>
        <textarea v-model="source" aria-label="订阅 JSON" :aria-invalid="!validation.valid" aria-describedby="subscription-json-validation" spellcheck="false" @keydown="insertIndent" />
      </div>
      <p id="subscription-json-validation" class="validation" :class="validation.valid ? 'valid' : 'invalid'" role="status"><AppIcon :name="validation.valid ? 'check' : 'info'" :size="14" />{{ validation.message }}</p>
      <p class="json-help"><AppIcon name="info" :size="15" /><span>保存时会整体替换该订阅的节点集合；除 <code>name</code> 和 <code>outbounds</code> 外的其他顶层配置不会保存。</span></p>
    </form>
    <template #footer>
      <button v-if="subscription" class="delete-button" type="button" :disabled="busy" @click="emit('delete')"><AppIcon name="trash" :size="14" />删除订阅</button>
      <button class="secondary-button" type="button" :disabled="busy" @click="emit('close')">取消</button>
      <button class="primary-button" type="submit" form="subscription-json-form" :disabled="!validation.valid || busy"><AppIcon :name="subscription ? 'save' : 'plus'" :size="14" />{{ busy ? "正在保存" : subscription ? "保存全部节点" : "导入全部节点" }}</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.subscription-json-dialog) { width: min(760px, calc(100vw - 32px)); }.subscription-json-form { display: grid; gap: 12px; }.json-intro { display: flex; align-items: flex-start; gap: 10px; padding: 11px 12px; border: 1px solid color-mix(in srgb, var(--primary) 15%, var(--line)); border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.json-intro strong, .json-intro span { display: block; }.json-intro strong { color: var(--text); font-size: var(--font-size-base); }.json-intro span { margin-top: 2px; color: var(--text-soft); font-size: var(--font-size-xs); }.json-intro code, .json-help code { color: var(--primary-strong); font: inherit; font-weight: 700; }.json-workbench { overflow: hidden; border: 1px solid var(--line); border-radius: 11px; background: #10182a; }.json-toolbar { display: flex; min-height: 39px; align-items: center; justify-content: space-between; gap: 10px; padding: 7px 10px; border-bottom: 1px solid rgb(255 255 255 / 9%); color: #91a0bb; font-size: var(--font-size-xs); }.json-toolbar b { margin-right: 8px; color: #8aa2ff; }.json-toolbar button { display: inline-flex; min-height: 27px; align-items: center; gap: 4px; padding: 4px 7px; color: #b8c4d9; background: transparent; font-size: var(--font-size-sm); }.json-toolbar button + button { margin-left: 3px; }.json-toolbar button:hover { background: rgb(255 255 255 / 8%); }.json-workbench textarea { display: block; width: 100%; min-height: 340px; padding: 14px 16px; border: 0; border-radius: 0; outline: 0; color: #d4def0; background: #10182a; font: var(--font-size-base)/1.62 ui-monospace, SFMono-Regular, Consolas, monospace; resize: vertical; }.json-workbench textarea[aria-invalid="true"] { box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--red) 75%, transparent); }.validation { display: flex; align-items: center; gap: 7px; margin: 0; padding: 8px 10px; border-radius: 8px; font-size: var(--font-size-xs); }.validation.valid { color: var(--green); background: var(--green-soft); }.validation.invalid { color: var(--red); background: var(--red-soft); }.json-help { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 9px 10px; border: 1px solid var(--line); border-radius: 9px; color: var(--primary); background: var(--surface-soft); font-size: var(--font-size-xs); }.json-help span { color: var(--text-soft); }.secondary-button, .primary-button, .delete-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }.delete-button { margin-right: auto; color: var(--red); border-color: color-mix(in srgb, var(--red) 20%, var(--line)); background: var(--red-soft); }
@media (max-width: 680px) { :global(.subscription-json-dialog) { width: 100%; }.json-workbench textarea { min-height: 42dvh; }.delete-button { width: 36px; justify-content: center; padding-inline: 0; overflow: hidden; color: transparent; gap: 0; }.delete-button :deep(.icon) { color: var(--red); } }
</style>
