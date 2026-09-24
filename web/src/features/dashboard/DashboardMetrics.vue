<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { CoreProcessState } from "@/features/core/types";
import { formatBytes } from "@/features/connections/format";
import { runtimeMetricsApi, type RuntimeMetrics, type RuntimeMetricsApi } from "./api";

const props = withDefaults(defineProps<{ state?: CoreProcessState; busy?: boolean; runtimeKey?: string; api?: RuntimeMetricsApi }>(), {
  busy: false,
  runtimeKey: "",
  api: () => runtimeMetricsApi
});
const snapshot = ref<RuntimeMetrics | null>(null);
const uploadSpeed = ref<number | null>(null);
const downloadSpeed = ref<number | null>(null);
const error = ref("");
const loading = ref(false);
const hidden = ref(document.hidden);
let previous: RuntimeMetrics | null = null;
let alive = true;
let generation = 0;
let pending: { controller: AbortController; timer: ReturnType<typeof setTimeout> } | undefined;
let pollTimer: ReturnType<typeof setInterval> | undefined;

const canRead = computed(() => props.state === "running" && !props.busy && !hidden.value);
const statusLabel = computed(() => {
  if (!props.state) return "正在读取核心状态";
  if (props.busy || props.state === "starting" || props.state === "stopping") return "核心状态变更中，等待采样";
  if (props.state !== "running") return "核心未运行，暂无实时数据";
  if (hidden.value) return "页面已隐藏，暂停采样";
  if (error.value) return error.value;
  if (!snapshot.value) return "正在读取实时数据";
  if (uploadSpeed.value === null) return "正在采样流量速率";
  return "sing-box 实时数据 · 每秒更新";
});

function amount(value: number | null | undefined, speed = false): { value: string; unit: string } {
  if (value === null || value === undefined) return { value: "--", unit: "" };
  const [number, unit] = formatBytes(value, speed ? 2 : 1).split(" ");
  return { value: number!, unit: speed ? `${unit}/s` : unit! };
}

const metrics = computed(() => [
  { key: "upload", icon: "upload", tone: "", label: "实时上传", ...amount(uploadSpeed.value, true), title: "经 sing-box 转发的上行流量，包括 direct 出站" },
  { key: "download", icon: "download", tone: "cyan", label: "实时下载", ...amount(downloadSpeed.value, true), title: "经 sing-box 转发的下行流量，包括 direct 出站" },
  { key: "memory", icon: "memory", tone: "orange", label: "内存占用", ...amount(snapshot.value?.memory), title: "sing-box 核心自身报告的内存占用" },
  { key: "connections", icon: "link", tone: "green", label: "活动连接", value: snapshot.value ? String(snapshot.value.connectionCount) : "--", unit: snapshot.value ? "条" : "", href: "/connections", title: "查看 sing-box 当前活动连接" }
]);

function checkedSnapshot(value: RuntimeMetrics): RuntimeMetrics {
  if (!value || ![value.uploadTotal, value.downloadTotal, value.memory, value.connectionCount].every((counter) => Number.isSafeInteger(counter) && counter >= 0)
    || typeof value.runtimeKey !== "string" || !value.runtimeKey || typeof value.observedAt !== "string" || !Number.isFinite(Date.parse(value.observedAt))) {
    throw new Error("核心返回的运行指标不完整或无效");
  }
  return value;
}

function clearSample(): void {
  previous = null;
  snapshot.value = null;
  uploadSpeed.value = null;
  downloadSpeed.value = null;
}

async function refresh(): Promise<void> {
  if (!alive || !canRead.value || pending) return;
  const currentGeneration = generation;
  const controller = new AbortController();
  const isCurrent = (): boolean => alive && generation === currentGeneration;
  const request = {
    controller,
    timer: setTimeout(() => {
      if (!isCurrent()) return;
      clearSample();
      error.value = "实时数据读取超时，请重试";
      controller.abort();
    }, 5000)
  };
  pending = request;
  loading.value = true;
  try {
    const value = checkedSnapshot(await props.api.get(controller.signal));
    if (!isCurrent() || controller.signal.aborted) return;
    const elapsed = previous ? Date.parse(value.observedAt) - Date.parse(previous.observedAt) : 0;
    if (previous && previous.runtimeKey === value.runtimeKey && elapsed > 0 && elapsed <= 5000
      && value.uploadTotal >= previous.uploadTotal && value.downloadTotal >= previous.downloadTotal) {
      uploadSpeed.value = (value.uploadTotal - previous.uploadTotal) * 1000 / elapsed;
      downloadSpeed.value = (value.downloadTotal - previous.downloadTotal) * 1000 / elapsed;
    } else {
      uploadSpeed.value = null;
      downloadSpeed.value = null;
    }
    previous = value;
    snapshot.value = value;
    error.value = "";
  } catch (cause) {
    if (!isCurrent()) return;
    clearSample();
    error.value = controller.signal.aborted ? "实时数据读取超时，请重试" : cause instanceof Error ? cause.message : "无法读取 sing-box 实时数据";
  } finally {
    clearTimeout(request.timer);
    if (pending === request) { pending = undefined; loading.value = false; }
  }
}

