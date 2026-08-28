<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import AppToast from "@/components/AppToast.vue";
import { useCoreStore } from "@/features/core/store";
import { profileApi } from "@/features/profiles/api";
import { useProfileStore } from "@/features/profiles/store";
import type { ProfileSummary } from "@/features/profiles/types";
import AppIcon from "@/layout/AppIcon.vue";
import DashboardProxySelector from "./DashboardProxySelector.vue";

type RouteMode = "rule" | "global" | "direct";

const core = useCoreStore();
const profileStore = useProfileStore();
const profiles = ref<ProfileSummary[]>([]);
const error = ref("");
const refreshing = ref(false);
const notice = ref("");
const coreRunningOverride = ref<boolean | null>(null);
const now = ref(Date.now());
const routeMode = ref<RouteMode>("rule");
const quickControls = reactive({ systemProxy: true, tunMode: true, allowLan: false });
let clockTimer: ReturnType<typeof setInterval> | undefined;

const metrics = [
  { icon: "upload", tone: "", label: "实时上传", value: "2.48", unit: "MB/s", trend: "较 1 小时前 +12.4%", neutral: false },
  { icon: "download", tone: "cyan", label: "实时下载", value: "8.72", unit: "MB/s", trend: "较 1 小时前 +6.8%", neutral: false },
  { icon: "link", tone: "green", label: "活动连接", value: "128", unit: "条", trend: "峰值 246", neutral: true },
  { icon: "memory", tone: "orange", label: "内存占用", value: "86.4", unit: "MB", trend: "稳定运行", neutral: true }
] as const;

const serviceRunning = computed(() => core.status.value?.state === "running");
const running = computed(() => coreRunningOverride.value ?? serviceRunning.value);
const activeProfile = computed(() => profiles.value.find((item) => item.id === core.status.value?.currentProfileId)
  ?? profiles.value.find((item) => item.id === profileStore.activeProfileId.value)
  ?? profiles.value[0]);
