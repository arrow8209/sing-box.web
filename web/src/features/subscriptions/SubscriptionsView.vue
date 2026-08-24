<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import AppIcon from "@/layout/AppIcon.vue";
import { subscriptionApi } from "./api";
import SubscriptionDeleteDialog from "./SubscriptionDeleteDialog.vue";
import SubscriptionJsonDialog from "./SubscriptionJsonDialog.vue";
import SubscriptionNodesDialog from "./SubscriptionNodesDialog.vue";
import type { SubscriptionDetail, SubscriptionInput, SubscriptionSummary } from "./types";

const subscriptions = ref<SubscriptionSummary[]>([]);
const loading = ref(true);
const loadError = ref("");
const saving = ref(false);
const deleting = ref(false);
const openingId = ref("");
const editing = ref<SubscriptionDetail | null>(null);
const viewing = ref<SubscriptionDetail | null>(null);
const deleteTarget = ref<SubscriptionDetail | null>(null);
const editorOpen = ref(false);
const nodesOpen = ref(false);
const deleteOpen = ref(false);
const notice = ref("");
const noticeTone = ref<"info" | "error" | "success">("info");

const totalNodes = computed(() => subscriptions.value.reduce((total, item) => total + item.nodeCount, 0));
const totalProtocols = computed(() => new Set(subscriptions.value.flatMap((item) => item.protocols)).size);
const suggestedName = computed(() => `JSON 订阅 ${subscriptions.value.length + 1}`);

onMounted(refresh);

async function refresh(): Promise<void> {
  loading.value = true;
  try {
    subscriptions.value = await subscriptionApi.list();
    loadError.value = "";
  } catch (cause) {
    loadError.value = cause instanceof Error ? cause.message : "无法加载订阅";
  } finally {
    loading.value = false;
  }
}

function show(message: string, tone: "info" | "error" | "success" = "info"): void {
  notice.value = "";
  noticeTone.value = tone;
  queueMicrotask(() => { notice.value = message; });
}

function openCreate(): void {
  editing.value = null;
  editorOpen.value = true;
}

function closeEditor(): void {
  editorOpen.value = false;
  editing.value = null;
}

function closeNodes(): void {
  nodesOpen.value = false;
  viewing.value = null;
}

function closeDelete(): void {
  deleteOpen.value = false;
  deleteTarget.value = null;
}

async function loadDetail(item: SubscriptionSummary): Promise<SubscriptionDetail | null> {
  openingId.value = item.id;
  try {
    return await subscriptionApi.get(item.id);
  } catch (cause) {
    show(cause instanceof Error ? cause.message : "无法读取订阅节点", "error");
    return null;
  } finally {
    openingId.value = "";
  }
}

async function openEdit(item: SubscriptionSummary): Promise<void> {
  const detail = await loadDetail(item);
  if (!detail) return;
  editing.value = detail;
  editorOpen.value = true;
}

async function openNodes(item: SubscriptionSummary): Promise<void> {
  const detail = await loadDetail(item);
  if (!detail) return;
  viewing.value = detail;
  nodesOpen.value = true;
}

async function submit(input: SubscriptionInput): Promise<void> {
  saving.value = true;
  try {
    const summary = editing.value
      ? await subscriptionApi.replace(editing.value.id, input)
      : await subscriptionApi.create(input);
    const index = subscriptions.value.findIndex((item) => item.id === summary.id);
    subscriptions.value = index < 0
      ? [...subscriptions.value, summary]
      : subscriptions.value.map((item) => item.id === summary.id ? summary : item);
    editorOpen.value = false;
    editing.value = null;
    show(`${summary.name} 的 ${summary.nodeCount} 个节点已保存。`, "success");
  } catch (cause) {
    show(cause instanceof Error ? cause.message : "保存订阅失败", "error");
  } finally {
    saving.value = false;
  }
}

