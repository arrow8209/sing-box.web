<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { profileApi } from "@/features/profiles/api";
import { useProfileStore } from "@/features/profiles/store";
import type { ProfileSummary } from "@/features/profiles/types";
import { ruleResourceApi } from "@/features/rule-resources/api";
import type { RuleResource } from "@/features/rule-resources/types";
import { useCoreStore } from "@/features/core/store";
import AppIcon from "@/layout/AppIcon.vue";

const core = useCoreStore();
const profileStore = useProfileStore();
const profiles = ref<ProfileSummary[]>([]);
const rules = ref<RuleResource[]>([]);
const error = ref("");
const running = computed(() => core.status.value?.state === "running");
const activeProfile = computed(() => profiles.value.find((item) => item.id === core.status.value?.currentProfileId) ?? profiles.value.find((item) => item.id === profileStore.activeProfileId.value));
const needsApply = computed(() => profiles.value.filter((item) => item.needsApply).length);
const enabledRules = computed(() => rules.value.filter((item) => !item.disabled).length);
const recentRule = computed(() => [...rules.value].filter((item) => item.lastSuccessAt).sort((a, b) => String(b.lastSuccessAt).localeCompare(String(a.lastSuccessAt)))[0]);
const coreStateLabels: Record<string, string> = { running: "运行中", starting: "启动中", stopping: "停止中", stopped: "已停止", not_installed: "尚未安装", start_failed: "启动失败", exited: "已退出" };
const draftStats = computed(() => {
  const config = profileStore.draft.value?.config ?? {};
  const entries = (key: string): number => Array.isArray(config[key]) ? config[key].length : 0;
  const route = config.route as Record<string, unknown> | undefined;
  return { inbounds: entries("inbounds"), outbounds: entries("outbounds"), routeRules: Array.isArray(route?.rules) ? route.rules.length : 0 };
});
const coreState = computed(() => coreStateLabels[core.status.value?.state ?? ""] ?? "正在读取");

async function refreshSummaries(): Promise<void> {
  try {
    [profiles.value, rules.value] = await Promise.all([profileApi.list(), ruleResourceApi.list()]);
    error.value = "";
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "无法读取仪表盘摘要";
  }
}

async function refresh(): Promise<void> {
  await Promise.all([core.refresh(), refreshSummaries()]);
}

onMounted(() => { core.startPolling(); void refreshSummaries(); });
onBeforeUnmount(core.stopPolling);
</script>

