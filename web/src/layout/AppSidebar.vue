<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { useCoreStore } from "@/features/core/store";
import { useProfileStore } from "@/features/profiles/store";
import AppIcon from "./AppIcon.vue";

defineProps<{ open: boolean; collapsed: boolean }>();
const emit = defineEmits<{ close: [] }>();
const route = useRoute();
const router = useRouter();
const profiles = useProfileStore();
const core = useCoreStore();
const configExpanded = ref(true);
const configActive = computed(() => route.path === "/workspace" || route.path === "/profiles");
const coreVersion = computed(() => core.status.value?.installedVersion || "未安装");
const coreState = computed(() => core.status.value?.state === "running" ? "核心运行中" : core.status.value?.state === "not_installed" ? "尚未安装核心" : "核心已停止");

onMounted(async () => {
  try {
    await profiles.refresh();
    if (profiles.profiles.value[0] && !profiles.activeProfileId.value) await profiles.open(profiles.profiles.value[0].id);
  } catch {
    // 页面各自会提供可读的加载错误；侧栏不覆盖真实错误信息。
  }
  await core.refresh();
});

async function selectProfile(id: string): Promise<void> {
  if (!await profiles.open(id)) return;
  await router.push({ name: "workspace" });
  emit("close");
}

function close(): void { emit("close"); }
</script>

<template>
  <aside class="app-sidebar sidebar" :class="{ open, collapsed }" aria-label="主导航">
    <div class="brand">
      <div class="brand-mark"><span>S</span></div>
      <div class="brand-copy"><strong>SingBox Control</strong><small>Web panel</small></div>
    </div>
    <nav class="sidebar-scroll" aria-label="主导航">
      <RouterLink class="nav-item" to="/" @click="close"><AppIcon name="dashboard" /><span>仪表盘</span></RouterLink>

      <div class="nav-parent-row">
        <RouterLink class="nav-parent" :class="{ active: configActive, expanded: configExpanded }" to="/workspace" @click="configExpanded = !configExpanded; close()"><AppIcon name="config" /><span>配置</span><b class="nav-count">{{ profiles.profiles.value.length }}</b><AppIcon class="nav-chevron" name="chevron" :size="16" /></RouterLink>
        <RouterLink class="nav-parent-manage" to="/profiles" aria-label="管理档案" title="管理档案" @click="close"><AppIcon name="folder" :size="17" /></RouterLink>
      </div>
      <div v-if="configExpanded" class="nav-config-tree">
        <div v-for="profile in profiles.profiles.value" :key="profile.id" class="config-tree-item" :class="{ active: profile.id === profiles.activeProfileId.value }">
          <button class="config-name" type="button" :disabled="Boolean(profile.quarantinedError)" :title="`切换到 ${profile.name}`" @click="selectProfile(profile.id)"><i class="config-status-dot" /><span>{{ profile.name }}</span><i v-if="profile.needsApply" class="config-dirty-dot" title="需要应用" /></button>
        </div>
      </div>

      <RouterLink class="nav-item" to="/inbounds" @click="close"><AppIcon name="inbound" /><span>入站配置</span></RouterLink>
      <RouterLink class="nav-item" to="/subscriptions" @click="close"><AppIcon name="subscription" /><span>订阅配置</span></RouterLink>
      <RouterLink class="nav-item" to="/rules" @click="close"><AppIcon name="rules" /><span>规则集</span></RouterLink>
      <RouterLink class="nav-item" to="/core" @click="close"><AppIcon name="cpu" /><span>核心管理</span></RouterLink>

      <RouterLink class="nav-item" to="/settings" @click="close"><AppIcon name="settings" /><span>设置</span></RouterLink>
    </nav>
    <footer class="sidebar-footer">
      <RouterLink class="core-mini" to="/core" @click="close"><span class="core-mini-icon"><AppIcon name="cpu" :size="17" /></span><span class="core-mini-copy"><strong>sing-box {{ coreVersion }}</strong><small :class="{ online: core.status.value?.state === 'running' }">{{ coreState }}</small></span><AppIcon class="core-chevron" name="chevron" :size="15" /></RouterLink>
    </footer>
  </aside>
</template>

