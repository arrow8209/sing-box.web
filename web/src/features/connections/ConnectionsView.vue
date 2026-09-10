<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ApiError } from "@/api/client";
import AppDialog from "@/components/AppDialog.vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { connectionsApi, type ConnectionsApi } from "./api";
import ConnectionDetailsDialog from "./ConnectionDetailsDialog.vue";
import {
  connectionChains,
  connectionDestination,
  connectionHost,
  connectionRule,
  connectionSearchText,
  connectionSource,
  connectionType,
  formatBytes,
  formatDuration,
  formatSpeed,
  formatStarted
} from "./format";
import type { ConnectionsSnapshot, DisplayConnection, RuntimeConnection } from "./types";

type ConnectionMode = "active" | "closed";
type ColumnKey = "type" | "host" | "source" | "destination" | "rule" | "chains" | "uploadSpeed" | "downloadSpeed" | "traffic" | "start";

const props = withDefaults(defineProps<{ api?: ConnectionsApi; pollInterval?: number }>(), {
  api: () => connectionsApi,
  pollInterval: 1000
});

const columnOptions: Array<{ key: ColumnKey; label: string }> = [
  { key: "type", label: "类型" },
  { key: "host", label: "主机" },
  { key: "source", label: "源地址" },
  { key: "destination", label: "目标地址" },
  { key: "rule", label: "匹配规则" },
  { key: "chains", label: "链路" },
  { key: "uploadSpeed", label: "上行速度" },
  { key: "downloadSpeed", label: "下行速度" },
  { key: "traffic", label: "累计流量" },
  { key: "start", label: "连接时间" }
];
const defaultColumns = columnOptions.map((column) => column.key);
const preferenceKey = "singbox-connections-columns";

const activeConnections = ref<DisplayConnection[]>([]);
const closedConnections = ref<DisplayConnection[]>([]);
const snapshot = ref<ConnectionsSnapshot>({ memory: 0, uploadTotal: 0, downloadTotal: 0, connections: [] });
const mode = ref<ConnectionMode>("active");
const keywords = ref("");
const paused = ref(false);
const loading = ref(true);
const refreshing = ref(false);
const closingAll = ref(false);
const closingIds = ref<Set<string>>(new Set());
const error = ref("");
const notice = ref("");
const lastUpdatedAt = ref<number | null>(null);
const now = ref(Date.now());
const selectedConnection = ref<DisplayConnection | null>(null);
const detailsOpen = ref(false);
const settingsOpen = ref(false);
const visibleColumns = ref<Set<ColumnKey>>(new Set(defaultColumns));
const sort = reactive<{ key: ColumnKey; direction: "asc" | "desc" }>({ key: "start", direction: "desc" });
let pollTimer: ReturnType<typeof setInterval> | undefined;
let clockTimer: ReturnType<typeof setInterval> | undefined;
let previousTraffic = new Map<string, { upload: number; download: number; observedAt: number }>();
const suppressedConnectionIds = new Set<string>();
let alive = true;

const currentConnections = computed(() => mode.value === "active" ? activeConnections.value : closedConnections.value);
const filteredConnections = computed(() => {
  const keyword = keywords.value.trim().toLocaleLowerCase();
  const result = keyword ? currentConnections.value.filter((connection) => connectionSearchText(connection).includes(keyword)) : [...currentConnections.value];
  const direction = sort.direction === "asc" ? 1 : -1;
  return result.sort((left, right) => compare(sortValue(left, sort.key), sortValue(right, sort.key)) * direction);
});
const lastUpdatedLabel = computed(() => {
  if (!lastUpdatedAt.value) return "尚未同步";
  const seconds = Math.max(0, Math.floor((now.value - lastUpdatedAt.value) / 1000));
  return seconds < 2 ? "刚刚更新" : `${seconds} 秒前更新`;
});

function normalizeConnection(connection: RuntimeConnection): RuntimeConnection {
  return {
    ...connection,
    metadata: connection.metadata ?? {},
    chains: Array.isArray(connection.chains) ? connection.chains : [],
    upload: Number.isFinite(connection.upload) ? connection.upload : 0,
    download: Number.isFinite(connection.download) ? connection.download : 0
  };
}

