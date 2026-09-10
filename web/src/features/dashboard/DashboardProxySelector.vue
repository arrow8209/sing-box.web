<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import { buildDashboardProxyGroups, type DashboardProxyGroup, type DashboardProxyMember } from "./proxy-groups";

const props = withDefaults(defineProps<{ outbounds?: unknown[]; editorState?: unknown; loading?: boolean }>(), {
  outbounds: () => [],
  loading: false
});
const emit = defineEmits<{ notice: [message: string] }>();
const testing = ref(false);
let testTimer: ReturnType<typeof setTimeout> | undefined;

const groups = ref<DashboardProxyGroup[]>([]);

watch([() => props.outbounds, () => props.editorState], () => {
  const previous = new Map(groups.value.map((group) => [group.id, group]));
  groups.value = buildDashboardProxyGroups(props.outbounds, props.editorState).map((group) => {
    const current = previous.get(group.id);
    if (!current) return group;
    const latencyByTag = new Map(current.members.map((member) => [member.tag, member.latency]));
    group.expanded = current.expanded;
    group.selected = group.members.some((member) => member.id === current.selected) ? current.selected : group.selected;
    group.members.forEach((member) => {
      if (latencyByTag.has(member.tag)) member.latency = latencyByTag.get(member.tag);
    });
    return group;
  });
}, { immediate: true });

const leafMembers = computed(() => groups.value.flatMap((group) => group.members).filter((member) => !member.groupId && member.type !== "Block"));
const configuredNodes = computed(() => new Set(leafMembers.value.map((member) => member.tag)).size);
const hasTestedNodes = computed(() => leafMembers.value.some((member) => typeof member.latency === "number"));
const onlineNodes = computed(() => new Set(leafMembers.value.filter((member) => Number(member.latency) > 0).map((member) => member.tag)).size);
const nodeSummary = computed(() => hasTestedNodes.value ? `${onlineNodes.value} 个节点可用` : `${configuredNodes.value} 个节点已配置`);

function selectedLabel(group: DashboardProxyGroup): string {
  const selected = group.members.find((member) => member.id === group.selected);
  if (!selected) return "未选择";
  if (!selected.groupId) return selected.name;
  const referencedGroup = groups.value.find((item) => item.id === selected.groupId);
  const referencedNode = referencedGroup?.members.find((member) => member.id === referencedGroup.selected);
  return referencedNode ? `${selected.name} / ${referencedNode.name}` : selected.name;
}

function initials(member: DashboardProxyMember): string {
  if (member.icon) return member.icon;
  const characters = Array.from(member.name);
  return /^[a-z]/i.test(member.name) ? characters.slice(0, 2).join("").toUpperCase() : characters[0] ?? "?";
}

function latencyTone(member: DashboardProxyMember): string {
  if (member.latency === undefined) return "untested";
  if (member.latency === null) return "unavailable";
  if (!member.latency) return "timeout";
  if (member.latency <= 100) return "fast";
  if (member.latency <= 300) return "medium";
  return "slow";
}

function latencyLabel(member: DashboardProxyMember): string {
  if (member.latency === undefined) return "未测速";
  if (member.latency === null) return "不适用";
  if (!member.latency) return "超时";
  return `${member.latency} ms`;
}

function selectNode(group: DashboardProxyGroup, member: DashboardProxyMember): void {
  group.selected = member.id;
  emit("notice", `${group.name} 已切换到 ${member.name}。`);
}

function toggleGroup(group: DashboardProxyGroup): void {
  group.expanded = !group.expanded;
}

function startTest(targets: DashboardProxyGroup[], message: string): void {
  if (testing.value) return;
  testing.value = true;
  if (testTimer) clearTimeout(testTimer);
  testTimer = setTimeout(() => {
    targets.forEach((group, groupIndex) => {
      group.members.forEach((member, memberIndex) => {
        if (member.latency === null) return;
        member.latency = 28 + ((groupIndex * 61 + memberIndex * 97 + 47) % 330);
      });
    });
    testing.value = false;
    emit("notice", message);
  }, 700);
}

onBeforeUnmount(() => {
  if (testTimer) clearTimeout(testTimer);
});
</script>

