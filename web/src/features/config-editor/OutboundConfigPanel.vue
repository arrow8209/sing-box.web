<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import { subscriptionApi } from "@/features/subscriptions/api";
import type { SubscriptionDetail, SubscriptionSummary } from "@/features/subscriptions/types";
import OutboundEditorDialog from "./OutboundEditorDialog.vue";
import {
  applyOutboundBufferEdit,
  isOutboundConfig,
  isObject,
  isPolicyOutbound,
  outboundDescription,
  outboundIcon,
  outboundMemberInitial,
  outboundReferenceTags,
  referencedSubscriptionOutboundTags,
  removeOutboundFromBuffer,
  updateOutboundFieldInBuffer,
  type OutboundConfig,
  type OutboundEditorResult
} from "./outbound";

const props = defineProps<{ draft: Draft; error?: string }>();
const emit = defineEmits<{ editJson: []; save: [text: string] }>();
const editorOpen = ref(false);
const editingIndex = ref<number | null>(null);
const preparing = ref(false);
const subscriptionSummaries = ref<SubscriptionSummary[]>([]);
const subscriptionDetails = ref<SubscriptionDetail[]>([]);
const sessionImportedTags = ref<Set<string>>(new Set());
const subscriptionError = ref("");
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");
let summaryRequest: Promise<void> | null = null;
let detailRequest: Promise<void> | null = null;

const rawItems = computed<unknown[]>(() => Array.isArray(props.draft.config.outbounds) ? props.draft.config.outbounds : []);
const entries = computed(() => rawItems.value.flatMap((value, index) => isOutboundConfig(value) ? [{ index, config: value }] : []));
const referencedTags = computed(() => new Set(entries.value.flatMap((entry) => isPolicyOutbound(entry.config) ? outboundReferenceTags(entry.config) : [])));
const subscriptionNodeTags = computed(() => new Set(referencedSubscriptionOutboundTags(
  rawItems.value,
  subscriptionDetails.value.flatMap((subscription) => subscription.outbounds)
)));
const hiddenNodeTags = computed(() => new Set([
  ...subscriptionNodeTags.value,
  ...[...sessionImportedTags.value].filter((tag) => referencedTags.value.has(tag))
]));
const visibleEntries = computed(() => entries.value.filter((entry) => !hiddenNodeTags.value.has(entry.config.tag)));
const invalidCount = computed(() => rawItems.value.length - entries.value.length);
const editing = computed(() => editingIndex.value === null ? null : entries.value.find((entry) => entry.index === editingIndex.value)?.config ?? null);
const dialogEditingIndex = computed(() => editingIndex.value === null ? null : entries.value.findIndex((entry) => entry.index === editingIndex.value));
const dialogOutbounds = computed(() => entries.value.map((entry) => entry.config));
const reservedTags = computed(() => rawItems.value.flatMap((item, index) => {
  if (index === editingIndex.value || !isObject(item) || typeof item.tag !== "string" || !item.tag.trim()) return [];
  return [item.tag.trim()];
}));
const healthEntry = computed(() => entries.value.find((entry) => entry.config.type === "urltest"));
const topology = computed(() => {
  const byTag = new Map(entries.value.map((entry) => [entry.config.tag, entry.config]));
  const route = props.draft.config.route;
  const routeFinal = route && typeof route === "object" && !Array.isArray(route) && typeof (route as Record<string, unknown>).final === "string"
    ? String((route as Record<string, unknown>).final)
    : "";
  let current = byTag.has(routeFinal)
    ? routeFinal
    : entries.value.find((entry) => entry.config.type === "selector")?.config.tag ?? entries.value[0]?.config.tag ?? "";
  const result = ["路由规则"];
  const visited = new Set<string>();
  while (current && result.length < 4 && !visited.has(current)) {
    visited.add(current);
    const config = byTag.get(current);
    result.push(config ? `${config.tag} · ${config.type}` : current);
    const next = config && isPolicyOutbound(config) ? outboundReferenceTags(config)[0] : "";
    current = next && byTag.has(next) ? next : "";
  }
  if (result.length === 1) result.push("尚未配置出站");
  result.push("Internet");
  return result;
});

onMounted(() => { void loadSubscriptionDetails(); });

function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}

function refreshSubscriptions(): Promise<void> {
  if (summaryRequest) return summaryRequest;
  summaryRequest = (async () => {
    try {
      subscriptionSummaries.value = await subscriptionApi.list();
      subscriptionError.value = "";
    } catch (cause) {
      subscriptionError.value = cause instanceof Error ? cause.message : "无法读取订阅列表";
    } finally {
      summaryRequest = null;
    }
  })();
  return summaryRequest;
}