function reconcile(value: ConnectionsSnapshot, observedAt: number): void {
  const incoming = (Array.isArray(value.connections) ? value.connections : []).filter((connection) => !suppressedConnectionIds.has(connection.id));
  const incomingIds = new Set(incoming.map((connection) => connection.id));
  for (const connection of activeConnections.value) {
    if (!incomingIds.has(connection.id)) archive(connection, new Date(observedAt).toISOString());
  }
  const nextTraffic = new Map<string, { upload: number; download: number; observedAt: number }>();
  const next = incoming.map((item) => {
    const connection = normalizeConnection(item);
    const previous = previousTraffic.get(connection.id);
    const seconds = previous ? Math.max((observedAt - previous.observedAt) / 1000, 0.001) : 0;
    const display: DisplayConnection = {
      ...connection,
      uploadSpeed: previous ? Math.max(0, connection.upload - previous.upload) / seconds : 0,
      downloadSpeed: previous ? Math.max(0, connection.download - previous.download) / seconds : 0
    };
    nextTraffic.set(connection.id, { upload: connection.upload, download: connection.download, observedAt });
    return display;
  });
  activeConnections.value = next;
  previousTraffic = nextTraffic;
  snapshot.value = { ...value, connections: incoming };
  if (selectedConnection.value && !selectedConnection.value.closedAt) {
    selectedConnection.value = next.find((connection) => connection.id === selectedConnection.value?.id) ?? selectedConnection.value;
  }
}

async function refreshConnections(force = false): Promise<void> {
  if (refreshing.value || (!force && paused.value) || (!force && document.hidden)) return;
  refreshing.value = true;
  try {
    const value = await props.api.list();
    if (!alive) return;
    const observedAt = Date.now();
    reconcile(value, observedAt);
    lastUpdatedAt.value = observedAt;
    error.value = "";
  } catch (cause) {
    if (alive) {
      if (cause instanceof ApiError && ["core_not_running", "clash_api_not_configured"].includes(cause.body.code)) {
        const closedAt = new Date().toISOString();
        for (const connection of activeConnections.value) archive(connection, closedAt);
        activeConnections.value = [];
        previousTraffic.clear();
        snapshot.value = { ...snapshot.value, connections: [] };
      }
      error.value = cause instanceof Error ? cause.message : "无法读取活动连接";
    }
  } finally {
    if (alive) {
      loading.value = false;
      refreshing.value = false;
    }
  }
}

function archive(connection: DisplayConnection, closedAt: string): DisplayConnection {
  const archived = { ...connection, closedAt };
  closedConnections.value = [archived, ...closedConnections.value.filter((item) => item.id !== connection.id)].slice(0, 500);
  if (selectedConnection.value?.id === connection.id && !selectedConnection.value.closedAt) selectedConnection.value = archived;
  return archived;
}

function removeActive(connection: DisplayConnection, closedAt: string): void {
  suppressedConnectionIds.add(connection.id);
  archive(connection, closedAt);
  activeConnections.value = activeConnections.value.filter((item) => item.id !== connection.id);
  previousTraffic.delete(connection.id);
}

async function closeConnection(connection: DisplayConnection): Promise<void> {
  if (closingIds.value.has(connection.id)) return;
  closingIds.value = new Set([...closingIds.value, connection.id]);
  try {
    await props.api.close(connection.id);
    removeActive(connection, new Date().toISOString());
    showNotice(`已关闭 ${connectionHost(connection)} 的连接。`);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "关闭连接失败";
  } finally {
    const next = new Set(closingIds.value);
    next.delete(connection.id);
    closingIds.value = next;
  }
}