<template>
  <section class="dashboard-page">
    <header class="page-intro">
      <div class="page-intro-copy"><h1>仪表盘</h1><p>核心、档案和规则资源均来自服务端当前状态。</p></div>
      <div class="page-actions"><button class="btn" type="button" @click="refresh"><AppIcon name="rotate" :size="16" />刷新状态</button><a class="btn btn-primary" href="/workspace"><AppIcon name="config" :size="16" />管理配置</a></div>
    </header>

    <p v-if="error || core.error.value" class="error" role="alert">{{ error || core.error.value }}</p>

    <div class="dashboard-grid">
      <article class="card hero-status" :class="{ stopped: !running }">
        <div class="hero-content">
          <div class="hero-eyebrow"><span class="pulse-dot" :class="{ stopped: !running }" />系统实时状态</div>
          <div class="hero-main"><h2>sing-box {{ running ? "正在运行" : coreState }}</h2><p>{{ running ? "核心进程与当前应用配置均由服务端持续监控。" : "当前不会转发任何流量；可在核心管理中检查安装和更新状态。" }}</p></div>
          <div class="hero-footer">
            <div class="hero-meta"><div class="hero-meta-item"><span>核心版本</span><strong>{{ core.status.value?.installedVersion || "未安装" }}</strong></div><div class="hero-meta-item"><span>进程</span><strong>{{ core.status.value?.pid ? `PID ${core.status.value.pid}` : "—" }}</strong></div><div class="hero-meta-item"><span>活动配置</span><strong>{{ activeProfile?.name || "未应用" }}</strong></div></div>
            <div class="hero-actions"><a class="hero-btn" href="/core"><AppIcon name="cpu" :size="14" />核心管理</a><a class="hero-btn" :class="{ 'hero-btn-danger': !running }" href="/workspace"><AppIcon name="config" :size="14" />{{ needsApply ? "应用配置" : "查看配置" }}</a></div>
          </div>
        </div>
      </article>

      <article class="card control-card">
        <header class="card-head"><div><h2 class="card-title">快速概览</h2><p class="card-subtitle">一期仅展示可由服务端确认的状态</p></div><span class="badge" :class="running ? 'badge-success' : 'badge-orange'">{{ coreState }}</span></header>
        <div class="control-list"><a class="control-row" href="/profiles"><span class="control-icon"><AppIcon name="config" :size="16" /></span><span class="control-copy"><strong>配置档案</strong><small>{{ profiles.length }} 个档案，{{ needsApply }} 个需要应用</small></span><AppIcon name="chevron" :size="16" /></a><a class="control-row" href="/rules"><span class="control-icon cyan"><AppIcon name="rules" :size="16" /></span><span class="control-copy"><strong>规则资源</strong><small>{{ enabledRules }} 个已启用，最近更新 {{ recentRule?.lastSuccessAt || "暂无" }}</small></span><AppIcon name="chevron" :size="16" /></a><a class="control-row" href="/core"><span class="control-icon violet"><AppIcon name="cpu" :size="16" /></span><span class="control-copy"><strong>核心状态</strong><small>{{ core.status.value?.lastError || "未报告错误" }}</small></span><AppIcon name="chevron" :size="16" /></a></div>
      </article>

      <div class="summary-grid">
        <article class="card summary-card"><span class="summary-icon"><AppIcon name="folder" /></span><div><small>配置档案</small><strong>{{ profiles.length }}</strong><em>{{ needsApply ? `${needsApply} 个需要应用` : "均已应用或待编辑" }}</em></div></article>
        <article class="card summary-card"><span class="summary-icon cyan"><AppIcon name="database" /></span><div><small>规则资源</small><strong>{{ rules.length }}</strong><em>{{ enabledRules }} 个启用</em></div></article>
        <article class="card summary-card"><span class="summary-icon green"><AppIcon name="check" /></span><div><small>活动配置</small><strong class="summary-name">{{ activeProfile?.name || "未应用" }}</strong><em>{{ activeProfile?.needsApply ? "草稿有待应用更改" : "服务端状态正常" }}</em></div></article>
        <article class="card summary-card"><span class="summary-icon orange"><AppIcon name="shield" /></span><div><small>配置状态</small><strong>{{ coreState }}</strong><em>{{ core.status.value?.lastError || "未报告错误" }}</em></div></article>
      </div>

      <div class="dashboard-lower">
        <article class="card service-summary"><header class="card-head"><div><h2 class="card-title">服务摘要</h2><p class="card-subtitle">当前服务只呈现可验证的配置与运行事实</p></div><a class="text-link" href="/rules">规则资源</a></header><div class="service-list"><div><span>已安装核心</span><strong>{{ core.status.value?.installedVersion || "否" }}</strong></div><div><span>当前应用修订</span><strong>{{ core.status.value?.appliedRevision ?? "—" }}</strong></div><div><span>最近规则更新</span><strong>{{ recentRule?.lastSuccessAt || "暂无" }}</strong></div><div><span>下次规则更新</span><strong>{{ recentRule?.nextUpdateAt || "按资源计划" }}</strong></div></div></article>
        <article class="card active-profile"><header><span class="profile-icon"><AppIcon name="config" /></span><span class="badge" :class="activeProfile ? 'badge-success' : 'badge-orange'">{{ activeProfile ? "当前档案" : "尚未应用" }}</span></header><h2>{{ activeProfile?.name || "尚未选择配置档案" }}</h2><p>{{ activeProfile?.needsApply ? "草稿已保存，但仍需手动应用到核心。" : "档案内容与服务端保存状态保持一致。" }}</p><div class="profile-stats"><div><strong>{{ draftStats.inbounds }}</strong><span>入站</span></div><div><strong>{{ draftStats.outbounds }}</strong><span>出站</span></div><div><strong>{{ draftStats.routeRules }}</strong><span>路由规则</span></div></div><a class="text-link profile-link" href="/workspace">打开配置工作台</a></article>
      </div>
    </div>
  </section>
</template>