<style scoped>
.sidebar { position: fixed; z-index: 40; inset: 0 auto 0 0; display: flex; width: var(--sidebar-width); flex-direction: column; overflow: hidden; border-right: 1px solid var(--line); background: color-mix(in srgb, var(--sidebar) 96%, transparent); box-shadow: 12px 0 36px -34px rgb(25 39 71 / 55%); backdrop-filter: blur(18px); transition: width 180ms ease, transform 220ms ease; }
.brand { display: flex; min-height: 64px; align-items: center; gap: 11px; padding: 12px 16px; }.brand-mark { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; overflow: hidden; border-radius: 8px; color: #fff; background: linear-gradient(145deg, #5978ff, #3d5ee9); box-shadow: 0 8px 18px -9px #3858e8; font-size: 11px; font-weight: 800; }.brand-copy { min-width: 0; }.brand-copy strong, .brand-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.brand-copy strong { font-size: 13px; letter-spacing: -.15px; }.brand-copy small { margin-top: 1px; color: var(--text-faint); font-size: 9px; }
.sidebar-scroll { flex: 1; overflow-y: auto; padding: 8px 10px 18px; }.nav-label { margin: 15px 9px 6px; color: var(--text-faint); font-size: 9px; font-weight: 750; letter-spacing: .85px; text-transform: uppercase; }.nav-label:first-child { margin-top: 4px; }.nav-item, .nav-parent, .config-name { display: flex; width: 100%; min-height: 38px; align-items: center; gap: 10px; padding: 8px 10px; border-radius: 8px; color: var(--text-soft); background: transparent; font-size: 12px; font-weight: 620; text-align: left; text-decoration: none; transition: color 150ms ease, background 150ms ease; }.nav-item:hover, .nav-parent:hover, .config-name:hover { color: var(--text); background: var(--surface-soft); }.nav-item.router-link-exact-active, .nav-parent.active { position: relative; color: var(--primary-strong); background: var(--primary-soft); }.nav-item.router-link-exact-active::before, .nav-parent.active::before { position: absolute; left: -10px; width: 3px; height: 18px; border-radius: 0 4px 4px 0; content: ""; background: var(--primary); }.nav-parent-row { display: flex; align-items: center; gap: 2px; }.nav-parent { min-width: 0; flex: 1; }.nav-parent .nav-chevron { margin-left: auto; transition: transform 180ms ease; }.nav-parent.expanded .nav-chevron { transform: rotate(90deg); }.nav-count { display: inline-grid; min-width: 18px; height: 18px; place-items: center; border-radius: 6px; color: var(--primary); background: var(--primary-soft); font-size: 9px; }.nav-parent-manage { display: inline-grid; width: 30px; height: 30px; flex: 0 0 auto; place-items: center; border-radius: 7px; color: var(--text-faint); text-decoration: none; }.nav-parent-manage:hover { color: var(--primary); background: var(--primary-soft); }
.nav-config-tree { position: relative; display: grid; gap: 1px; margin: 3px 0 7px 19px; padding-left: 10px; }.nav-config-tree::before { position: absolute; inset: 0 auto 0 0; width: 1px; content: ""; background: var(--line); }.config-tree-item { display: flex; min-width: 0; align-items: center; }.config-name { min-width: 0; min-height: 31px; flex: 1; gap: 7px; padding: 6px 7px; border-radius: 7px; font-size: 10px; }.config-name span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.config-tree-item.active .config-name { color: var(--primary-strong); background: var(--primary-soft); }.config-status-dot { width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: var(--green); box-shadow: 0 0 0 3px var(--green-soft); }.config-dirty-dot { width: 5px; height: 5px; margin-left: auto; border-radius: 50%; background: var(--orange); }
.sidebar-footer { padding: 10px; border-top: 1px solid var(--line); }.core-mini { display: flex; min-height: 49px; align-items: center; gap: 9px; padding: 8px; border-radius: 10px; color: var(--text); text-decoration: none; }.core-mini:hover { background: var(--surface-soft); }.core-mini-icon { display: grid; width: 29px; height: 29px; flex: 0 0 auto; place-items: center; border-radius: 8px; color: var(--violet); background: color-mix(in srgb, var(--violet) 13%, transparent); }.core-mini-copy { min-width: 0; flex: 1; }.core-mini-copy strong, .core-mini-copy small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.core-mini-copy strong { font-size: 10px; }.core-mini-copy small { margin-top: 2px; color: var(--text-faint); font-size: 9px; }.core-mini-copy small.online { color: var(--green); }.core-chevron { color: var(--text-faint); }
.sidebar.collapsed { width: 72px; }.sidebar.collapsed .brand { padding-inline: 19px; }.sidebar.collapsed .brand-copy, .sidebar.collapsed .nav-label, .sidebar.collapsed .nav-item span, .sidebar.collapsed .nav-parent span, .sidebar.collapsed .nav-count, .sidebar.collapsed .nav-chevron, .sidebar.collapsed .nav-config-tree, .sidebar.collapsed .core-mini-copy, .sidebar.collapsed .core-chevron { display: none; }.sidebar.collapsed .nav-parent-row { display: block; }.sidebar.collapsed .nav-item, .sidebar.collapsed .nav-parent { justify-content: center; padding-inline: 8px; }.sidebar.collapsed .nav-parent-manage { display: none; }.sidebar.collapsed .nav-item.router-link-exact-active::before, .sidebar.collapsed .nav-parent.active::before { left: -10px; }
@media (max-width: 1100px) { .sidebar, .sidebar.collapsed { width: var(--sidebar-width); transform: translateX(-100%); box-shadow: var(--shadow-lg); }.sidebar.open { transform: translateX(0); }.sidebar.collapsed .brand-copy, .sidebar.collapsed .nav-label, .sidebar.collapsed .nav-item span, .sidebar.collapsed .nav-parent span, .sidebar.collapsed .nav-count, .sidebar.collapsed .nav-chevron, .sidebar.collapsed .nav-config-tree, .sidebar.collapsed .core-mini-copy, .sidebar.collapsed .core-chevron { display: block; }.sidebar.collapsed .nav-parent-row { display: flex; }.sidebar.collapsed .nav-item, .sidebar.collapsed .nav-parent { justify-content: flex-start; padding-inline: 10px; }.sidebar.collapsed .nav-parent-manage { display: inline-grid; } }
</style>