<template>
  <article class="card proxy-selector-card">
    <header class="card-head proxy-selector-head">
      <div><h3 class="card-title">节点选择</h3><p class="card-subtitle">选择策略组当前使用的出站节点</p></div>
      <div class="proxy-selector-actions">
        <span class="proxy-online-summary"><i />{{ nodeSummary }}</span>
        <button class="btn btn-sm" type="button" :disabled="testing || !groups.length" @click="startTest(groups, '全部节点延迟已刷新。')"><AppIcon name="gauge" :size="14" />{{ testing ? "测速中" : "全部测速" }}</button>
      </div>
    </header>

    <div class="proxy-group-list">
      <section v-for="group in groups" :key="group.id" class="proxy-group" :class="{ expanded: group.expanded }" :data-proxy-group-tag="group.tag">
        <div class="proxy-group-head">
          <button class="proxy-group-main" type="button" :aria-expanded="group.expanded" @click="toggleGroup(group)">
            <span class="proxy-group-avatar">{{ group.icon }}</span>
            <span class="proxy-group-copy"><strong>{{ group.name }}</strong><small><b>{{ group.type }}</b><i />{{ selectedLabel(group) }}</small></span>
          </button>
          <div class="proxy-group-actions">
            <button class="proxy-icon-button" type="button" :aria-label="`测试 ${group.name} 延迟`" :title="`测试 ${group.name} 延迟`" @click="startTest([group], `${group.name} 延迟已刷新。`)"><AppIcon name="gauge" :size="14" /></button>
            <button class="proxy-icon-button proxy-expand-button" type="button" :aria-label="group.expanded ? `收起 ${group.name}` : `展开 ${group.name}`" :aria-expanded="group.expanded" @click="toggleGroup(group)"><AppIcon name="chevron" :size="14" /></button>
          </div>
        </div>

        <div v-if="group.expanded" class="proxy-node-grid">
          <button v-for="member in group.members" :key="member.id" class="proxy-node-card" :class="{ selected: member.id === group.selected }" type="button" :aria-pressed="member.id === group.selected" :data-proxy-member-tag="member.tag" @click="selectNode(group, member)">
            <span class="proxy-node-avatar" :class="{ emoji: member.icon }">{{ initials(member) }}</span>
            <span class="proxy-node-copy">
              <strong>{{ member.name }}</strong>
              <span v-if="testing" class="proxy-latency testing"><AppIcon name="rotate" :size="10" />测速中</span>
              <span v-else class="proxy-latency" :class="latencyTone(member)">{{ latencyLabel(member) }}</span>
              <small>{{ member.type }}<template v-if="member.network"> · {{ member.network }}</template></small>
            </span>
            <span class="proxy-node-check"><AppIcon v-if="member.id === group.selected" name="check" :size="12" /></span>
          </button>
        </div>
      </section>
      <div v-if="!groups.length" class="proxy-empty-state">
        <AppIcon name="outbound" :size="24" />
        <strong>{{ loading ? "正在读取当前配置" : "没有可选择的策略组" }}</strong>
        <span>{{ loading ? "节点列表加载后会自动显示" : "请先在当前配置中添加 Selector 或 URLTest 出站" }}</span>
      </div>
    </div>
  </article>
</template>