function loadSubscriptionDetails(): Promise<void> {
  if (detailRequest) return detailRequest;
  detailRequest = (async () => {
    await refreshSubscriptions();
    const cached = new Map(subscriptionDetails.value.map((detail) => [detail.id, detail]));
    const missing = subscriptionSummaries.value.filter((summary) => !cached.has(summary.id));
    if (!missing.length) return;
    const results = await Promise.allSettled(missing.map((summary) => subscriptionApi.get(summary.id)));
    let failures = 0;
    results.forEach((result) => {
      if (result.status === "fulfilled") cached.set(result.value.id, result.value);
      else failures++;
    });
    subscriptionDetails.value = subscriptionSummaries.value.flatMap((summary) => {
      const detail = cached.get(summary.id);
      return detail ? [detail] : [];
    });
    if (failures) subscriptionError.value = `有 ${failures} 个订阅暂时无法读取，仍可编辑其他引用。`;
  })().finally(() => { detailRequest = null; });
  return detailRequest;
}

async function openEditor(index: number | null): Promise<void> {
  preparing.value = true;
  try {
    await loadSubscriptionDetails();
  } catch (cause) {
    subscriptionError.value = cause instanceof Error ? cause.message : "无法读取订阅节点";
  } finally {
    editingIndex.value = index;
    editorOpen.value = true;
    preparing.value = false;
  }
}

function closeEditor(): void {
  editorOpen.value = false;
  editingIndex.value = null;
}

function bufferText(): string {
  return props.draft.buffers.outbounds ?? JSON.stringify(rawItems.value, null, 2);
}

function submit(result: OutboundEditorResult): void {
  const wasEditing = editingIndex.value !== null;
  const merged = applyOutboundBufferEdit(bufferText(), rawItems.value, editingIndex.value, result);
  if (merged.imports.length) {
    sessionImportedTags.value = new Set([...sessionImportedTags.value, ...merged.imports.map((config) => config.tag)]);
  }
  emit("save", merged.text);
  closeEditor();
  const imported = merged.importedCount ? `，并复制 ${merged.importedCount} 个订阅节点` : "";
  show(`${result.config.tag} 已${wasEditing ? "更新" : "添加"}${imported}。`, "success");
}

function remove(index: number, config: OutboundConfig): void {
  emit("save", removeOutboundFromBuffer(bufferText(), rawItems.value, index));
  show(`${config.tag} 已从当前配置移除；现有引用将在应用检查中提示。`, "success");
}

function updateHealth(key: "url" | "interval" | "tolerance", event: Event): void {
  if (!healthEntry.value) return;
  const target = event.currentTarget as HTMLInputElement;
  let value: string | number = target.value.trim();
  if (key === "tolerance") {
    const tolerance = Number(target.value);
    if (!Number.isFinite(tolerance) || tolerance < 0) {
      show("测试容差必须是非负数。", "error");
      return;
    }
    value = tolerance;
  }
  emit("save", updateOutboundFieldInBuffer(bufferText(), rawItems.value, healthEntry.value.index, key, value));
  show("健康检查参数已更新。", "success");
}

function cardMembers(config: OutboundConfig): string[] {
  const references = outboundReferenceTags(config);
  if (references.length) return references;
  if (config.type === "direct") return ["本地"];
  if (config.type === "block") return ["阻断"];
  return [];
}

function countLabel(config: OutboundConfig): string {
  if (isPolicyOutbound(config)) return `${outboundReferenceTags(config).length} 个引用`;
  if (config.type === "direct" || config.type === "block") return "内置出站";
  return "完整 JSON";
}

function tone(config: OutboundConfig): string {
  if (config.type === "urltest") return "cyan";
  if (config.type === "direct") return "green";
  if (config.type === "block") return "orange";
  if (config.type !== "selector") return "violet";
  return "";
}
</script>