async function closeFilteredConnections(): Promise<void> {
  const targets = [...filteredConnections.value];
  if (!targets.length || closingAll.value) return;
  closingAll.value = true;
  const closedAt = new Date().toISOString();
  try {
    if (!keywords.value.trim() && targets.length === activeConnections.value.length) {
      await props.api.closeAll();
      for (const connection of targets) removeActive(connection, closedAt);
      showNotice(`已关闭 ${targets.length} 条活动连接。`);
      return;
    }
    const results = await Promise.allSettled(targets.map((connection) => props.api.close(connection.id)));
    let succeeded = 0;
    results.forEach((result, index) => {
      if (result.status !== "fulfilled") return;
      const connection = targets[index];
      if (connection) { removeActive(connection, closedAt); succeeded += 1; }
    });
    const failed = targets.length - succeeded;
    if (failed) error.value = `${succeeded} 条连接已关闭，${failed} 条关闭失败`;
    else showNotice(`已关闭 ${succeeded} 条筛选结果。`);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "关闭连接失败";
  } finally {
    closingAll.value = false;
  }
}

function clearClosedConnections(): void {
  const count = closedConnections.value.length;
  closedConnections.value = [];
  if (selectedConnection.value?.closedAt) closeDetails();
  showNotice(count ? `已清除 ${count} 条已关闭记录。` : "没有需要清除的记录。");
}

function togglePause(): void {
  paused.value = !paused.value;
  if (!paused.value) void refreshConnections(true);
}

function showDetails(connection: DisplayConnection): void {
  selectedConnection.value = connection;
  detailsOpen.value = true;
}

function closeDetails(): void {
  detailsOpen.value = false;
  selectedConnection.value = null;
}

function showNotice(message: string): void {
  notice.value = "";
  queueMicrotask(() => { notice.value = message; });
}

function isVisible(key: ColumnKey): boolean { return visibleColumns.value.has(key); }

function toggleColumn(key: ColumnKey): void {
  const next = new Set(visibleColumns.value);
  if (next.has(key)) {
    if (next.size === 1) return;
    next.delete(key);
  } else next.add(key);
  visibleColumns.value = next;
}

function saveColumnPreferences(): void {
  localStorage.setItem(preferenceKey, JSON.stringify(columnOptions.filter((column) => visibleColumns.value.has(column.key)).map((column) => column.key)));
  settingsOpen.value = false;
  showNotice("连接列表显示字段已保存。");
}

function resetColumnPreferences(): void {
  visibleColumns.value = new Set(defaultColumns);
  localStorage.removeItem(preferenceKey);
}

function loadColumnPreferences(): void {
  try {
    const parsed = JSON.parse(localStorage.getItem(preferenceKey) ?? "[]") as unknown;
    if (!Array.isArray(parsed)) return;
    const allowed = new Set(defaultColumns);
    const selected = parsed.filter((key): key is ColumnKey => typeof key === "string" && allowed.has(key as ColumnKey));
    if (selected.length) visibleColumns.value = new Set(selected);
  } catch {
    localStorage.removeItem(preferenceKey);
  }
}

function changeSort(key: ColumnKey): void {
  if (sort.key === key) sort.direction = sort.direction === "asc" ? "desc" : "asc";
  else {
    sort.key = key;
    sort.direction = ["uploadSpeed", "downloadSpeed", "traffic", "start"].includes(key) ? "desc" : "asc";
  }
}

function ariaSort(key: ColumnKey): "ascending" | "descending" | "none" {
  if (sort.key !== key) return "none";
  return sort.direction === "asc" ? "ascending" : "descending";
}

function sortValue(connection: DisplayConnection, key: ColumnKey): string | number {
  switch (key) {
  case "type": return connectionType(connection);
  case "host": return connectionHost(connection);
  case "source": return connectionSource(connection);
  case "destination": return connectionDestination(connection);
  case "rule": return connectionRule(connection);
  case "chains": return connectionChains(connection);
  case "uploadSpeed": return connection.uploadSpeed;
  case "downloadSpeed": return connection.downloadSpeed;
  case "traffic": return connection.upload + connection.download;
  case "start": return new Date(connection.start).getTime() || 0;
  }
}

function compare(left: string | number, right: string | number): number {
  if (typeof left === "number" && typeof right === "number") return left - right;
  return String(left).localeCompare(String(right), "zh-CN", { numeric: true, sensitivity: "base" });
}