function requestDelete(): void {
  if (!editing.value) return;
  deleteTarget.value = editing.value;
  editorOpen.value = false;
  editing.value = null;
  deleteOpen.value = true;
}

async function confirmDelete(): Promise<void> {
  if (!deleteTarget.value) return;
  deleting.value = true;
  try {
    await subscriptionApi.remove(deleteTarget.value.id);
    subscriptions.value = subscriptions.value.filter((item) => item.id !== deleteTarget.value?.id);
    const name = deleteTarget.value.name;
    deleteOpen.value = false;
    deleteTarget.value = null;
    editing.value = null;
    show(`${name} 已删除。`, "success");
  } catch (cause) {
    show(cause instanceof Error ? cause.message : "删除订阅失败", "error");
  } finally {
    deleting.value = false;
  }
}

function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length > 1) return words.slice(0, 2).map((word) => Array.from(word)[0]).join("").toUpperCase();
  return Array.from(name.trim()).slice(0, 2).join("").toUpperCase() || "JS";
}

function updatedLabel(value: string): string {
  const stamp = new Date(value).getTime();
  if (!Number.isFinite(stamp)) return "未知";
  const seconds = Math.max(0, Math.floor((Date.now() - stamp) / 1000));
  if (seconds < 60) return "刚刚";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`;
  if (seconds < 604800) return `${Math.floor(seconds / 86400)} 天前`;
  return new Date(value).toLocaleDateString("zh-CN");
}
</script>

<template>
  <section class="subscriptions-page">
    <header class="page-intro"><div><h1>订阅配置</h1><p>仅支持通过 JSON 编辑器批量导入并维护 sing-box 出站节点。</p></div><button class="primary-action" type="button" @click="openCreate"><AppIcon name="code" :size="15" />导入 JSON 订阅</button></header>
    <p v-if="loadError" class="page-error" role="alert">{{ loadError }} <button type="button" @click="refresh">重试</button></p>

    <div class="summary-grid">
      <article class="summary-card"><span class="summary-icon"><AppIcon name="subscription" /></span><div><strong>{{ subscriptions.length }}</strong><span>JSON 订阅 · 本地编辑与维护</span></div></article>
      <article class="summary-card"><span class="summary-icon cyan"><AppIcon name="globe" /></span><div><strong>{{ totalNodes }}</strong><span>已导入节点 · 完整保留 outbounds</span></div></article>
      <article class="summary-card"><span class="summary-icon orange"><AppIcon name="code" /></span><div><strong>{{ totalProtocols }}</strong><span>协议类型 · 由节点 JSON 自动识别</span></div></article>
    </div>

    <div class="subscription-grid" :aria-busy="loading">
      <article v-for="(item, index) in subscriptions" :key="item.id" class="subscription-card" :class="{ opening: openingId === item.id }" tabindex="0" :aria-label="`双击查看 ${item.name} 的全部 ${item.nodeCount} 个节点`" @dblclick="openNodes(item)" @keydown.enter.self.prevent="openNodes(item)" @keydown.space.self.prevent="openNodes(item)">
        <div class="subscription-main">
          <div class="subscription-top"><span class="provider-logo" :class="{ cyan: index % 2 === 1 }">{{ initials(item.name) }}</span><div class="subscription-name"><strong>{{ item.name }}</strong><span>JSON 节点集合 · {{ item.nodeCount }} 个出站</span></div><span class="json-badge">JSON</span></div>
          <div class="json-source"><span class="source-icon"><AppIcon name="code" :size="15" /></span><div><strong>完整节点已导入</strong><span>编辑 JSON 会整体更新该订阅中的全部节点</span></div></div>
          <div class="subscription-meta"><div><span>节点数量</span><strong>{{ item.nodeCount }} 个</strong></div><div><span>协议类型</span><strong>{{ item.protocols.length }} 种</strong></div><div><span>最后编辑</span><strong>{{ updatedLabel(item.updatedAt) }}</strong></div></div>
          <div class="protocols"><span v-for="protocol in item.protocols.slice(0, 4)" :key="protocol">{{ protocol }}</span><span v-if="item.protocols.length > 4">+{{ item.protocols.length - 4 }}</span></div>
        </div>
        <footer class="subscription-footer" @dblclick.stop><span><AppIcon name="info" :size="12" />双击卡片查看全部节点</span><div><button type="button" :disabled="openingId === item.id" @click.stop="openNodes(item)"><AppIcon name="globe" :size="14" />查看节点</button><button type="button" :disabled="openingId === item.id" @click.stop="openEdit(item)"><AppIcon name="edit" :size="14" />编辑 JSON</button></div></footer>
      </article>

      <button class="add-card" type="button" @click="openCreate"><span class="empty-icon"><AppIcon name="code" :size="24" /></span><strong>{{ loading ? "正在读取订阅" : "添加 JSON 订阅" }}</strong><span>{{ loading ? "请稍候" : "粘贴完整配置或节点数组，一次导入全部节点" }}</span><b>打开 JSON 编辑器</b></button>
    </div>

    <SubscriptionJsonDialog :open="editorOpen" :subscription="editing" :suggested-name="suggestedName" :busy="saving" @close="closeEditor" @submit="submit" @delete="requestDelete" />
    <SubscriptionNodesDialog :open="nodesOpen" :subscription="viewing" @close="closeNodes" @copied="(message) => show(message, 'success')" @error="(message) => show(message, 'error')" />
    <SubscriptionDeleteDialog :open="deleteOpen" :name="deleteTarget?.name ?? ''" :busy="deleting" @close="closeDelete" @confirm="confirmDelete" />
    <AppToast v-if="notice" :message="notice" :tone="noticeTone" />
  </section>
</template>

<style scoped>
.subscriptions-page { display: grid; gap: 14px; }.page-intro { display: flex; align-items: end; justify-content: space-between; gap: 18px; }.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.45px; }.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }.primary-action { display: inline-flex; min-height: 35px; align-items: center; gap: 6px; padding: 7px 11px; font-size: 10px; }.page-error { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 0; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: 10px; }.page-error button { min-height: 28px; padding: 4px 8px; color: var(--red); background: var(--surface); font-size: 9px; }.summary-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }.summary-card { display: flex; min-height: 89px; align-items: center; gap: 13px; padding: 14px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.summary-icon { display: grid; width: 40px; height: 40px; flex: 0 0 auto; place-items: center; border-radius: 11px; color: var(--primary); background: var(--primary-soft); }.summary-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.summary-icon.orange { color: var(--orange); background: var(--orange-soft); }.summary-card strong, .summary-card div span { display: block; }.summary-card strong { color: var(--text); font-size: 19px; }.summary-card div span { margin-top: 2px; color: var(--text-soft); font-size: 9px; }.subscription-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 15px; }.subscription-card { overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); cursor: pointer; transition: transform 160ms ease, border-color 160ms ease, box-shadow 160ms ease; }.subscription-card:hover, .subscription-card:focus-visible { border-color: color-mix(in srgb, var(--primary) 34%, var(--line)); box-shadow: 0 13px 28px rgb(26 39 75 / 9%); transform: translateY(-1px); }.subscription-card:focus-visible { outline: 2px solid color-mix(in srgb, var(--primary) 28%, transparent); outline-offset: 2px; }.subscription-card.opening { opacity: .72; }.subscription-main { padding: 17px; }.subscription-top { display: flex; align-items: flex-start; gap: 12px; }.provider-logo { display: grid; width: 43px; height: 43px; flex: 0 0 auto; place-items: center; border-radius: 13px; color: #fff; background: linear-gradient(145deg, #657ffc, #3e5bd6); box-shadow: 0 8px 17px rgb(79 111 248 / 17%); font-size: 12px; font-weight: 760; }.provider-logo.cyan { background: linear-gradient(145deg, #24c7d6, #1199aa); box-shadow: 0 8px 17px rgb(19 185 203 / 17%); }.subscription-name { min-width: 0; flex: 1; }.subscription-name strong, .subscription-name span { display: block; }.subscription-name strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }.subscription-name span { margin-top: 3px; color: var(--text-faint); font-size: 9px; }.json-badge { display: inline-flex; min-height: 22px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--green); background: var(--green-soft); font-size: 10px; font-weight: 660; }.json-badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }.json-source { display: flex; align-items: center; gap: 10px; margin-top: 16px; padding: 11px 12px; border: 1px solid color-mix(in srgb, var(--primary) 12%, var(--line)); border-radius: 10px; background: color-mix(in srgb, var(--primary-soft) 50%, var(--surface)); }.source-icon { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; border-radius: 8px; color: var(--primary); background: var(--surface); }.json-source strong, .json-source span { display: block; }.json-source strong { font-size: 10px; }.json-source div span { margin-top: 2px; color: var(--text-faint); font-size: 8px; }.subscription-meta { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-top: 15px; }.subscription-meta div { min-width: 0; padding: 8px; border-radius: 9px; background: var(--surface-soft); }.subscription-meta span, .subscription-meta strong { display: block; }.subscription-meta span { color: var(--text-faint); font-size: 8px; }.subscription-meta strong { overflow: hidden; margin-top: 2px; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }.protocols { display: flex; min-height: 22px; align-items: center; gap: 5px; margin-top: 11px; overflow: hidden; }.protocols span { display: inline-flex; min-height: 21px; align-items: center; padding: 3px 7px; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font: 700 9px ui-monospace, SFMono-Regular, Consolas, monospace; white-space: nowrap; }.subscription-footer { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 11px 14px; border-top: 1px solid var(--line); background: var(--surface-soft); }.subscription-footer > span { display: inline-flex; align-items: center; gap: 5px; color: var(--text-faint); font-size: 9px; }.subscription-footer > div { display: flex; gap: 6px; }.subscription-footer button { display: inline-flex; min-height: 31px; align-items: center; gap: 5px; padding: 5px 8px; border: 1px solid transparent; color: var(--text-soft); background: transparent; font-size: 9px; }.subscription-footer button:last-child { border-color: var(--line); background: var(--surface); }.subscription-footer button:hover { color: var(--primary); background: var(--primary-soft); }.add-card { display: grid; min-height: 306px; place-items: center; align-content: center; gap: 5px; padding: 30px; border: 1px solid var(--line); border-radius: var(--radius); color: var(--text); background: var(--surface); box-shadow: var(--shadow-sm); text-align: center; }.add-card:hover { border-color: color-mix(in srgb, var(--primary) 30%, var(--line)); }.empty-icon { display: grid; width: 52px; height: 52px; margin-bottom: 6px; place-items: center; border-radius: 15px; color: var(--primary); background: var(--primary-soft); }.add-card strong { font-size: 13px; }.add-card > span:not(.empty-icon) { color: var(--text-faint); font-size: 10px; font-weight: 400; }.add-card b { margin-top: 7px; padding: 7px 10px; border-radius: 8px; color: #fff; background: var(--primary); font-size: 9px; }
@media (max-width: 900px) { .subscription-grid { grid-template-columns: 1fr; } }
@media (max-width: 680px) { .page-intro { align-items: flex-start; flex-direction: column; }.primary-action { width: 100%; justify-content: center; }.summary-grid { grid-template-columns: 1fr; gap: 9px; }.summary-card { min-height: 78px; }.subscription-main { padding: 14px; }.subscription-footer { align-items: stretch; flex-direction: column; }.subscription-footer > div { width: 100%; }.subscription-footer button { flex: 1; justify-content: center; }.add-card { min-height: 220px; } }
@media (max-width: 430px) { .subscription-meta { grid-template-columns: 1fr 1fr; }.subscription-meta div:last-child { grid-column: 1 / -1; }.json-source { align-items: flex-start; } }
</style>
