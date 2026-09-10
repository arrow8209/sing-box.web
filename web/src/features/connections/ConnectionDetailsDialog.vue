<script setup lang="ts">
import { computed } from "vue";
import AppDialog from "@/components/AppDialog.vue";
import AppIcon from "@/layout/AppIcon.vue";
import {
  connectionChains,
  connectionDestination,
  connectionHost,
  connectionRule,
  connectionSource,
  connectionType,
  formatBytes,
  formatDuration,
  formatSpeed,
  formatStarted,
  metadataValue
} from "./format";
import type { DisplayConnection } from "./types";

const props = defineProps<{ open: boolean; connection: DisplayConnection | null; now: number; closing?: boolean }>();
defineEmits<{ close: []; terminate: [connection: DisplayConnection] }>();

const metadataLabels: Record<string, string> = {
  type: "入站类型",
  network: "网络协议",
  host: "请求主机",
  sourceIP: "源 IP",
  sourcePort: "源端口",
  destinationIP: "目标 IP",
  destinationPort: "目标端口",
  inboundIP: "入站监听 IP",
  inboundPort: "入站监听端口",
  inboundName: "入站名称",
  inboundUser: "入站用户",
  process: "进程",
  processPath: "进程路径",
  dnsMode: "DNS 模式",
  uid: "用户 ID",
  specialProxy: "特殊代理",
  specialRules: "特殊规则",
  remoteDestination: "远端目标"
};

const metadataEntries = computed(() => Object.entries(props.connection?.metadata ?? {}).sort(([left], [right]) => {
  const leftKnown = left in metadataLabels ? 0 : 1;
  const rightKnown = right in metadataLabels ? 0 : 1;
  return leftKnown - rightKnown || left.localeCompare(right);
}));
const rawDetails = computed(() => {
  const connection = props.connection;
  if (!connection) return "";
  const { uploadSpeed, downloadSpeed, closedAt, ...raw } = connection;
  return JSON.stringify({
    ...raw,
    calculated: { uploadSpeed, downloadSpeed, ...(closedAt ? { closedAt } : {}) }
  }, null, 2);
});
</script>

<template>
  <AppDialog class="connection-details-dialog" :open="open" title="连接详情" description="当前连接的路由、端点、流量和核心元数据" @close="$emit('close')">
    <div v-if="connection" class="connection-details">
      <section class="detail-hero">
        <span class="detail-hero-icon"><AppIcon name="link" :size="20" /></span>
        <div><strong>{{ connectionHost(connection) }}</strong><small>{{ connectionType(connection) }} · {{ connection.closedAt ? "已关闭" : "活动中" }}</small></div>
        <span class="detail-state" :class="{ closed: connection.closedAt }">{{ connection.closedAt ? "已关闭" : "活动" }}</span>
      </section>

      <section class="detail-summary" aria-label="连接摘要">
        <div><span>源地址</span><strong>{{ connectionSource(connection) }}</strong></div>
        <div><span>目标地址</span><strong>{{ connectionDestination(connection) }}</strong></div>
        <div><span>匹配规则</span><strong>{{ connectionRule(connection) }}</strong></div>
        <div><span>代理链路</span><strong>{{ connectionChains(connection) }}</strong></div>
        <div><span>开始时间</span><strong>{{ formatStarted(connection.start) }}</strong></div>
        <div><span>持续时间</span><strong>{{ formatDuration(connection.start, now, connection.closedAt) }}</strong></div>
      </section>

      <section class="detail-traffic" aria-label="连接流量">
        <div><span><AppIcon name="upload" :size="14" />上行</span><strong>{{ formatBytes(connection.upload) }}</strong><small>{{ formatSpeed(connection.uploadSpeed) }}</small></div>
        <div><span><AppIcon name="download" :size="14" />下行</span><strong>{{ formatBytes(connection.download) }}</strong><small>{{ formatSpeed(connection.downloadSpeed) }}</small></div>
        <div><span><AppIcon name="database" :size="14" />合计</span><strong>{{ formatBytes(connection.upload + connection.download) }}</strong><small>连接累计流量</small></div>
      </section>

      <section class="metadata-section">
        <header><h3>核心元数据</h3><span>{{ metadataEntries.length }} 个字段</span></header>
        <dl class="metadata-grid">
          <div v-for="[key, value] in metadataEntries" :key="key"><dt>{{ metadataLabels[key] || key }}</dt><dd :title="metadataValue(value)">{{ metadataValue(value) }}</dd></div>
        </dl>
      </section>

      <details class="raw-details">
        <summary><AppIcon name="code" :size="14" />查看原始 JSON</summary>
        <pre>{{ rawDetails }}</pre>
      </details>
    </div>
    <template #footer><button v-if="connection && !connection.closedAt" class="danger terminate-connection" type="button" :disabled="closing" @click="$emit('terminate', connection)">{{ closing ? "正在关闭" : "关闭连接" }}</button><button class="secondary" type="button" @click="$emit('close')">关闭</button></template>
  </AppDialog>
