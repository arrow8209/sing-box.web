<script setup lang="ts">
import { computed, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import type { Draft } from "@/features/profiles/types";
import OutboundEditorDialog from "./OutboundEditorDialog.vue";
import { useOutboundEditor } from "./use-outbound-editor";
import {
  cloneValue,
  isObject,
  isPolicyOutbound,
  outboundDescription,
  outboundIcon,
  outboundReferenceTags,
  persistOutboundInputs,
  removeOutboundGroup,
  updateOutboundGroup,
  type OutboundEditorGroup,
  type OutboundEditorResult,
  type OutboundPanelSave
} from "./outbound";

const props = defineProps<{ draft: Draft; error?: string; serverResolution?: boolean }>();
const emit = defineEmits<{ editJson: []; save: [payload: OutboundPanelSave] }>();
const editorOpen = ref(false);
const editingId = ref<string | null>(null);
const preparing = ref(false);
const { rawItems, groups, subscriptionDetails, subscriptionError, loadSubscriptionDetails } = useOutboundEditor(() => props.draft);
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");
const editing = computed(() => groups.value.find((group) => group.id === editingId.value) ?? null);
const reservedTags = computed(() => rawItems.value.flatMap((item) => {
  if (!isObject(item) || typeof item.tag !== "string" || !item.tag.trim() || item.tag === editing.value?.tag) return [];
  return [item.tag.trim()];
}));
const healthGroup = computed(() => groups.value.find((group) => group.type === "urltest"));
const topology = computed(() => {
  const byTag = new Map(groups.value.map((group) => [group.tag, group.config]));
  const route = props.draft.config.route;
  const routeFinal = route && typeof route === "object" && !Array.isArray(route) && typeof (route as Record<string, unknown>).final === "string"
    ? String((route as Record<string, unknown>).final)
    : "";
  let current = byTag.has(routeFinal) ? routeFinal : groups.value.find((group) => group.type === "selector")?.tag ?? groups.value[0]?.tag ?? "";
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

function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}

async function openEditor(id: string | null): Promise<void> {
  preparing.value = true;
  try {
    await loadSubscriptionDetails();
  } catch (cause) {
    subscriptionError.value = cause instanceof Error ? cause.message : "无法读取订阅节点";
  } finally {
    editingId.value = id;
    editorOpen.value = true;
    preparing.value = false;
  }
}

function closeEditor(): void {
  editorOpen.value = false;
  editingId.value = null;
}

function persist(nextGroups: OutboundEditorGroup[], renamed?: { from: string; to: string }): boolean {
  try { emit("save", persistOutboundInputs(props.draft, groups.value, nextGroups, renamed)); return true; }
  catch (cause) { show(cause instanceof Error ? cause.message : "无法保存出站输入", "error"); return false; }
}

function submit(result: OutboundEditorResult): void {
  const wasEditing = editingId.value !== null;
  const renamed = editing.value && editing.value.tag !== result.group.tag
    ? { from: editing.value.tag, to: result.group.tag }
    : undefined;
  if (!persist(updateOutboundGroup(groups.value, result.group), renamed)) return;
  closeEditor();
  show(`${result.group.tag} 已${wasEditing ? "更新" : "添加"}；引用将在服务端生成。`, "success");
}

function remove(group: OutboundEditorGroup): void {
  if (!persist(removeOutboundGroup(groups.value, group.id))) return;
  show(`${group.tag} 已从当前配置移除；其他策略组中的原引用会保留并标记为失效。`, "success");
}

function updateHealth(key: "url" | "interval" | "tolerance", event: Event): void {
  if (!healthGroup.value) return;
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
  const group = cloneValue(healthGroup.value);
  group.config[key] = value;
  if (!persist(updateOutboundGroup(groups.value, group))) return;
  show("健康检查参数已更新。", "success");
}

function countLabel(group: OutboundEditorGroup): string {
  if (!isPolicyOutbound(group.config)) return "内置出站";
  const subscriptions = group.references.filter((reference) => reference.kind === "subscription").length;
  const outbounds = group.references.length - subscriptions;
  return `${outbounds} 个出站 / ${subscriptions} 个订阅`;
}
function hasLost(group: OutboundEditorGroup): boolean {
  return group.references.some((reference) => reference.kind === "missing"
    || (reference.kind === "outbound" && !groups.value.some((candidate) => candidate.id === reference.outboundId))
    || (reference.kind === "subscription" && !subscriptionDetails.value.some((subscription) => subscription.id === reference.subscriptionId))
    || (reference.kind === "subscription-node" && !subscriptionDetails.value.some((subscription) => subscription.id === reference.subscriptionId && subscription.outbounds.some((node) => isObject(node) && node.tag === reference.nodeTag))));
}

function tone(group: OutboundEditorGroup): string {
  if (group.type === "urltest") return "cyan";
  if (group.type === "direct") return "green";
  if (group.type === "block") return "orange";
  return "";
}
</script>

<template>
  <section class="outbound-panel">
    <div class="outbound-layout">
      <article class="outbound-list-card">
        <header><div><h2>出站与策略组</h2><p>决定流量最终使用的出口</p></div><div><button class="json-button" type="button" @click="emit('editJson')"><AppIcon name="code" :size="14" />编辑全部 JSON</button><button class="add-button" type="button" :disabled="preparing" @click="openEditor(null)"><AppIcon name="plus" :size="14" />{{ preparing ? "读取引用中" : "新增出站" }}</button></div></header>
        <div class="outbound-grid">
          <article v-for="group in groups" :key="group.id" class="outbound-card" :data-outbound-tag="group.tag">
            <div class="outbound-head"><span class="entry-icon" :class="tone(group)"><img v-if="group.icon" :src="group.icon" alt="" /><AppIcon v-else :name="outboundIcon(group.config)" :size="18" /></span><div class="outbound-name"><strong>{{ group.tag }}</strong><span>{{ outboundDescription(group.config) }}</span></div><div class="outbound-actions"><button type="button" :aria-label="`编辑出站 ${group.tag}`" title="编辑出站" :disabled="preparing" @click="openEditor(group.id)"><AppIcon name="edit" :size="14" /></button><button class="danger" type="button" :aria-label="`删除出站 ${group.tag}`" title="删除出站" :disabled="preparing" @click="remove(group)"><AppIcon name="trash" :size="14" /></button></div></div>
            <div class="outbound-summary"><div class="entry-meta"><b>{{ group.type }}</b><span>{{ countLabel(group) }}</span><em v-if="group.hidden">隐藏</em><em v-if="hasLost(group) && !serverResolution" class="lost">引用失效</em><em v-else-if="isPolicyOutbound(group.config) && !group.references.length" class="lost">尚未引用</em></div></div>
          </article>
          <button v-if="!groups.length" class="empty-state" type="button" :disabled="preparing" @click="openEditor(null)"><AppIcon name="outbound" :size="31" /><strong>尚未添加出站</strong><span>点击创建第一个出站或策略组</span></button>
        </div>
      </article>

      <aside>
        <article class="side-card"><header><div><h2>默认链路</h2><p>当前出站拓扑</p></div></header><div class="topology"><div v-for="(node, index) in topology" :key="`${node}-${index}`" class="topology-node" :class="{ active: index === 0 }"><span class="topology-dot" />{{ node }}</div></div></article>
        <article class="side-card"><header><div><h2>健康检查</h2><p>策略组延迟检测参数</p></div></header><div class="health-fields">
          <label><span>测试地址</span><input aria-label="测试地址" :value="healthGroup?.config.url ?? 'https://www.gstatic.com/generate_204'" :disabled="!healthGroup" @change="updateHealth('url', $event)" /></label>
          <label><span>测试间隔</span><span class="health-suffix"><input aria-label="测试间隔" :value="healthGroup?.config.interval ?? '3m'" :disabled="!healthGroup" @change="updateHealth('interval', $event)" /></span></label>
          <label><span>容差</span><span class="health-suffix"><input aria-label="容差" type="number" min="0" :value="healthGroup?.config.tolerance ?? 150" :disabled="!healthGroup" @change="updateHealth('tolerance', $event)" /><b>ms</b></span></label>
          <p v-if="!healthGroup"><AppIcon name="info" :size="13" />添加“自动选择”出站后可在此快速调整。</p>
        </div></article>
        <p v-if="subscriptionError" class="subscription-error" role="status"><AppIcon name="info" :size="14" />{{ subscriptionError }}</p>
      </aside>
    </div>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <OutboundEditorDialog :open="editorOpen" :group="editing" :groups="groups" :reserved-tags="reservedTags" :subscriptions="subscriptionDetails" @close="closeEditor" @submit="submit" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.outbound-panel { display: grid; gap: 10px; }.outbound-layout { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(230px, .65fr); gap: var(--section-gap); }.outbound-list-card, .side-card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.outbound-list-card > header, .side-card > header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }.outbound-list-card h2, .side-card h2, .outbound-list-card header p, .side-card header p { margin: 0; }.outbound-list-card h2, .side-card h2 { font-size: var(--font-size-heading); }.outbound-list-card header p, .side-card header p { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }.outbound-list-card header > div:last-child { display: flex; gap: 6px; }.outbound-list-card header button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; font-size: var(--font-size-sm); }.json-button { color: var(--text-soft); border: 1px solid var(--line); background: var(--surface-soft); }.add-button { color: #fff; background: var(--primary); }.outbound-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 280px), 1fr)); gap: 8px; padding: 10px; }.outbound-card { position: relative; min-width: 0; overflow: hidden; padding: 11px; border: 1px solid var(--line); border-radius: 12px; background: var(--surface); transition: border-color 150ms ease, box-shadow 150ms ease, transform 150ms ease; }.outbound-card:hover { border-color: color-mix(in srgb, var(--primary) 25%, var(--line)); box-shadow: 0 8px 22px rgb(34 50 89 / 7%); transform: translateY(-2px); }.outbound-head { display: flex; align-items: flex-start; gap: 11px; }.entry-icon { display: grid; width: 34px; height: 34px; flex: 0 0 auto; overflow: hidden; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.entry-icon img { width: 100%; height: 100%; object-fit: cover; }.entry-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.entry-icon.orange { color: var(--orange); background: var(--orange-soft); }.entry-icon.green { color: var(--green); background: var(--green-soft); }.outbound-name { min-width: 0; flex: 1; }.outbound-name strong, .outbound-name span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.outbound-name strong { color: var(--text); font-size: var(--font-size-base); }.outbound-name span { margin-top: 2px; color: var(--text-faint); font-size: var(--font-size-xs); }.outbound-summary { min-width: 0; margin-top: 11px; }.entry-meta { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 5px; color: var(--text-faint); font-size: var(--font-size-sm); }.entry-meta b { padding: 2px 6px; border-radius: 5px; color: var(--primary); background: var(--primary-soft); font: 700 var(--font-size-sm) ui-monospace, monospace; }.entry-meta em { padding: 2px 6px; border-radius: 999px; color: var(--orange); background: var(--orange-soft); font-style: normal; font-weight: 680; }.entry-meta em.lost { color: var(--red); background: var(--red-soft); }.empty-state { display: grid; min-height: var(--empty-state-height); grid-column: 1 / -1; place-items: center; align-content: center; gap: 7px; color: var(--primary); border: 1px dashed var(--line-strong); background: var(--surface-soft); }.empty-state strong { color: var(--text); font-size: var(--font-size-base); }.empty-state span { color: var(--text-faint); font-size: var(--font-size-xs); font-weight: 400; }.outbound-layout aside { display: grid; align-content: start; gap: var(--section-gap); }.topology { padding: 6px 4px 8px; }.topology-node { display: flex; align-items: center; gap: 9px; padding: 8px; border-radius: 9px; color: var(--text-soft); font-size: var(--font-size-xs); }.topology-node + .topology-node { position: relative; margin-top: 7px; }.topology-node + .topology-node::before { position: absolute; top: -10px; left: 21px; width: 1px; height: 12px; content: ""; background: var(--line-strong); }.topology-dot { width: 9px; height: 9px; border: 2px solid var(--surface); border-radius: 50%; background: var(--primary); box-shadow: 0 0 0 2px var(--primary-soft); }.topology-node.active { color: var(--text); background: var(--surface-soft); font-weight: 650; }.health-fields { display: grid; gap: 9px; padding: 12px; }.health-fields label { display: grid; gap: 5px; color: var(--text-soft); font-size: var(--font-size-base); font-weight: 650; }.health-fields input { width: 100%; min-height: 36px; padding: 7px 9px; font-size: var(--font-size-base); }.health-fields input:disabled { opacity: .62; }.health-suffix { position: relative; }.health-suffix input { padding-right: 36px; }.health-suffix b { position: absolute; top: 50%; right: 9px; color: var(--text-faint); font-size: var(--font-size-sm); transform: translateY(-50%); }.health-fields > p, .subscription-error { display: flex; align-items: flex-start; gap: 6px; margin: 0; color: var(--text-faint); font-size: var(--font-size-sm); }.subscription-error { padding: 10px; border: 1px solid var(--line); border-radius: 10px; color: var(--orange); background: var(--surface); }.panel-error { margin: 0; color: var(--red); font-size: var(--font-size-sm); }.outbound-actions { display: flex; gap: 2px; }.outbound-actions button { display: grid; width: 30px; min-height: 30px; place-items: center; padding: 0; color: var(--text-faint); background: transparent; }.outbound-actions button:hover { color: var(--primary); background: var(--primary-soft); }.outbound-actions button.danger:hover { color: var(--red); background: var(--red-soft); }
@media (max-width: 900px) { .outbound-layout { grid-template-columns: 1fr; }.outbound-layout aside { grid-template-columns: 1fr 1fr; }.subscription-error { grid-column: 1 / -1; } }
@media (max-width: 680px) { .outbound-list-card > header { flex-direction: column; }.outbound-list-card header > div:last-child { width: 100%; }.outbound-list-card header button { flex: 1; justify-content: center; }.outbound-grid { grid-template-columns: 1fr; }.outbound-layout aside { grid-template-columns: 1fr; }.subscription-error { grid-column: auto; }.outbound-card { padding: 10px; } }
</style>