<template>
  <section class="outbound-panel">
    <div class="outbound-layout">
      <article class="outbound-list-card">
        <header><div><h2>出站与策略组</h2><p>决定流量最终使用的出口</p></div><div><button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />编辑全部 JSON</button><button class="add-button" type="button" :disabled="preparing" @click="openEditor(null)"><AppIcon name="plus" :size="14" />{{ preparing ? "读取引用中" : "新增出站" }}</button></div></header>
        <div class="outbound-grid">
          <article v-for="entry in visibleEntries" :key="entry.index" class="outbound-card" :data-outbound-tag="entry.config.tag">
            <div class="outbound-head"><span class="entry-icon" :class="tone(entry.config)"><AppIcon :name="outboundIcon(entry.config)" :size="18" /></span><div class="outbound-name"><strong>{{ entry.config.tag }}</strong><span>{{ outboundDescription(entry.config) }}</span></div><div class="outbound-actions"><button type="button" :aria-label="`编辑出站 ${entry.config.tag}`" title="编辑出站" :disabled="preparing" @click="openEditor(entry.index)"><AppIcon name="edit" :size="14" /></button><button class="danger" type="button" :aria-label="`删除出站 ${entry.config.tag}`" title="删除出站" :disabled="preparing" @click="remove(entry.index, entry.config)"><AppIcon name="trash" :size="14" /></button></div></div>
            <div class="outbound-summary"><div class="entry-meta"><b>{{ entry.config.type }}</b><span>{{ countLabel(entry.config) }}</span><em v-if="entry.config.hidden === true">隐藏</em></div><div class="outbound-members"><span v-for="(member, index) in cardMembers(entry.config).slice(0, 3)" :key="`${member}-${index}`" class="member-avatar" :style="{ zIndex: 3 - index }" :title="member">{{ outboundMemberInitial(member) }}</span><span v-if="cardMembers(entry.config).length > 3" class="member-avatar more">+{{ cardMembers(entry.config).length - 3 }}</span></div></div>
          </article>
          <button v-if="!visibleEntries.length" class="empty-state" type="button" :disabled="preparing" @click="openEditor(null)"><AppIcon name="outbound" :size="31" /><strong>尚未添加出站</strong><span>点击创建第一个出站或策略组</span></button>
          <p v-if="invalidCount" class="invalid-items" role="alert"><AppIcon name="info" :size="14" />还有 {{ invalidCount }} 个无法结构化显示的数组项，请使用“编辑全部 JSON”处理。</p>
        </div>
      </article>

      <aside>
        <article class="side-card"><header><div><h2>默认链路</h2><p>当前出站拓扑</p></div></header><div class="topology"><div v-for="(node, index) in topology" :key="`${node}-${index}`" class="topology-node" :class="{ active: index === 0 }"><span class="topology-dot" />{{ node }}</div></div></article>
        <article class="side-card"><header><div><h2>健康检查</h2><p>策略组延迟检测参数</p></div></header><div class="health-fields">
          <label><span>测试地址</span><input aria-label="测试地址" :value="healthEntry?.config.url ?? 'https://www.gstatic.com/generate_204'" :disabled="!healthEntry" @change="updateHealth('url', $event)" /></label>
          <label><span>测试间隔</span><span class="health-suffix"><input aria-label="测试间隔" :value="healthEntry?.config.interval ?? '3m'" :disabled="!healthEntry" @change="updateHealth('interval', $event)" /></span></label>
          <label><span>容差</span><span class="health-suffix"><input aria-label="容差" type="number" min="0" :value="healthEntry?.config.tolerance ?? 150" :disabled="!healthEntry" @change="updateHealth('tolerance', $event)" /><b>ms</b></span></label>
          <p v-if="!healthEntry"><AppIcon name="info" :size="13" />添加“自动选择”出站后可在此快速调整。</p>
        </div></article>
        <p v-if="subscriptionError" class="subscription-error" role="status"><AppIcon name="info" :size="14" />{{ subscriptionError }}</p>
      </aside>
    </div>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <OutboundEditorDialog :open="editorOpen" :outbound="editing" :editing-index="dialogEditingIndex" :outbounds="dialogOutbounds" :reserved-tags="reservedTags" :subscriptions="subscriptionDetails" @close="closeEditor" @submit="submit" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.outbound-panel { display: grid; gap: 10px; }.outbound-layout { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(230px, .65fr); gap: 12px; }.outbound-list-card, .side-card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.outbound-list-card > header, .side-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 14px 15px; border-bottom: 1px solid var(--line); }.outbound-list-card h2, .side-card h2, .outbound-list-card header p, .side-card header p { margin: 0; }.outbound-list-card h2, .side-card h2 { font-size: 12px; }.outbound-list-card header p, .side-card header p { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.outbound-list-card header > div:last-child { display: flex; gap: 6px; }.outbound-list-card header button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; font-size: 9px; }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.add-button { color: #fff; background: var(--primary); }.outbound-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; padding: 10px; }.outbound-card { position: relative; min-width: 0; overflow: hidden; padding: 11px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); transition: border-color 150ms ease, box-shadow 150ms ease, transform 150ms ease; }.outbound-card:hover { border-color: color-mix(in srgb, var(--primary) 25%, var(--line)); box-shadow: 0 8px 22px rgb(34 50 89 / 7%); transform: translateY(-2px); }.outbound-head { display: flex; align-items: flex-start; gap: 11px; }.entry-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.entry-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.entry-icon.orange { color: var(--orange); background: var(--orange-soft); }.entry-icon.green { color: var(--green); background: var(--green-soft); }.entry-icon.violet { color: var(--violet); background: color-mix(in srgb, var(--violet) 11%, var(--surface)); }.outbound-name { min-width: 0; flex: 1; }.outbound-name strong, .outbound-name span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.outbound-name strong { color: var(--text); font-size: 12px; }.outbound-name span { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.edit-button { display: grid; width: 30px; min-height: 30px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.edit-button:hover { color: var(--primary); background: var(--primary-soft); }.outbound-summary { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 10px; margin-top: 11px; }.entry-meta { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 5px; color: var(--text-faint); font-size: 8px; }.entry-meta b { padding: 2px 6px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 700 8px ui-monospace, monospace; }.entry-meta em { padding: 2px 6px; border-radius: 999px; color: var(--orange); background: var(--orange-soft); font-style: normal; font-weight: 680; }.outbound-members { display: flex; flex: 0 0 auto; }.member-avatar { display: grid; width: 27px; height: 27px; margin-left: -5px; place-items: center; border: 2px solid var(--surface); border-radius: 50%; color: #fff; background: var(--primary); font-size: 8px; font-weight: 720; }.member-avatar:first-child { margin-left: 0; }.member-avatar:nth-child(2) { background: var(--cyan); }.member-avatar:nth-child(3) { background: var(--violet); }.member-avatar.more { color: var(--text-soft); background: var(--surface-soft); }.empty-state { display: grid; min-height: 235px; grid-column: 1 / -1; place-items: center; align-content: center; gap: 7px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface-soft); }.empty-state strong { color: var(--text); font-size: 12px; }.empty-state span { color: var(--text-faint); font-size: 9px; font-weight: 400; }.invalid-items { display: flex; grid-column: 1 / -1; align-items: center; gap: 7px; margin: 0; padding: 9px; border-radius: 8px; color: var(--orange); background: var(--orange-soft); font-size: 9px; }.outbound-layout aside { display: grid; align-content: start; gap: 12px; }.topology { padding: 6px 4px 8px; }.topology-node { display: flex; align-items: center; gap: 9px; padding: 8px; border-radius: 9px; color: var(--text-soft); font-size: 9px; }.topology-node + .topology-node { position: relative; margin-top: 7px; }.topology-node + .topology-node::before { position: absolute; top: -10px; left: 21px; width: 1px; height: 12px; content: ""; background: var(--line-strong); }.topology-dot { width: 9px; height: 9px; border: 2px solid var(--surface); border-radius: 50%; background: var(--primary); box-shadow: 0 0 0 2px var(--primary-soft); }.topology-node.active { color: var(--text); background: var(--surface-soft); font-weight: 650; }.health-fields { display: grid; gap: 9px; padding: 12px; }.health-fields label { display: grid; gap: 5px; color: var(--text-soft); font-size: 8px; font-weight: 650; }.health-fields input { width: 100%; min-height: 36px; padding: 7px 9px; font-size: 9px; }.health-fields input:disabled { opacity: .62; }.health-suffix { position: relative; }.health-suffix input { padding-right: 36px; }.health-suffix b { position: absolute; top: 50%; right: 9px; color: var(--text-faint); font-size: 8px; transform: translateY(-50%); }.health-fields > p, .subscription-error { display: flex; align-items: flex-start; gap: 6px; margin: 0; color: var(--text-faint); font-size: 8px; }.subscription-error { padding: 10px; border: 1px solid var(--line); border-radius: 10px; color: var(--orange); background: var(--surface); }.panel-error { margin: 0; color: var(--red); font-size: 10px; }
.outbound-actions { display: flex; gap: 2px; }.outbound-actions button { display: grid; width: 30px; min-height: 30px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.outbound-actions button:hover { color: var(--primary); background: var(--primary-soft); }.outbound-actions button.danger:hover { color: var(--red); background: var(--red-soft); }
@media (max-width: 900px) { .outbound-layout { grid-template-columns: 1fr; }.outbound-layout aside { grid-template-columns: 1fr 1fr; }.subscription-error { grid-column: 1 / -1; } }
@media (max-width: 680px) { .outbound-list-card > header { flex-direction: column; }.outbound-list-card header > div:last-child { width: 100%; }.outbound-list-card header button { flex: 1; justify-content: center; }.outbound-grid { grid-template-columns: 1fr; }.outbound-layout aside { grid-template-columns: 1fr; }.subscription-error { grid-column: auto; }.outbound-card { padding: 10px; } }
</style>