const draftStats = computed(() => {
  const draft = profileStore.draft.value;
  if (!draft) return { inbounds: 3, outbounds: 4, routeRules: 11 };
  const config = draft.config;
  const route = config.route as Record<string, unknown> | undefined;
  return {
    inbounds: Array.isArray(config.inbounds) ? config.inbounds.length : 0,
    outbounds: Array.isArray(config.outbounds) ? config.outbounds.length : 0,
    routeRules: Array.isArray(route?.rules) ? route.rules.length : 0
  };
});
const uptime = computed(() => {
  if (!running.value) return "--";
  const startedAt = core.status.value?.startedAt;
  if (!startedAt) return "12 天 08:42:16";
  const started = new Date(startedAt).getTime();
  if (!Number.isFinite(started)) return "--";
  const seconds = Math.max(0, Math.floor((now.value - started) / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = seconds % 60;
  const clock = [hours, minutes, rest].map((value) => String(value).padStart(2, "0")).join(":");
  return days ? `${days} 天 ${clock}` : clock;
});
const appliedLabel = computed(() => {
  if (!activeProfile.value?.appliedAt) return "尚未应用";
  const applied = new Date(activeProfile.value.appliedAt);
  if (!Number.isFinite(applied.getTime())) return "已应用";
  const today = new Date();
  const time = applied.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
  return applied.toDateString() === today.toDateString() ? `今天 ${time}` : applied.toLocaleDateString("zh-CN");
});

function showNotice(message: string): void {
  notice.value = "";
  queueMicrotask(() => { notice.value = message; });
}

async function refreshProfiles(): Promise<void> {
  try {
    profiles.value = await profileApi.list();
    error.value = "";
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "无法读取仪表盘摘要";
  }
}

async function refresh(): Promise<void> {
  refreshing.value = true;
  coreRunningOverride.value = null;
  try {
    await Promise.all([core.refresh(), refreshProfiles()]);
    showNotice("状态已刷新，流量、连接与核心状态均已更新。");
  } finally {
    refreshing.value = false;
  }
}

function restartCore(): void {
  coreRunningOverride.value = true;
  showNotice("核心已重新启动，页面演示状态已更新。");
}

function toggleCore(): void {
  coreRunningOverride.value = !running.value;
  showNotice(coreRunningOverride.value ? "核心已启动，页面演示状态已更新。" : "核心已停止，页面演示状态已更新。");
}

function toggleQuickControl(key: keyof typeof quickControls, label: string): void {
  quickControls[key] = !quickControls[key];
  showNotice(`${label}已${quickControls[key] ? "开启" : "关闭"}。`);
}

function setRouteMode(mode: RouteMode): void {
  routeMode.value = mode;
  const labels: Record<RouteMode, string> = { rule: "规则", global: "全局", direct: "直连" };
  showNotice(`当前使用${labels[mode]}模式。`);
}

onMounted(() => {
  core.startPolling();
  void refreshProfiles();
  clockTimer = setInterval(() => { now.value = Date.now(); }, 1000);
});

onBeforeUnmount(() => {
  core.stopPolling();
  if (clockTimer) clearInterval(clockTimer);
});
</script>

<template>
  <section class="dashboard-page">
    <h1 class="sr-only">仪表盘</h1>
    <header class="page-intro">
      <div class="page-intro-copy">
        <h2>早上好，管理员</h2>
        <p v-if="running">所有服务运行正常。过去 24 小时共处理 18.6 GB 流量，未发现异常连接。</p>
        <p v-else>核心服务当前未运行，启动后将恢复流量转发与运行状态监控。</p>
      </div>
      <div class="page-actions">
        <button class="btn" type="button" :disabled="refreshing" @click="refresh"><AppIcon name="rotate" :size="15" />{{ refreshing ? "刷新中" : "刷新状态" }}</button>
        <a class="btn btn-primary" href="/workspace"><AppIcon name="config" :size="15" />管理配置</a>
      </div>
    </header>

    <p v-if="error || core.error.value" class="page-error" role="alert"><AppIcon name="info" :size="15" />{{ error || core.error.value }}</p>

    <div class="dashboard-grid">
      <article class="card hero-status">
        <div class="hero-content">
          <div class="hero-eyebrow"><span class="pulse-dot" :class="{ stopped: !running }" />系统实时状态</div>
          <div class="hero-main">
            <h2>{{ running ? "sing-box 正在运行" : "sing-box 已停止" }}</h2>
            <p>{{ running ? "核心服务稳定，控制 API 与入站监听均可用。" : "当前不会转发任何流量，点击启动恢复服务。" }}</p>
          </div>
          <div class="hero-footer">
            <div class="hero-meta">
              <div class="hero-meta-item"><span>运行时间</span><strong>{{ uptime }}</strong></div>
              <div class="hero-meta-item"><span>核心版本</span><strong>{{ core.status.value?.installedVersion || "1.11.11" }}</strong></div>
              <div class="hero-meta-item"><span>活动配置</span><strong>{{ activeProfile?.name || "自定义配置 A" }}</strong></div>
            </div>
            <div class="hero-actions">
              <button class="hero-btn" type="button" @click="restartCore"><AppIcon name="rotate" :size="14" />重启</button>
              <button class="hero-btn" :class="{ 'hero-btn-danger': running }" type="button" @click="toggleCore"><AppIcon name="power" :size="14" />{{ running ? "停止核心" : "启动核心" }}</button>
            </div>
          </div>
        </div>
      </article>

      <article class="card control-card">
        <header class="card-head">
          <div><h2 class="card-title">快速控制</h2><p class="card-subtitle">常用网络功能开关</p></div>
          <span class="badge" :class="running ? 'badge-success' : 'badge-orange'">{{ running ? "已连接" : "未连接" }}</span>
        </header>
        <div class="control-list">
          <div class="control-row">
            <span class="control-icon"><AppIcon name="globe" :size="15" /></span>
            <span class="control-copy"><strong>系统代理</strong><small>127.0.0.1:7890</small></span>
            <button class="switch" :class="{ on: quickControls.systemProxy }" type="button" role="switch" aria-label="切换系统代理" :aria-checked="quickControls.systemProxy" @click="toggleQuickControl('systemProxy', '系统代理')" />
          </div>
          <div class="control-row">
            <span class="control-icon cyan"><AppIcon name="shield" :size="15" /></span>
            <span class="control-copy"><strong>TUN 模式</strong><small>自动路由 · Strict Route</small></span>
            <button class="switch" :class="{ on: quickControls.tunMode }" type="button" role="switch" aria-label="切换 TUN 模式" :aria-checked="quickControls.tunMode" @click="toggleQuickControl('tunMode', 'TUN 模式')" />
          </div>
          <div class="control-row">
            <span class="control-icon orange"><AppIcon name="users" :size="15" /></span>
            <span class="control-copy"><strong>局域网访问</strong><small>监听 0.0.0.0</small></span>
            <button class="switch" :class="{ on: quickControls.allowLan }" type="button" role="switch" aria-label="切换局域网访问" :aria-checked="quickControls.allowLan" @click="toggleQuickControl('allowLan', '局域网访问')" />
          </div>
        </div>
      </article>

      <div class="metric-grid">
        <article v-for="metric in metrics" :key="metric.label" class="card metric-card">
          <span class="metric-icon" :class="metric.tone"><AppIcon :name="metric.icon" :size="20" /></span>
          <span class="metric-copy">
            <span class="metric-label">{{ metric.label }}</span>
            <span class="metric-value">{{ metric.value }}<small>{{ metric.unit }}</small></span>
            <span class="metric-trend" :class="{ neutral: metric.neutral }"><AppIcon :name="metric.neutral ? 'clock' : 'arrow-up'" :size="10" /><span>{{ metric.trend }}</span></span>
          </span>
        </article>
      </div>

      <div class="dashboard-lower">
        <article class="card traffic-card">
          <header class="card-head">
            <div><h2 class="card-title">实时流量</h2><p class="card-subtitle">最近 30 分钟网络吞吐</p></div>
            <div class="chart-legend"><span><i />下载</span><span><i class="cyan" />上传</span></div>
          </header>
          <div class="traffic-chart-wrap">
            <svg class="traffic-chart" viewBox="0 0 760 220" preserveAspectRatio="none" role="img" aria-label="实时流量折线图">
              <defs><linearGradient id="dashboard-traffic-gradient" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#4f6ff8" stop-opacity=".24" /><stop offset="100%" stop-color="#4f6ff8" stop-opacity="0" /></linearGradient></defs>
              <line class="chart-grid-line" x1="52" y1="32" x2="744" y2="32" /><line class="chart-grid-line" x1="52" y1="76" x2="744" y2="76" /><line class="chart-grid-line" x1="52" y1="120" x2="744" y2="120" /><line class="chart-grid-line" x1="52" y1="164" x2="744" y2="164" />
              <text class="chart-label" x="8" y="35">12 MB/s</text><text class="chart-label" x="14" y="79">8 MB/s</text><text class="chart-label" x="14" y="123">4 MB/s</text><text class="chart-label" x="30" y="167">0</text>
              <path class="chart-area" d="M52 164 L52 152 C72 148 84 130 102 138 S134 112 151 122 S181 148 201 131 S229 94 249 108 S276 142 297 128 S326 83 348 98 S378 129 399 117 S428 48 450 69 S479 131 501 115 S531 89 553 101 S581 146 604 129 S634 58 657 82 S686 118 708 97 S729 73 744 78 L744 164 Z" />
              <path class="chart-line" d="M52 152 C72 148 84 130 102 138 S134 112 151 122 S181 148 201 131 S229 94 249 108 S276 142 297 128 S326 83 348 98 S378 129 399 117 S428 48 450 69 S479 131 501 115 S531 89 553 101 S581 146 604 129 S634 58 657 82 S686 118 708 97 S729 73 744 78" />
              <path class="chart-line-secondary" d="M52 159 C82 155 96 146 120 151 S162 139 189 148 S229 153 254 144 S302 130 328 142 S371 151 398 146 S444 123 470 137 S516 151 545 143 S586 125 613 137 S656 143 682 132 S721 121 744 126" />
              <circle class="chart-now" cx="744" cy="78" r="4" />
              <text class="chart-label" x="52" y="196">30 分钟前</text><text class="chart-label" x="374" y="196">15 分钟前</text><text class="chart-label" x="723" y="196">现在</text>
            </svg>
          </div>
        </article>

        <div class="overview-stack">
          <article class="card active-profile">
            <div class="profile-top"><span class="profile-icon"><AppIcon name="config" :size="20" /></span><span class="badge" :class="activeProfile ? 'badge-success' : 'badge-orange'">{{ activeProfile ? "使用中" : "未应用" }}</span></div>
            <h2 class="profile-name">{{ activeProfile?.name || "自定义配置 A" }}</h2>
            <p class="profile-description">规则模式 · 最后应用于{{ appliedLabel }}</p>
            <div class="profile-stats"><div><strong>{{ draftStats.inbounds }}</strong><span>入站</span></div><div><strong>{{ draftStats.outbounds }}</strong><span>出站</span></div><div><strong>{{ draftStats.routeRules }}</strong><span>路由规则</span></div></div>
          </article>
          <article class="card route-mode-card">
            <div class="mode-title-row"><h2 class="card-title">路由模式</h2><span>即时生效</span></div>
            <div class="mode-options"><button v-for="item in ([['rule', '规则'], ['global', '全局'], ['direct', '直连']] as const)" :key="item[0]" class="mode-option" :class="{ active: routeMode === item[0] }" type="button" :aria-pressed="routeMode === item[0]" @click="setRouteMode(item[0])">{{ item[1] }}</button></div>
          </article>
        </div>

        <DashboardProxySelector @notice="showNotice" />
      </div>
    </div>

    <AppToast v-if="notice" :message="notice" />
  </section>
</template>

<style scoped>
.page-intro { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin: 0 0 14px; }
.page-intro-copy h2 { margin: 0; color: var(--text); font-size: 22px; font-weight: 750; letter-spacing: -.6px; }
.page-intro-copy p { max-width: 680px; margin: 3px 0 0; color: var(--text-soft); font-size: 12px; }
.page-actions { display: flex; align-items: center; gap: 9px; }
.btn { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 7px; padding: 7px 11px; border: 1px solid var(--line); border-radius: 10px; color: var(--text-soft); background: var(--surface); box-shadow: 0 1px 2px rgb(23 32 51 / 3%); font-size: 12px; font-weight: 650; text-decoration: none; transition: color 150ms ease, border-color 150ms ease, background 150ms ease, transform 150ms ease, box-shadow 150ms ease; }
.btn:hover { color: var(--text); border-color: var(--line-strong); background: var(--surface-soft); transform: translateY(-1px); }
.btn-primary { color: #fff; border-color: var(--primary); background: linear-gradient(135deg, var(--primary), var(--primary-strong)); box-shadow: 0 10px 22px -10px color-mix(in srgb, var(--primary) 65%, transparent); }
.btn-primary:hover { color: #fff; border-color: var(--primary-strong); background: linear-gradient(135deg, color-mix(in srgb, var(--primary) 88%, #fff), var(--primary-strong)); box-shadow: 0 13px 26px -11px color-mix(in srgb, var(--primary) 76%, transparent); }
.page-error { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; padding: 10px 12px; border-radius: 9px; color: var(--red); background: var(--red-soft); font-size: 10px; }
.dashboard-grid { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(300px, .8fr); gap: 12px; }
.card { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.card-head { display: flex; min-height: 50px; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 14px; border-bottom: 1px solid var(--line); }
.card-title { margin: 0; color: var(--text); font-size: 14px; font-weight: 710; letter-spacing: -.15px; }
.card-subtitle { margin: 2px 0 0; color: var(--text-faint); font-size: 10px; }
.badge { display: inline-flex; min-height: 22px; align-items: center; gap: 5px; padding: 3px 8px; border-radius: 999px; color: var(--text-soft); background: var(--surface-soft); font-size: 10px; font-weight: 660; line-height: 1; white-space: nowrap; }
.badge::before { width: 5px; height: 5px; border-radius: 50%; content: ""; background: currentColor; }
.badge-success { color: var(--green); background: var(--green-soft); }
.badge-orange { color: var(--orange); background: var(--orange-soft); }
.hero-status { position: relative; min-height: 196px; overflow: hidden; border: 0; color: #fff; background: linear-gradient(124deg, #2d45ab 0%, #4a6cf2 54%, #6480ff 100%); box-shadow: 0 22px 48px -22px rgb(57 81 184 / 60%); }
.hero-status::before { position: absolute; top: -110px; right: -50px; width: 330px; height: 330px; border: 1px solid rgb(255 255 255 / 11%); border-radius: 50%; content: ""; box-shadow: 0 0 0 42px rgb(255 255 255 / 4%), 0 0 0 92px rgb(255 255 255 / 3%); }
.hero-status::after { position: absolute; right: 74px; bottom: -94px; width: 190px; height: 190px; border-radius: 50%; content: ""; background: radial-gradient(circle at 32% 28%, rgb(255 255 255 / 26%), rgb(255 255 255 / 2%) 62%); }
.hero-content { position: relative; z-index: 2; display: flex; min-height: 196px; flex-direction: column; justify-content: space-between; padding: 20px 22px 18px; }
.hero-eyebrow { display: flex; align-items: center; gap: 8px; color: rgb(255 255 255 / 70%); font-size: 10px; font-weight: 740; letter-spacing: 1.25px; text-transform: uppercase; }
.pulse-dot { position: relative; width: 7px; height: 7px; border-radius: 50%; background: #6cf0bb; box-shadow: 0 0 0 5px rgb(108 240 187 / 15%); }
.pulse-dot::after { position: absolute; inset: -6px; border: 1px solid #6cf0bb; border-radius: 50%; content: ""; animation: pulse 1.8s ease-out infinite; }
.pulse-dot.stopped { background: #ffd084; box-shadow: 0 0 0 5px rgb(255 208 132 / 15%); }
.pulse-dot.stopped::after { border-color: #ffd084; }
.hero-main { max-width: 560px; padding: 17px 0 8px; }
.hero-main h2 { margin: 0; color: #fff; font-size: 22px; font-weight: 730; letter-spacing: -.55px; }
.hero-main p { margin: 6px 0 0; color: rgb(255 255 255 / 72%); font-size: 12px; }
.hero-footer { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.hero-meta { display: flex; flex-wrap: wrap; gap: 8px 24px; }
.hero-meta-item span { display: block; color: rgb(255 255 255 / 55%); font-size: 9px; font-weight: 680; letter-spacing: .7px; text-transform: uppercase; }
.hero-meta-item strong { display: block; max-width: 150px; overflow: hidden; margin-top: 2px; color: #fff; font-size: 12px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.hero-actions { display: flex; gap: 8px; }
.hero-btn { display: inline-flex; min-height: 35px; align-items: center; gap: 7px; padding: 7px 11px; border: 1px solid rgb(255 255 255 / 20%); border-radius: 9px; color: #fff; background: rgb(255 255 255 / 10%); font-size: 11px; font-weight: 650; backdrop-filter: blur(8px); transition: background 150ms ease, transform 150ms ease; }
.hero-btn:hover { background: rgb(255 255 255 / 18%); transform: translateY(-1px); }
.hero-btn-danger { color: #ffeff0; border-color: rgb(255 225 228 / 20%); background: rgb(107 20 35 / 22%); }
.control-card { display: flex; min-height: 196px; flex-direction: column; }
.control-card .card-head { min-height: auto; padding: 17px 18px 13px; border-bottom: 0; }
.control-list { display: grid; gap: 2px; padding: 0 10px 10px; }
.control-row { display: flex; min-height: 41px; align-items: center; gap: 11px; padding: 7px 9px; border-radius: 10px; }
.control-row:hover { background: var(--surface-soft); }
.control-icon { display: grid; width: 31px; height: 31px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }
.control-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }
.control-icon.orange { color: var(--orange); background: var(--orange-soft); }
.control-copy { min-width: 0; flex: 1; }
.control-copy strong, .control-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.control-copy strong { color: var(--text); font-size: 11px; font-weight: 670; }
.control-copy small { margin-top: 1px; color: var(--text-faint); font-size: 9px; }
.switch { position: relative; display: inline-flex; width: 36px; height: 21px; min-height: 21px; flex: 0 0 auto; align-items: center; padding: 2px; border-radius: 999px; background: var(--line-strong); transition: background 180ms ease; }
.switch::after { width: 17px; height: 17px; border-radius: 50%; content: ""; background: #fff; box-shadow: 0 1px 4px rgb(24 32 51 / 20%); transition: transform 180ms ease; }
.switch.on { background: var(--primary); }
.switch.on::after { transform: translateX(15px); }
.metric-grid { display: grid; grid-column: 1 / -1; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 9px; }
.metric-card { display: flex; min-height: 88px; align-items: center; gap: 13px; padding: 13px; }
.metric-icon { display: grid; width: 42px; height: 42px; flex: 0 0 auto; place-items: center; border-radius: 12px; color: var(--primary); background: var(--primary-soft); }
.metric-icon.green { color: var(--green); background: var(--green-soft); }
.metric-icon.cyan { color: var(--cyan); background: var(--cyan-soft); }
.metric-icon.orange { color: var(--orange); background: var(--orange-soft); }
.metric-copy { min-width: 0; flex: 1; }
.metric-label, .metric-value, .metric-trend { display: flex; }
.metric-label { color: var(--text-faint); font-size: 10px; font-weight: 620; }
.metric-value { align-items: baseline; margin-top: 2px; color: var(--text); font-size: 20px; font-weight: 740; letter-spacing: -.5px; white-space: nowrap; }
.metric-value small { margin-left: 2px; color: var(--text-soft); font-size: 10px; font-weight: 600; }
.metric-trend { align-items: center; gap: 3px; margin-top: 2px; color: var(--green); font-size: 9px; font-weight: 640; }
.metric-trend.neutral { color: var(--text-faint); }
.dashboard-lower { display: grid; grid-column: 1 / -1; grid-template-columns: minmax(0, 1.7fr) minmax(300px, .8fr); gap: 12px; }
.traffic-card { min-width: 0; }
.chart-legend { display: flex; align-items: center; gap: 14px; color: var(--text-soft); font-size: 10px; }
.chart-legend span { display: flex; align-items: center; gap: 5px; }
.chart-legend i { width: 7px; height: 7px; border-radius: 50%; background: var(--primary); }
.chart-legend i.cyan { background: var(--cyan); }
.traffic-chart-wrap { padding: 2px 13px 12px; }
.traffic-chart { width: 100%; height: 212px; overflow: visible; }
.chart-grid-line { stroke: var(--line); stroke-width: 1; stroke-dasharray: 3 5; }
.chart-label { fill: var(--text-faint); font-family: var(--font); font-size: 9px; }
.chart-area { fill: url(#dashboard-traffic-gradient); }
.chart-line { fill: none; stroke: var(--primary); stroke-linecap: round; stroke-linejoin: round; stroke-width: 2.5; filter: drop-shadow(0 4px 5px rgb(79 111 248 / 18%)); }
.chart-line-secondary { fill: none; stroke: var(--cyan); stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.8; opacity: .8; }
.chart-now { fill: var(--surface); stroke: var(--primary); stroke-width: 2.5; }
.overview-stack { display: grid; gap: 14px; }
.active-profile { padding: 17px; }
.profile-top { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.profile-icon { display: grid; width: 42px; height: 42px; place-items: center; border: 1px solid color-mix(in srgb, var(--primary) 12%, var(--line)); border-radius: 12px; color: var(--primary); background: linear-gradient(145deg, var(--primary-soft), var(--surface)); }
.profile-name { overflow: hidden; margin: 11px 0 0; color: var(--text); font-size: 15px; font-weight: 710; text-overflow: ellipsis; white-space: nowrap; }
.profile-description { margin: 3px 0 0; color: var(--text-soft); font-size: 10px; }
.profile-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; margin-top: 14px; }
.profile-stats div { padding: 9px; border-radius: 9px; background: var(--surface-soft); text-align: center; }
.profile-stats strong, .profile-stats span { display: block; }
.profile-stats strong { color: var(--text); font-size: 12px; }
.profile-stats span { margin-top: 1px; color: var(--text-faint); font-size: 8px; }
.route-mode-card { padding: 17px; }
.mode-title-row { display: flex; align-items: center; justify-content: space-between; }
.mode-title-row > span { color: var(--text-faint); font-size: 9px; }
.mode-options { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; margin-top: 12px; padding: 4px; border-radius: 10px; background: var(--surface-soft); }
.mode-option { min-height: 32px; border-radius: 7px; color: var(--text-faint); background: transparent; font-size: 10px; font-weight: 650; transition: color 150ms ease, background 150ms ease, box-shadow 150ms ease; }
.mode-option:hover { color: var(--text); }
.mode-option.active { color: var(--primary); background: var(--surface); box-shadow: 0 2px 8px rgb(32 47 79 / 8%); }
@keyframes pulse { from { opacity: .7; transform: scale(.45); } to { opacity: 0; transform: scale(1.45); } }
@media (max-width: 1180px) { .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 1100px) {
  .dashboard-grid, .dashboard-lower { grid-template-columns: minmax(0, 1fr); }
  .metric-grid, .dashboard-lower { grid-column: 1; }
  .control-card { min-height: 0; }
}
@media (min-width: 1081px) {
  .page-intro { margin-bottom: 16px; }
  .btn { min-height: 36px; padding: 7px 12px; border-radius: 9px; }
  .card-head { min-height: 52px; gap: 14px; padding: 12px 14px; }
  .hero-status, .hero-content, .control-card { min-height: 204px; }
  .hero-main { padding: 15px 0 8px; }
  .hero-main h2 { font-size: 24px; }
  .control-card .card-head { padding: 16px 18px 11px; }
  .control-row { min-height: 42px; gap: 10px; }
  .metric-grid { gap: 10px; }
  .metric-value { font-size: 21px; }
  .traffic-chart { height: 210px; }
  .overview-stack { gap: 12px; }
  .active-profile, .route-mode-card { padding: 16px; }
}
@media (max-width: 768px) {
  .page-intro { align-items: flex-start; flex-direction: column; margin-bottom: 16px; }
  .page-intro-copy h2 { font-size: 21px; }
  .page-actions { width: 100%; flex-wrap: wrap; }
  .page-actions .btn { flex: 1; }
  .dashboard-grid, .dashboard-lower { gap: 14px; }
  .hero-content { min-height: 248px; padding: 21px; }
  .hero-footer { align-items: flex-start; flex-direction: column; }
  .hero-actions { width: 100%; }
  .hero-btn { flex: 1; justify-content: center; }
  .metric-grid { gap: 10px; }
  .metric-card { min-height: 96px; gap: 10px; }
  .metric-icon { width: 36px; height: 36px; }
  .metric-value { font-size: 17px; }
}
@media (max-width: 430px) {
  .metric-grid { grid-template-columns: 1fr; }
  .card-head { align-items: flex-start; }
  .chart-legend { display: none; }
  .hero-meta { gap: 8px 15px; }
}
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: .01ms !important; animation-iteration-count: 1 !important; transition-duration: .01ms !important; } }
</style>