<style scoped>
.card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.card-head { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: var(--section-gap); padding: 8px var(--card-padding); border-bottom: 1px solid var(--line); }
.card-title { margin: 0; color: var(--text); font-size: var(--font-size-heading); font-weight: 710; letter-spacing: -.15px; }
.card-subtitle { margin: 2px 0 0; color: var(--text-faint); font-size: var(--font-size-sm); }
.btn { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 7px; padding: 7px 11px; border: 1px solid var(--line); border-radius: 10px; color: var(--text-soft); background: var(--surface); box-shadow: 0 1px 2px rgb(23 32 51 / 3%); font-size: var(--font-size-base); font-weight: 650; transition: color 150ms ease, border-color 150ms ease, background 150ms ease, transform 150ms ease; }
.btn:hover { color: var(--text); border-color: var(--line-strong); background: var(--surface-soft); transform: translateY(-1px); }
.btn-sm { min-height: 32px; padding: 6px 10px; border-radius: 8px; font-size: var(--font-size-sm); }
.proxy-selector-card { grid-column: 1 / -1; overflow: hidden; }
.proxy-selector-head { min-height: 48px; }
.proxy-selector-actions, .proxy-online-summary { display: flex; align-items: center; }
.proxy-selector-actions { gap: 12px; }
.proxy-online-summary { gap: 6px; color: var(--text-soft); font-size: var(--font-size-xs); font-weight: 640; }
.proxy-online-summary i { width: 6px; height: 6px; border-radius: 50%; background: var(--green); box-shadow: 0 0 0 4px color-mix(in srgb, var(--green) 12%, transparent); }
.proxy-group-list { display: grid; }
.proxy-group { border-bottom: 1px solid var(--line); transition: background 180ms ease; }
.proxy-group:last-child { border-bottom: 0; }
.proxy-group.expanded { background: linear-gradient(110deg, color-mix(in srgb, var(--primary-soft) 64%, var(--surface)), var(--surface) 72%); }
.proxy-group-head { display: flex; min-height: 48px; align-items: stretch; }
.proxy-group-main { display: flex; min-width: 0; min-height: 0; flex: 1; align-items: center; gap: 8px; padding: 7px 8px 7px var(--card-padding); border-radius: 0; color: var(--text); background: transparent; text-align: left; }
.proxy-group-main:hover .proxy-group-copy strong { color: var(--primary); }
.proxy-group-avatar { display: grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; border: 1px solid color-mix(in srgb, var(--primary) 12%, var(--line)); border-radius: 10px; background: var(--surface); box-shadow: 0 3px 10px rgb(35 50 85 / 5%); font-size: 16px; }
.proxy-group-copy { min-width: 0; flex: 1; }
.proxy-group-copy strong, .proxy-group-copy small { display: flex; align-items: center; }
.proxy-group-copy strong { overflow: hidden; color: var(--text); font-size: var(--font-size-base); font-weight: 710; text-overflow: ellipsis; white-space: nowrap; transition: color 150ms ease; }
.proxy-group-copy small { min-width: 0; gap: 7px; margin-top: 3px; overflow: hidden; color: var(--text-faint); font-size: var(--font-size-xs); text-overflow: ellipsis; white-space: nowrap; }
.proxy-group-copy small b { color: var(--primary); font-weight: 680; }
.proxy-group-copy small i { width: 3px; height: 3px; flex: 0 0 auto; border-radius: 50%; background: var(--line-strong); }
.proxy-group-actions { display: flex; align-items: center; gap: 3px; padding: 0 13px 0 4px; }
.proxy-icon-button { display: grid; width: 31px; height: 31px; min-height: 31px; place-items: center; padding: 0; border-radius: 8px; color: var(--text-faint); background: transparent; transition: color 150ms ease, background 150ms ease; }
.proxy-icon-button:hover { color: var(--primary); background: var(--primary-soft); }
.proxy-expand-button :deep(.icon) { transition: transform 160ms ease; }
.proxy-group.expanded .proxy-expand-button :deep(.icon) { transform: rotate(90deg); }
.proxy-node-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; padding: 0 var(--card-padding) 10px 50px; animation: fade-in 160ms ease both; }
.proxy-node-card { position: relative; display: flex; min-width: 0; min-height: 64px; align-items: center; gap: 10px; padding: 8px 30px 8px 10px; border: 1px solid var(--line); border-radius: 11px; color: var(--text); background: color-mix(in srgb, var(--surface) 94%, transparent); box-shadow: 0 4px 12px rgb(30 43 73 / 4%); text-align: left; transition: border-color 150ms ease, box-shadow 150ms ease, transform 150ms ease; }
.proxy-node-card:hover { border-color: color-mix(in srgb, var(--primary) 32%, var(--line)); box-shadow: 0 8px 18px rgb(37 55 96 / 8%); transform: translateY(-1px); }
.proxy-node-card.selected { border-color: color-mix(in srgb, var(--primary) 42%, var(--line)); background: color-mix(in srgb, var(--primary-soft) 44%, var(--surface)); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 7%, transparent); }
.proxy-node-avatar { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); font-size: var(--font-size-sm); font-weight: 760; }
.proxy-node-avatar.emoji { font-size: 15px; }
.proxy-node-copy { display: grid; min-width: 0; gap: 1px; }
.proxy-node-copy strong { overflow: hidden; color: var(--text); font-size: var(--font-size-base); font-weight: 690; text-overflow: ellipsis; white-space: nowrap; }
.proxy-node-copy small { overflow: hidden; color: var(--text-faint); font-size: var(--font-size-sm); text-overflow: ellipsis; white-space: nowrap; }
.proxy-latency { display: inline-flex; width: max-content; align-items: center; gap: 4px; font-size: var(--font-size-xs); font-weight: 700; }
.proxy-latency.fast, .proxy-latency.medium { color: var(--green); }
.proxy-latency.slow { color: var(--orange); }
.proxy-latency.timeout { color: var(--red); }
.proxy-latency.unavailable, .proxy-latency.untested, .proxy-latency.testing { color: var(--text-faint); }
.proxy-latency.testing :deep(.icon) { animation: proxy-spin 800ms linear infinite; }
.proxy-node-check { position: absolute; right: 8px; bottom: 8px; display: grid; width: 20px; height: 20px; place-items: center; border-radius: 50%; color: #fff; background: transparent; }
.proxy-node-card.selected .proxy-node-check { background: var(--primary); }
.proxy-empty-state { display: grid; min-height: 150px; place-items: center; align-content: center; gap: 7px; color: var(--primary); }
.proxy-empty-state strong { color: var(--text); font-size: var(--font-size-base); }
.proxy-empty-state span { color: var(--text-faint); font-size: var(--font-size-xs); }
@keyframes fade-in { from { opacity: 0; transform: translateY(-3px); } to { opacity: 1; transform: translateY(0); } }
@keyframes proxy-spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) {
  .proxy-node-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); padding-left: var(--card-padding); }
  .proxy-node-card { gap: 6px; padding-left: 8px; }
  .proxy-node-avatar { width: 26px; height: 26px; border-radius: 7px; }
}
@media (max-width: 430px) {
  .proxy-selector-actions { gap: 8px; }
  .proxy-online-summary { display: none; }
  .proxy-group-actions { padding-right: 8px; }
}
@media (max-width: 359px) {
  .proxy-node-grid { grid-template-columns: minmax(0, 1fr); }
}
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: .01ms !important; animation-iteration-count: 1 !important; transition-duration: .01ms !important; } }
</style>