function reset(): void {
  generation++;
  if (pending) { clearTimeout(pending.timer); pending.controller.abort(); pending = undefined; }
  loading.value = false;
  error.value = "";
  clearSample();
}

watch([() => props.state, () => props.busy, () => props.runtimeKey], () => { reset(); void refresh(); }, { immediate: true });

function onVisibility(): void {
  hidden.value = document.hidden;
  reset();
  if (!hidden.value) void refresh();
}

onMounted(() => {
  pollTimer = setInterval(() => { void refresh(); }, 1000);
  document.addEventListener("visibilitychange", onVisibility);
});
onBeforeUnmount(() => {
  alive = false;
  reset();
  if (pollTimer) clearInterval(pollTimer);
  document.removeEventListener("visibilitychange", onVisibility);
});
defineExpose({ refresh });
</script>

<template>
  <div class="dashboard-metrics">
    <div class="hero-metrics" aria-label="实时运行数据" :aria-busy="loading && !snapshot">
      <component :is="metric.href ? 'a' : 'div'" v-for="metric in metrics" :key="metric.key" class="hero-metric" :class="{ 'hero-metric-link': metric.href }" :href="metric.href" :title="metric.title" :data-metric="metric.key">
        <span class="hero-metric-icon" :class="metric.tone"><AppIcon :name="metric.icon" :size="14" /></span>
        <span class="hero-metric-copy"><span>{{ metric.label }}</span><strong>{{ metric.value }} <small v-if="metric.unit">{{ metric.unit }}</small></strong></span>
        <AppIcon v-if="metric.href" class="hero-metric-arrow" name="chevron" :size="12" />
      </component>
    </div>
    <p class="metrics-status" :role="error ? 'alert' : 'status'">{{ statusLabel }}<button v-if="error" type="button" :disabled="loading || !canRead" @click="refresh">重试</button></p>
  </div>
</template>

<style scoped>
.hero-metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 5px 6px; }
.hero-metric { display: flex; min-width: 0; min-height: 34px; align-items: center; gap: 5px; padding: 4px 6px; border: 1px solid rgb(255 255 255 / 13%); border-radius: 8px; color: #fff; background: rgb(255 255 255 / 10%); text-decoration: none; }
.hero-metric-link { transition: background 150ms ease, border-color 150ms ease; }
.hero-metric-link:hover { border-color: rgb(255 255 255 / 40%); background: rgb(255 255 255 / 20%); }
.hero-metric-link:focus-visible, .metrics-status button:focus-visible { outline: 2px solid #fff; outline-offset: 2px; }
.hero-metric-arrow { flex: 0 0 auto; opacity: .8; }
.hero-metric-icon { display: grid; width: 24px; height: 24px; flex: 0 0 auto; place-items: center; border-radius: 7px; color: #dce5ff; background: rgb(255 255 255 / 13%); }
.hero-metric-icon.cyan { color: #a4f4ff; }
.hero-metric-icon.orange { color: #ffe0ad; }
.hero-metric-icon.green { color: #aaf3d0; }
.hero-metric-copy { display: flex; min-width: 0; flex: 1; align-items: baseline; justify-content: space-between; gap: 4px; }
.hero-metric-copy > span { color: rgb(255 255 255 / 76%); font-size: var(--font-size-xs); white-space: nowrap; }
.hero-metric-copy strong { overflow: hidden; color: #fff; font-size: 15px; font-weight: 730; line-height: 1.2; text-overflow: ellipsis; white-space: nowrap; }
.hero-metric-copy small { font-size: var(--font-size-xs); font-weight: 550; }
.metrics-status { margin: 5px 0 0; color: rgb(255 255 255 / 80%); font-size: var(--font-size-xs); overflow-wrap: anywhere; }
.metrics-status button { margin-left: 6px; color: #fff; background: transparent; text-decoration: underline; }
@media (max-width: 430px) {
  .hero-metric-copy { display: block; }
  .hero-metric-copy > span, .hero-metric-copy strong { display: block; }
}
@media (max-width: 359px) { .hero-metric-icon { display: none; } }
@media (prefers-reduced-motion: reduce) { .hero-metric-link { transition: none; } }
</style>
