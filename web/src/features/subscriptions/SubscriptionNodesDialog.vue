<script setup lang="ts">
import { computed } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { SubscriptionDetail, SubscriptionOutbound } from "./types";

const props = defineProps<{ open: boolean; subscription: SubscriptionDetail | null }>();
const emit = defineEmits<{ close: []; copied: [message: string]; error: [message: string] }>();

const protocols = computed(() => [...new Set((props.subscription?.outbounds ?? []).map((node) => String(node.type ?? "unknown")))]);

function initials(value: unknown): string {
  const text = String(value ?? "?").trim();
  const words = text.split(/\s+/).filter(Boolean);
  if (words.length > 1) return words.slice(0, 2).map((word) => Array.from(word)[0]).join("").toUpperCase();
  return Array.from(text).slice(0, 2).join("").toUpperCase();
}

function endpoint(node: SubscriptionOutbound): string {
  const server = node.server ?? node.address ?? node.server_address;
  const port = node.server_port ?? node.port;
  if (server) return `${String(server)}${port !== undefined && port !== "" ? `:${String(port)}` : ""}`;
  if (Array.isArray(node.outbounds)) return `${node.outbounds.length} 个引用出站`;
  return "未设置远程地址";
}

async function copy(): Promise<void> {
  if (!props.subscription) return;
  try {
    await navigator.clipboard.writeText(JSON.stringify({ name: props.subscription.name, outbounds: props.subscription.outbounds }, null, 2));
    emit("copied", `已复制 ${props.subscription.outbounds.length} 个节点的完整 JSON。`);
  } catch {
    emit("error", "浏览器未允许写入剪贴板，请从编辑器中手动复制。" );
  }
}
</script>

<template>
  <AppDialog class="subscription-nodes-dialog" :open="open" title="全部订阅节点" :description="subscription ? `${subscription.name} 中的所有节点` : ''" @close="emit('close')">
    <div v-if="subscription" class="nodes-body">
      <div class="node-overview">
        <span class="provider-logo">{{ initials(subscription.name) }}</span>
        <div><strong>{{ subscription.name }}</strong><span>{{ subscription.outbounds.length }} 个节点 · {{ protocols.length }} 种协议 · 完整来自 JSON</span></div>
        <div class="overview-protocols"><span v-for="protocol in protocols" :key="protocol" class="type-badge">{{ protocol }}</span></div>
      </div>
      <div class="node-list-head"><span>#</span><span>节点</span><span>协议</span><span>服务器</span></div>
      <div class="node-list">
        <div v-for="(node, index) in subscription.outbounds" :key="`${String(node.tag)}-${index}`" class="node-row">
          <span class="node-index">{{ String(index + 1).padStart(2, "0") }}</span>
          <span class="node-logo">{{ initials(node.tag ?? node.type) }}</span>
          <div class="node-copy"><strong :title="String(node.tag)">{{ node.tag }}</strong><small>订阅节点 #{{ index + 1 }}</small></div>
          <span class="type-badge">{{ node.type }}</span>
          <code :title="endpoint(node)">{{ endpoint(node) }}</code>
        </div>
      </div>
    </div>
    <template #footer>
      <button class="secondary-button" type="button" @click="copy"><AppIcon name="copy" :size="14" />复制全部 JSON</button>
      <button class="primary-button" type="button" @click="emit('close')">完成</button>
    </template>
  </AppDialog>
</template>

<style scoped>
:global(.subscription-nodes-dialog) { width: min(820px, calc(100vw - 32px)); }.nodes-body { display: grid; gap: 12px; }.node-overview { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 13px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface-soft); }.provider-logo { display: grid; width: 43px; height: 43px; place-items: center; border-radius: 13px; color: #fff; background: linear-gradient(145deg, #657ffc, #3e5bd6); box-shadow: 0 8px 17px rgb(79 111 248 / 17%); font-size: var(--font-size-base); font-weight: 760; }.node-overview > div:nth-child(2) { min-width: 0; }.node-overview > div:nth-child(2) > strong, .node-overview > div:nth-child(2) > span { display: block; }.node-overview > div:nth-child(2) > strong { overflow: hidden; font-size: var(--font-size-base); text-overflow: ellipsis; white-space: nowrap; }.node-overview > div:nth-child(2) > span { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }.overview-protocols { display: flex; max-width: 280px; flex-wrap: wrap; justify-content: flex-end; gap: 4px; }.type-badge { display: inline-flex; min-height: 21px; align-items: center; padding: 3px 7px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font: 700 var(--font-size-xs) ui-monospace, SFMono-Regular, Consolas, monospace; white-space: nowrap; }.node-list-head { display: grid; grid-template-columns: 76px minmax(170px, 1fr) 100px minmax(170px, .8fr); gap: 10px; padding: 0 12px; color: var(--text-faint); font-size: var(--font-size-sm); }.node-list { max-height: min(52dvh, 500px); overflow: auto; border: 1px solid var(--line); border-radius: 11px; }.node-row { display: grid; min-height: 55px; grid-template-columns: 34px 32px minmax(128px, 1fr) 100px minmax(170px, .8fr); align-items: center; gap: 10px; padding: 8px 12px; border-bottom: 1px solid var(--line); }.node-row:last-child { border-bottom: 0; }.node-row:hover { background: var(--surface-soft); }.node-index { color: var(--text-faint); font: var(--font-size-sm) ui-monospace, monospace; }.node-logo { display: grid; width: 30px; height: 30px; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); font-weight: 760; }.node-copy { min-width: 0; }.node-copy strong, .node-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.node-copy strong { font-size: var(--font-size-base); }.node-copy small { margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-sm); }.node-row code { overflow: hidden; color: var(--text-soft); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }.secondary-button, .primary-button { display: inline-flex; min-height: 36px; align-items: center; gap: 6px; padding: 7px 11px; border: 1px solid var(--line); font-size: var(--font-size-sm); }.secondary-button { color: var(--text-soft); background: var(--surface); }.primary-button { color: #fff; border-color: var(--primary); background: var(--primary); }
@media (max-width: 680px) { :global(.subscription-nodes-dialog) { width: 100%; }.node-overview { grid-template-columns: auto minmax(0, 1fr); }.overview-protocols { grid-column: 1 / -1; max-width: none; justify-content: flex-start; }.node-list-head { display: none; }.node-list { max-height: 50dvh; }.node-row { grid-template-columns: 28px minmax(0, 1fr) auto; gap: 8px; }.node-logo { display: none; }.node-copy { grid-column: 2; }.node-row > .type-badge { grid-column: 3; grid-row: 1; }.node-row code { grid-column: 2 / -1; grid-row: 2; }.node-index { grid-row: 1 / span 2; } }
</style>