function onVisibilityChange(): void {
  if (!document.hidden && !paused.value) void refreshConnections(true);
}

onMounted(() => {
  loadColumnPreferences();
  void refreshConnections(true);
  pollTimer = setInterval(() => { void refreshConnections(); }, Math.max(500, props.pollInterval));
  clockTimer = setInterval(() => { now.value = Date.now(); }, 1000);
  document.addEventListener("visibilitychange", onVisibilityChange);
});

onBeforeUnmount(() => {
  alive = false;
  if (pollTimer) clearInterval(pollTimer);
  if (clockTimer) clearInterval(clockTimer);
  document.removeEventListener("visibilitychange", onVisibilityChange);
});
</script>

<template>
  <section class="connections-page">
    <header class="page-header connections-header">
      <div><h1>活动连接</h1><p>实时查看 sing-box 当前连接、路由链路与传输详情。</p></div>
      <div class="header-status" :class="{ paused, error: Boolean(error) }"><i />{{ error ? "连接数据不可用" : paused ? "已暂停刷新" : "实时同步中" }}</div>
    </header>

    <div class="connection-stats" aria-label="连接运行摘要">
      <article><span class="stat-icon green"><AppIcon name="link" :size="18" /></span><div><small>活动连接</small><strong>{{ activeConnections.length.toLocaleString("zh-CN") }}<b>条</b></strong></div></article>
      <article><span class="stat-icon"><AppIcon name="upload" :size="18" /></span><div><small>累计上传</small><strong>{{ formatBytes(snapshot.uploadTotal) }}</strong></div></article>
      <article><span class="stat-icon cyan"><AppIcon name="download" :size="18" /></span><div><small>累计下载</small><strong>{{ formatBytes(snapshot.downloadTotal) }}</strong></div></article>
      <article><span class="stat-icon orange"><AppIcon name="memory" :size="18" /></span><div><small>核心内存</small><strong>{{ formatBytes(snapshot.memory) }}</strong></div></article>
    </div>

    <article class="connections-panel">
      <div class="connection-toolbar">
        <div class="connection-tabs" role="tablist" aria-label="连接状态">
          <button type="button" role="tab" :aria-selected="mode === 'active'" :class="{ active: mode === 'active' }" @click="mode = 'active'"><span>活动</span><b>{{ activeConnections.length }}</b></button>
          <button type="button" role="tab" :aria-selected="mode === 'closed'" :class="{ active: mode === 'closed' }" @click="mode = 'closed'"><span>已关闭</span><b>{{ closedConnections.length }}</b></button>
        </div>
        <label class="connection-search"><AppIcon name="search" :size="15" /><input v-model="keywords" type="search" aria-label="搜索连接" placeholder="搜索主机、IP、进程、规则或链路" /></label>
        <div class="toolbar-actions">
          <button class="toolbar-button" type="button" :aria-label="paused ? '继续实时刷新' : '暂停实时刷新'" :title="paused ? '继续实时刷新' : '暂停实时刷新'" @click="togglePause"><AppIcon :name="paused ? 'play' : 'pause'" :size="15" /></button>
          <button class="toolbar-button" type="button" aria-label="立即刷新连接" title="立即刷新" :disabled="refreshing" @click="refreshConnections(true)"><AppIcon name="rotate" :size="15" /></button>
          <button v-if="mode === 'active'" class="toolbar-button danger-action" type="button" aria-label="关闭筛选出的连接" title="关闭筛选出的连接" :disabled="closingAll || !filteredConnections.length" @click="closeFilteredConnections"><AppIcon name="x" :size="15" /></button>
          <button v-else class="toolbar-button danger-action" type="button" aria-label="清除已关闭记录" title="清除已关闭记录" :disabled="!closedConnections.length" @click="clearClosedConnections"><AppIcon name="trash" :size="15" /></button>
          <button class="toolbar-button" type="button" aria-label="设置显示字段" title="设置显示字段" @click="settingsOpen = true"><AppIcon name="sliders" :size="15" /></button>
        </div>
      </div>

      <p v-if="error" class="connections-error" role="alert"><AppIcon name="info" :size="15" /><span>{{ error }}</span><button type="button" @click="refreshConnections(true)">重试</button></p>

      <div v-if="filteredConnections.length" class="connections-table-wrap">
        <table class="connections-table">
          <thead><tr>
            <th v-if="isVisible('type')" :aria-sort="ariaSort('type')"><button type="button" @click="changeSort('type')">类型<i>{{ sort.key === "type" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('host')" :aria-sort="ariaSort('host')"><button type="button" @click="changeSort('host')">主机<i>{{ sort.key === "host" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('source')" :aria-sort="ariaSort('source')"><button type="button" @click="changeSort('source')">源地址<i>{{ sort.key === "source" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('destination')" :aria-sort="ariaSort('destination')"><button type="button" @click="changeSort('destination')">目标地址<i>{{ sort.key === "destination" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('rule')" :aria-sort="ariaSort('rule')"><button type="button" @click="changeSort('rule')">匹配规则<i>{{ sort.key === "rule" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('chains')" :aria-sort="ariaSort('chains')"><button type="button" @click="changeSort('chains')">链路<i>{{ sort.key === "chains" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('uploadSpeed')" class="numeric" :aria-sort="ariaSort('uploadSpeed')"><button type="button" @click="changeSort('uploadSpeed')">上行速度<i>{{ sort.key === "uploadSpeed" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('downloadSpeed')" class="numeric" :aria-sort="ariaSort('downloadSpeed')"><button type="button" @click="changeSort('downloadSpeed')">下行速度<i>{{ sort.key === "downloadSpeed" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('traffic')" class="numeric" :aria-sort="ariaSort('traffic')"><button type="button" @click="changeSort('traffic')">累计流量<i>{{ sort.key === "traffic" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th v-if="isVisible('start')" :aria-sort="ariaSort('start')"><button type="button" @click="changeSort('start')">连接时间<i>{{ sort.key === "start" ? (sort.direction === "asc" ? "↑" : "↓") : "" }}</i></button></th>
            <th class="actions-column"><span>操作</span></th>
          </tr></thead>
          <tbody>
            <tr v-for="connection in filteredConnections" :key="`${mode}-${connection.id}`">
              <td v-if="isVisible('type')"><span class="connection-type">{{ connectionType(connection) }}</span></td>
              <td v-if="isVisible('host')"><button class="host-button" type="button" :title="connectionHost(connection)" @click="showDetails(connection)">{{ connectionHost(connection) }}</button><small>{{ connection.metadata.process || connection.metadata.processPath || connection.id }}</small></td>
              <td v-if="isVisible('source')"><code>{{ connectionSource(connection) }}</code></td>
              <td v-if="isVisible('destination')"><code>{{ connectionDestination(connection) }}</code></td>
              <td v-if="isVisible('rule')"><span class="ellipsis" :title="connectionRule(connection)">{{ connectionRule(connection) }}</span></td>
              <td v-if="isVisible('chains')"><span class="chain" :title="connectionChains(connection)">{{ connectionChains(connection) }}</span></td>
              <td v-if="isVisible('uploadSpeed')" class="numeric speed upload-speed">{{ formatSpeed(connection.uploadSpeed) }}</td>
              <td v-if="isVisible('downloadSpeed')" class="numeric speed download-speed">{{ formatSpeed(connection.downloadSpeed) }}</td>
              <td v-if="isVisible('traffic')" class="numeric"><strong>{{ formatBytes(connection.upload + connection.download) }}</strong><small>↑ {{ formatBytes(connection.upload) }} · ↓ {{ formatBytes(connection.download) }}</small></td>
              <td v-if="isVisible('start')"><span class="connection-time">{{ formatDuration(connection.start, now, connection.closedAt) }}</span><small>{{ formatStarted(connection.start) }}</small></td>
              <td class="actions-column"><div class="row-actions"><button type="button" :aria-label="`查看 ${connectionHost(connection)} 的连接详情`" title="查看详情" @click="showDetails(connection)"><AppIcon name="eye" :size="14" /></button><button v-if="mode === 'active'" class="close-row" type="button" :aria-label="`关闭 ${connectionHost(connection)} 的连接`" title="关闭连接" :disabled="closingIds.has(connection.id)" @click="closeConnection(connection)"><AppIcon name="x" :size="14" /></button></div></td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else class="connections-empty">
        <span class="empty-icon"><AppIcon :name="keywords ? 'search' : mode === 'active' ? 'link' : 'clock'" :size="27" /></span>
        <strong>{{ loading ? "正在读取连接" : keywords ? "没有匹配的连接" : mode === "active" ? "当前没有活动连接" : "尚无已关闭记录" }}</strong>
        <span>{{ loading ? "正在连接 sing-box 控制 API" : keywords ? "请调整搜索关键词后重试" : mode === "active" ? "新连接建立后会自动显示在这里" : "页面打开期间断开的连接会保留在这里" }}</span>
        <button v-if="keywords" type="button" @click="keywords = ''">清除搜索</button>
      </div>

      <footer class="connections-footer"><span>显示 {{ filteredConnections.length }} / {{ currentConnections.length }} 条</span><span><i :class="{ paused }" />{{ paused ? "刷新已暂停" : lastUpdatedLabel }}</span></footer>
    </article>

    <ConnectionDetailsDialog :open="detailsOpen" :connection="selectedConnection" :now="now" :closing="Boolean(selectedConnection && closingIds.has(selectedConnection.id))" @close="closeDetails" @terminate="closeConnection" />

    <AppDialog class="connection-settings-dialog" :open="settingsOpen" title="连接列表设置" description="选择表格中需要展示的字段；至少保留一项。" @close="settingsOpen = false">
      <div class="column-settings">
        <label v-for="column in columnOptions" :key="column.key"><span><strong>{{ column.label }}</strong><small>{{ column.key }}</small></span><input type="checkbox" :checked="visibleColumns.has(column.key)" :disabled="visibleColumns.size === 1 && visibleColumns.has(column.key)" @change="toggleColumn(column.key)" /></label>
      </div>
      <template #footer><button class="secondary reset-columns" type="button" @click="resetColumnPreferences">恢复默认</button><button class="secondary" type="button" @click="settingsOpen = false">取消</button><button type="button" @click="saveColumnPreferences">保存设置</button></template>
    </AppDialog>

    <AppToast v-if="notice" :message="notice" />
  </section>
</template>

<style scoped>
.connections-page { min-width: 0; }.connections-header { align-items: end; margin-bottom: 14px; }.connections-header h1 { font-size: 22px; }.connections-header p { margin-top: 3px; font-size: var(--font-size-sm); }.header-status { display: inline-flex; min-height: 28px; align-items: center; gap: 7px; padding: 5px 9px; border: 1px solid color-mix(in srgb, var(--green) 24%, var(--line)); border-radius: 999px; color: var(--green); background: var(--green-soft); font-size: var(--font-size-xs); font-weight: 680; }.header-status i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; box-shadow: 0 0 0 4px color-mix(in srgb, currentColor 12%, transparent); }.header-status.paused { color: var(--orange); border-color: color-mix(in srgb, var(--orange) 24%, var(--line)); background: var(--orange-soft); }.header-status.error { color: var(--red); border-color: color-mix(in srgb, var(--red) 24%, var(--line)); background: var(--red-soft); }
.connection-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 9px; margin-bottom: 11px; }.connection-stats article { display: flex; min-width: 0; align-items: center; gap: 10px; padding: 11px 12px; border: 1px solid var(--line); border-radius: 11px; background: var(--surface); box-shadow: var(--shadow-sm); }.stat-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.stat-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.stat-icon.green { color: var(--green); background: var(--green-soft); }.stat-icon.orange { color: var(--orange); background: var(--orange-soft); }.connection-stats div { min-width: 0; }.connection-stats small, .connection-stats strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.connection-stats small { color: var(--text-faint); font-size: var(--font-size-sm); }.connection-stats strong { margin-top: 1px; font-size: 14px; font-variant-numeric: tabular-nums; }.connection-stats strong b { margin-left: 3px; color: var(--text-faint); font-size: var(--font-size-sm); font-weight: 600; }
.connections-panel { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.connection-toolbar { display: grid; grid-template-columns: auto minmax(220px, 1fr) auto; align-items: center; gap: 9px; padding: 10px 11px; border-bottom: 1px solid var(--line); }.connection-tabs { display: flex; gap: 3px; padding: 3px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface-soft); }.connection-tabs button { display: flex; min-height: 30px; align-items: center; gap: 6px; padding: 5px 9px; border-radius: 7px; color: var(--text-soft); background: transparent; font-size: var(--font-size-sm); }.connection-tabs button.active { color: var(--primary); background: var(--surface); box-shadow: 0 1px 3px rgb(20 33 61 / 8%); }.connection-tabs b { display: inline-grid; min-width: 17px; min-height: 17px; place-items: center; padding: 1px 4px; border-radius: 999px; color: var(--text-faint); background: var(--line); font-size: var(--font-size-xs); }.connection-tabs button.active b { color: var(--primary); background: var(--primary-soft); }.connection-search { position: relative; display: block; }.connection-search > :first-child { position: absolute; z-index: 1; top: 50%; left: 10px; color: var(--text-faint); transform: translateY(-50%); }.connection-search input { width: 100%; min-height: 38px; padding: 7px 10px 7px 33px; background: var(--surface-soft); font-size: var(--font-size-base); }.toolbar-actions { display: flex; gap: 3px; }.toolbar-button { display: inline-grid; width: 34px; min-height: 34px; place-items: center; padding: 0; border: 1px solid transparent; color: var(--text-soft); background: transparent; }.toolbar-button:hover { color: var(--primary); border-color: var(--line); background: var(--primary-soft); }.toolbar-button.danger-action:hover { color: var(--red); background: var(--red-soft); }.toolbar-button:disabled { cursor: not-allowed; opacity: .4; }
.connections-error { display: flex; align-items: center; gap: 8px; margin: 10px 11px 0; padding: 9px 10px; border-radius: 8px; color: var(--red); background: var(--red-soft); font-size: var(--font-size-xs); }.connections-error span { min-width: 0; flex: 1; }.connections-error button { min-height: 27px; padding: 4px 8px; color: var(--red); background: var(--surface); font-size: var(--font-size-sm); }
.connections-table-wrap { overflow: auto; }.connections-table { width: 100%; min-width: 1380px; table-layout: auto; border-collapse: collapse; }.connections-table th { position: sticky; z-index: 1; top: 0; padding: 0; border-bottom: 1px solid var(--line); color: var(--text-faint); background: var(--surface-soft); text-align: left; white-space: nowrap; }.connections-table th button, .connections-table th > span { display: flex; width: 100%; min-height: 34px; align-items: center; gap: 4px; justify-content: flex-start; padding: 7px 10px; color: inherit; background: transparent; font-size: var(--font-size-sm); font-weight: 700; text-align: inherit; }.connections-table th button:hover { color: var(--primary); }.connections-table th i { color: var(--primary); font-style: normal; }.connections-table td { max-width: 230px; padding: 9px 10px; border-bottom: 1px solid var(--line); color: var(--text-soft); font-size: var(--font-size-base); vertical-align: middle; }.connections-table tbody tr:last-child td { border-bottom: 0; }.connections-table tbody tr:hover td { background: color-mix(in srgb, var(--surface-soft) 75%, transparent); }.connections-table td small { display: block; max-width: 220px; overflow: hidden; margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); text-overflow: ellipsis; white-space: nowrap; }.connections-table code { color: var(--text-soft); font: var(--font-size-sm) ui-monospace, SFMono-Regular, Consolas, monospace; white-space: nowrap; }.numeric { text-align: right !important; font-variant-numeric: tabular-nums; }.connections-table th.numeric button { justify-content: flex-end; }.connection-type { display: inline-flex; padding: 3px 6px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); white-space: nowrap; }.host-button { display: block; width: 100%; min-height: 0; overflow: hidden; padding: 0; color: var(--text); background: transparent; font-size: var(--font-size-sm); font-weight: 680; text-align: left; text-overflow: ellipsis; white-space: nowrap; }.host-button:hover { color: var(--primary); text-decoration: underline; }.ellipsis, .chain { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.chain { color: var(--primary); }.speed { white-space: nowrap; }.upload-speed { color: var(--violet); }.download-speed { color: var(--cyan); }.connection-time { color: var(--text); font-weight: 650; white-space: nowrap; }.actions-column { width: 72px; text-align: right !important; }.connections-table th.actions-column > span { justify-content: flex-end; }.row-actions { display: flex; justify-content: flex-end; gap: 2px; }.row-actions button { display: inline-grid; width: 27px; min-height: 27px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.row-actions button:hover { color: var(--primary); background: var(--primary-soft); }.row-actions .close-row:hover { color: var(--red); background: var(--red-soft); }
.connections-empty { display: grid; min-height: var(--empty-state-height); place-items: center; align-content: center; gap: 6px; padding: var(--card-padding); color: var(--text-faint); text-align: center; }.empty-icon { display: grid; width: 54px; height: 54px; place-items: center; margin-bottom: 3px; border-radius: 16px; color: var(--primary); background: var(--primary-soft); }.connections-empty strong { color: var(--text); font-size: var(--font-size-base); }.connections-empty > span:last-of-type { font-size: var(--font-size-xs); }.connections-empty button { min-height: 30px; margin-top: 4px; padding: 5px 9px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); }.connections-footer { display: flex; min-height: 35px; align-items: center; justify-content: space-between; gap: 10px; padding: 7px 11px; border-top: 1px solid var(--line); color: var(--text-faint); background: var(--surface-soft); font-size: var(--font-size-sm); }.connections-footer span:last-child { display: inline-flex; align-items: center; gap: 6px; }.connections-footer i { width: 5px; height: 5px; border-radius: 50%; background: var(--green); }.connections-footer i.paused { background: var(--orange); }
.column-settings { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }.column-settings label { display: flex; min-width: 0; min-height: 49px; align-items: center; gap: 9px; padding: 8px 10px; border: 1px solid var(--line); border-radius: 9px; background: var(--surface-soft); cursor: pointer; }.column-settings label > span { min-width: 0; flex: 1; }.column-settings strong, .column-settings small { display: block; }.column-settings strong { font-size: var(--font-size-base); }.column-settings small { margin-top: 1px; color: var(--text-faint); font: var(--font-size-xs) ui-monospace, monospace; }.column-settings input { width: 15px; min-height: 15px; accent-color: var(--primary); }.reset-columns { margin-right: auto; }
:global(.app-dialog.connection-settings-dialog) { width: min(620px, calc(100vw - 32px)); }:global(.connection-settings-dialog .app-dialog__footer button) { min-height: 34px; padding: 6px 10px; font-size: var(--font-size-xs); }
@media (max-width: 900px) { .connection-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }.connection-toolbar { grid-template-columns: auto minmax(0, 1fr); }.connection-search { grid-row: 2; grid-column: 1 / -1; }.toolbar-actions { justify-content: flex-end; } }
@media (max-width: 680px) { .connections-header { align-items: start; }.header-status { flex: 0 0 auto; }.connection-stats { gap: 7px; }.connection-stats article { gap: 8px; padding: 9px; }.stat-icon { width: 30px; height: 30px; }.connection-stats strong { font-size: var(--font-size-base); }.connection-toolbar { grid-template-columns: minmax(0, 1fr) auto; padding: 8px; }.connection-tabs { overflow-x: auto; }.toolbar-actions .toolbar-button:nth-child(2) { display: none; }.connections-error { margin-inline: 8px; }.connections-empty { min-height: var(--empty-state-height); }.column-settings { grid-template-columns: 1fr; }:global(.app-dialog.connection-settings-dialog) { width: 100%; max-width: 100%; }.connections-footer { position: sticky; bottom: 0; } }
</style>
