<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { runtimeModeApi, type RouteMode, type RuntimeMode, type RuntimeModeApi } from "./api";

const props = withDefaults(defineProps<{ running: boolean; busy?: boolean; runtimeKey?: string; api?: RuntimeModeApi }>(), {
  busy: false,
  runtimeKey: "",
  api: () => runtimeModeApi
});
const emit = defineEmits<{ notice: [message: string] }>();
const modes = [{ value: "rule", label: "规则" }, { value: "global", label: "全局" }, { value: "direct", label: "直连" }] as const;
const snapshot = ref<RuntimeMode | null>(null);
const error = ref("");
const loading = ref(false);
const switching = ref<RouteMode | null>(null);
let alive = true;
let generation = 0;
let readVersion = 0;
let reading = false;
let pollTimer: ReturnType<typeof setInterval> | undefined;

const statusLabel = computed(() => {
  if (!props.running) return "核心未运行";
  if (props.busy) return "核心状态变更中";
  if (switching.value) return "正在切换";
  if (loading.value && !snapshot.value) return "正在读取";
  return snapshot.value ? "对新连接生效" : "状态不可用";
});
const currentLabel = computed(() => modes.find((mode) => mode.value === snapshot.value?.mode)?.label ?? snapshot.value?.mode);
const unavailableModes = computed(() => snapshot.value ? modes.filter((mode) => !snapshot.value!.availableModes.includes(mode.value)).map((mode) => mode.label).join("、") : "");

function checkedSnapshot(value: RuntimeMode): RuntimeMode {
  if (!value || typeof value.mode !== "string" || !value.mode.trim() || !Array.isArray(value.availableModes)
    || value.availableModes.some((mode) => !modes.some((item) => item.value === mode))) throw new Error("核心返回的模式状态无效");
  return value;
}

function isCurrent(value: number): boolean { return alive && value === generation; }

async function refresh(): Promise<void> {
  if (!alive || !props.running || props.busy || reading || switching.value) return;
  const currentGeneration = generation;
  const version = ++readVersion;
  reading = true;
  loading.value = true;
  try {
    const result = checkedSnapshot(await props.api.get());
    if (!isCurrent(currentGeneration) || version !== readVersion) return;
    snapshot.value = result;
    error.value = "";
  } catch (cause) {
    if (!isCurrent(currentGeneration) || version !== readVersion) return;
    snapshot.value = null;
    error.value = cause instanceof Error ? cause.message : "无法读取核心当前模式";
  } finally {
    if (isCurrent(currentGeneration)) { reading = false; loading.value = false; }
  }
}

function canSelect(mode: RouteMode): boolean {
  return props.running && !props.busy && !switching.value && Boolean(snapshot.value?.availableModes.includes(mode));
}

async function selectMode(mode: RouteMode): Promise<void> {
  if (!canSelect(mode) || snapshot.value?.mode === mode) return;
  const currentGeneration = generation;
  // Ignore any poll started before this change, even if it returns afterwards.
  readVersion++;
  switching.value = mode;
  error.value = "";
  try {
    const result = checkedSnapshot(await props.api.set(mode));
    if (!isCurrent(currentGeneration)) return;
    if (result.mode !== mode) throw new Error("核心未确认模式切换，请刷新后重试");
    snapshot.value = result;
    emit("notice", `已切换到${currentLabel.value}模式，对新连接生效。`);
  } catch (cause) {
    if (!isCurrent(currentGeneration)) return;
    const message = cause instanceof Error ? cause.message : "模式切换失败";
    // A lost response does not prove the core rejected the change. Read its
    // state again instead of assuming that the previous mode is still active.
    snapshot.value = null;
    try {
      const current = checkedSnapshot(await props.api.get());
      if (isCurrent(currentGeneration)) snapshot.value = current;
    } catch { /* Keep the actual mode unknown until a successful refresh. */ }
    if (isCurrent(currentGeneration)) error.value = message;
  } finally {
    if (isCurrent(currentGeneration)) switching.value = null;
  }
}

watch([() => props.running, () => props.busy, () => props.runtimeKey], () => {
  generation++;
  readVersion++;
  reading = false;
  loading.value = false;
  switching.value = null;
  snapshot.value = null;
  error.value = "";
  void refresh();
}, { immediate: true });

function onVisibility(): void { if (!document.hidden) void refresh(); }

onMounted(() => {
  pollTimer = setInterval(onVisibility, 5000);
  document.addEventListener("visibilitychange", onVisibility);
});
onBeforeUnmount(() => {
  alive = false;
  generation++;
  if (pollTimer) clearInterval(pollTimer);
  document.removeEventListener("visibilitychange", onVisibility);
});
defineExpose({ refresh });
</script>

<template>
  <article class="card route-mode-card" :aria-busy="loading || Boolean(switching)">
    <div class="mode-title-row"><h2>路由模式</h2><span role="status">{{ statusLabel }}</span></div>
    <div class="mode-options" role="group" aria-label="路由模式">
      <button v-for="mode in modes" :key="mode.value" class="mode-option" :class="{ active: snapshot?.mode === mode.value }" type="button" :aria-pressed="snapshot?.mode === mode.value" :disabled="!canSelect(mode.value)" @click="selectMode(mode.value)">{{ mode.label }}</button>
    </div>
    <p v-if="!running" class="mode-hint">启动核心后可切换路由模式。</p>
    <p v-else-if="snapshot" class="mode-hint">当前模式：{{ currentLabel }}<template v-if="unavailableModes">。当前配置未启用{{ unavailableModes }}模式。</template></p>
    <p v-if="error" class="mode-error" role="alert">{{ error }}<button type="button" :disabled="loading || Boolean(switching) || !running || busy" @click="refresh">重试</button></p>
  </article>
</template>

<style scoped>
.route-mode-card { display: flex; min-width: 0; flex-direction: column; padding: 10px; }
.mode-title-row { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 3px; }
.mode-title-row h2 { margin: 0; color: var(--text); font-size: var(--font-size-heading); font-weight: 710; }
.mode-title-row > span { color: var(--text-faint); font-size: var(--font-size-xs); }
.mode-options { display: grid; flex: 1; grid-template-rows: repeat(3, minmax(35px, 1fr)); gap: 5px; margin-top: 9px; padding: 4px; border-radius: 10px; background: var(--surface-soft); }
.mode-option { min-height: 35px; border-radius: 7px; color: var(--text-faint); background: transparent; font-size: var(--font-size-sm); font-weight: 650; transition: color 150ms ease, background 150ms ease, box-shadow 150ms ease; }
.mode-option:hover:not(:disabled) { color: var(--text); }
.mode-option.active { color: var(--primary); background: var(--surface); box-shadow: 0 2px 8px rgb(32 47 79 / 8%); }
.mode-option:disabled { cursor: not-allowed; opacity: .6; }
.mode-option:focus-visible, .mode-error button:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; }
.mode-hint, .mode-error { margin: 8px 0 0; color: var(--text-faint); font-size: var(--font-size-xs); overflow-wrap: anywhere; }
.mode-error { color: var(--red); }
.mode-error button { margin-left: 6px; color: var(--primary); background: transparent; text-decoration: underline; }
@media (max-width: 980px) {
  .mode-options { grid-template-columns: repeat(3, minmax(0, 1fr)); grid-template-rows: none; }
}
@media (prefers-reduced-motion: reduce) { .mode-option { transition: none; } }
</style>