</template>

<style scoped>
:global(.app-dialog.connection-details-dialog) { width: min(900px, calc(100vw - 32px)); }
.connection-details { display: grid; gap: 14px; }.detail-hero { display: flex; min-width: 0; align-items: center; gap: 11px; padding: 13px; border: 1px solid color-mix(in srgb, var(--primary) 16%, var(--line)); border-radius: 12px; background: linear-gradient(120deg, var(--primary-soft), var(--surface) 72%); }.detail-hero-icon { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--surface); }.detail-hero > div { min-width: 0; flex: 1; }.detail-hero strong, .detail-hero small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.detail-hero strong { font-size: 13px; }.detail-hero small { margin-top: 2px; color: var(--text-soft); font-size: 9px; }.detail-state { padding: 4px 8px; border-radius: 999px; color: var(--green); background: var(--green-soft); font-size: 9px; font-weight: 700; }.detail-state.closed { color: var(--text-soft); background: var(--surface-soft); }
.detail-summary { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1px; overflow: hidden; border: 1px solid var(--line); border-radius: 11px; background: var(--line); }.detail-summary > div { min-width: 0; padding: 10px 12px; background: var(--surface); }.detail-summary span, .detail-summary strong { display: block; }.detail-summary span { color: var(--text-faint); font-size: var(--font-size-sm); }.detail-summary strong { overflow: hidden; margin-top: 3px; color: var(--text-soft); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.detail-traffic { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }.detail-traffic > div { padding: 11px 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface-soft); }.detail-traffic span { display: flex; align-items: center; gap: 5px; color: var(--text-faint); font-size: var(--font-size-sm); }.detail-traffic strong, .detail-traffic small { display: block; }.detail-traffic strong { margin-top: 4px; font-size: 13px; }.detail-traffic small { margin-top: 1px; color: var(--text-faint); font-size: var(--font-size-sm); }
.metadata-section { overflow: hidden; border: 1px solid var(--line); border-radius: 11px; }.metadata-section header { display: flex; align-items: center; justify-content: space-between; padding: 9px 12px; border-bottom: 1px solid var(--line); background: var(--surface-soft); }.metadata-section h3 { margin: 0; font-size: 10px; }.metadata-section header span { color: var(--text-faint); font-size: var(--font-size-sm); }.metadata-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); margin: 0; }.metadata-grid > div { display: grid; min-width: 0; grid-template-columns: minmax(92px, .38fr) minmax(0, 1fr); gap: 8px; padding: 8px 11px; border-bottom: 1px solid var(--line); }.metadata-grid > div:nth-child(odd) { border-right: 1px solid var(--line); }.metadata-grid > div:nth-last-child(-n + 2) { border-bottom: 0; }.metadata-grid dt { color: var(--text-faint); font-size: var(--font-size-sm); }.metadata-grid dd { overflow: hidden; margin: 0; color: var(--text-soft); font: 9px ui-monospace, SFMono-Regular, Consolas, monospace; text-overflow: ellipsis; white-space: nowrap; }
.raw-details { overflow: hidden; border: 1px solid var(--line); border-radius: 11px; }.raw-details summary { display: flex; min-height: 36px; align-items: center; gap: 6px; padding: 8px 11px; color: var(--text-soft); background: var(--surface-soft); cursor: pointer; font-size: 9px; font-weight: 650; }.raw-details pre { max-height: 320px; margin: 0; overflow: auto; padding: 13px; color: #d7e0f2; background: #10182a; font: 9px/1.6 ui-monospace, SFMono-Regular, Consolas, monospace; }
:global(.connection-details-dialog .app-dialog__footer button) { min-height: 34px; padding: 6px 12px; font-size: 9px; }
.terminate-connection { margin-right: auto; }
@media (max-width: 680px) { :global(.app-dialog.connection-details-dialog) { width: 100%; max-width: 100%; }.detail-summary, .metadata-grid { grid-template-columns: 1fr; }.metadata-grid > div:nth-child(odd) { border-right: 0; }.metadata-grid > div:nth-last-child(2) { border-bottom: 1px solid var(--line); }.detail-traffic { grid-template-columns: 1fr; }.detail-state { display: none; } }
</style>