<style scoped>
.dashboard-page { display: grid; gap: 18px; }.page-intro { display: flex; align-items: end; justify-content: space-between; gap: 18px; }.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.45px; }.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: 12px; }.page-actions { display: flex; gap: 8px; }.btn { display: inline-flex; min-height: 35px; align-items: center; gap: 7px; padding: 7px 11px; border: 1px solid var(--line); border-radius: 8px; color: var(--text-soft); background: var(--surface); font-size: 11px; font-weight: 660; text-decoration: none; }.btn:hover { color: var(--text); border-color: var(--line-strong); }.btn-primary { border-color: var(--primary); color: #fff; background: var(--primary); }.error { margin: 0; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); }
.dashboard-grid { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(300px, .8fr); gap: 12px; }.card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.hero-status { position: relative; min-height: 196px; overflow: hidden; border: 0; color: #fff; background: linear-gradient(124deg, #2d45ab 0%, #4a6cf2 54%, #6480ff 100%); box-shadow: 0 22px 48px -22px rgb(57 81 184 / 60%); }.hero-status.stopped { background: linear-gradient(124deg, #465166, #69758c 54%, #8491a8); }.hero-status::before { position: absolute; top: -110px; right: -50px; width: 330px; height: 330px; border: 1px solid rgb(255 255 255 / 11%); border-radius: 50%; content: ""; box-shadow: 0 0 0 42px rgb(255 255 255 / 4%), 0 0 0 92px rgb(255 255 255 / 3%); }.hero-status::after { position: absolute; right: 74px; bottom: -94px; width: 190px; height: 190px; border-radius: 50%; content: ""; background: radial-gradient(circle at 32% 28%, rgb(255 255 255 / 26%), rgb(255 255 255 / 2%) 62%); }.hero-content { position: relative; z-index: 2; display: flex; min-height: 196px; flex-direction: column; justify-content: space-between; padding: 20px 22px 18px; }.hero-eyebrow { display: flex; align-items: center; gap: 8px; color: rgb(255 255 255 / 70%); font-size: 10px; font-weight: 740; letter-spacing: 1.25px; text-transform: uppercase; }.pulse-dot { width: 7px; height: 7px; border-radius: 50%; background: #6cf0bb; box-shadow: 0 0 0 5px rgb(108 240 187 / 15%); }.pulse-dot.stopped { background: #ffd084; box-shadow: 0 0 0 5px rgb(255 208 132 / 15%); }.hero-main { max-width: 560px; padding: 17px 0 8px; }.hero-main h2 { margin: 0; font-size: 22px; font-weight: 730; letter-spacing: -.55px; }.hero-main p { margin: 6px 0 0; color: rgb(255 255 255 / 72%); font-size: 12px; }.hero-footer { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }.hero-meta { display: flex; flex-wrap: wrap; gap: 8px 24px; }.hero-meta-item span { display: block; color: rgb(255 255 255 / 55%); font-size: 9px; font-weight: 680; letter-spacing: .7px; text-transform: uppercase; }.hero-meta-item strong { display: block; max-width: 130px; overflow: hidden; margin-top: 2px; color: #fff; font-size: 12px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }.hero-actions { display: flex; gap: 8px; }.hero-btn { display: inline-flex; min-height: 35px; align-items: center; gap: 7px; padding: 7px 11px; border: 1px solid rgb(255 255 255 / 20%); border-radius: 9px; color: #fff; background: rgb(255 255 255 / 10%); font-size: 11px; font-weight: 650; text-decoration: none; backdrop-filter: blur(8px); }.hero-btn:hover { background: rgb(255 255 255 / 18%); }.hero-btn-danger { color: #ffeff0; background: rgb(107 20 35 / 22%); }
.control-card { display: flex; min-height: 196px; flex-direction: column; }.card-head { display: flex; min-height: 50px; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 14px; border-bottom: 1px solid var(--line); }.control-card .card-head { padding: 17px 18px 13px; border-bottom: 0; }.card-title { margin: 0; font-size: 14px; font-weight: 710; }.card-subtitle { margin: 2px 0 0; color: var(--text-faint); font-size: 10px; }.badge { display: inline-flex; min-height: 22px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface-soft); font-size: 10px; font-weight: 660; white-space: nowrap; }.badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }.badge-success { color: var(--green); background: var(--green-soft); }.badge-orange { color: var(--orange); background: var(--orange-soft); }.control-list { display: grid; gap: 2px; padding: 0 10px 10px; }.control-row { display: flex; min-height: 41px; align-items: center; gap: 11px; padding: 7px 9px; border-radius: 10px; color: var(--text-faint); text-decoration: none; }.control-row:hover { background: var(--surface-soft); }.control-icon { display: grid; width: 31px; height: 31px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.control-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.control-icon.violet { color: var(--violet); background: color-mix(in srgb, var(--violet) 12%, transparent); }.control-copy { min-width: 0; flex: 1; }.control-copy strong, .control-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.control-copy strong { color: var(--text); font-size: 11px; font-weight: 670; }.control-copy small { margin-top: 1px; color: var(--text-faint); font-size: 9px; }
.summary-grid { display: grid; grid-column: 1 / -1; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }.summary-card { display: flex; min-height: 104px; align-items: center; gap: 12px; padding: 15px; }.summary-icon { display: grid; width: 37px; height: 37px; flex: 0 0 auto; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.summary-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }.summary-icon.green { color: var(--green); background: var(--green-soft); }.summary-icon.orange { color: var(--orange); background: var(--orange-soft); }.summary-card div { min-width: 0; }.summary-card small, .summary-card strong, .summary-card em { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.summary-card small { color: var(--text-faint); font-size: 10px; }.summary-card strong { margin-top: 1px; color: var(--text); font-size: 20px; letter-spacing: -.4px; }.summary-card strong.summary-name { font-size: 14px; }.summary-card em { margin-top: 2px; color: var(--text-soft); font-size: 9px; font-style: normal; }.dashboard-lower { display: grid; grid-column: 1 / -1; grid-template-columns: minmax(0, 1.7fr) minmax(250px, .8fr); gap: 12px; }.service-summary { min-height: 210px; }.service-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 22px; padding: 7px 16px; }.service-list div { display: flex; min-height: 43px; align-items: center; justify-content: space-between; gap: 14px; border-bottom: 1px solid var(--line); }.service-list span { color: var(--text-soft); font-size: 11px; }.service-list strong { max-width: 55%; overflow: hidden; color: var(--text); font-size: 11px; text-align: right; text-overflow: ellipsis; white-space: nowrap; }.text-link { color: var(--primary); font-size: 11px; font-weight: 650; text-decoration: none; }.active-profile { padding: 16px; }.active-profile header { display: flex; align-items: center; justify-content: space-between; }.profile-icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 10px; color: var(--primary); background: var(--primary-soft); }.active-profile h2 { overflow: hidden; margin: 13px 0 0; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }.active-profile p { min-height: 34px; margin: 4px 0 0; color: var(--text-faint); font-size: 10px; }.profile-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-top: 13px; }.profile-stats div { padding: 8px 0; border-top: 1px solid var(--line); }.profile-stats strong, .profile-stats span { display: block; }.profile-stats strong { font-size: 15px; }.profile-stats span { margin-top: 1px; color: var(--text-faint); font-size: 9px; }.profile-link { display: inline-block; margin-top: 7px; }
@media (max-width: 980px) { .dashboard-grid, .dashboard-lower { grid-template-columns: 1fr; }.summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }.service-summary { min-height: auto; } }.service-list { grid-template-columns: 1fr; }
@media (max-width: 680px) { .dashboard-page { gap: 14px; }.page-intro { display: block; }.page-actions { margin-top: 12px; }.page-actions .btn { flex: 1; justify-content: center; }.hero-content { min-height: 234px; padding: 18px; }.hero-main h2 { font-size: 20px; }.hero-footer { display: block; }.hero-actions { margin-top: 13px; }.hero-btn { flex: 1; justify-content: center; }.hero-meta { gap: 8px 15px; }.summary-grid { grid-template-columns: 1fr 1fr; gap: 9px; }.summary-card { min-height: 88px; padding: 11px; }.summary-icon { width: 32px; height: 32px; }.summary-card strong { font-size: 17px; }.summary-card strong.summary-name { font-size: 12px; }.service-list { padding-inline: 13px; }.control-card .card-head { padding-inline: 15px; } }
</style>
