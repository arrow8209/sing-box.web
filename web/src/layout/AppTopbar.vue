<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/features/auth/store";
import AppIcon from "./AppIcon.vue";

defineProps<{ collapsed: boolean }>();
const emit = defineEmits<{ toggleMenu: []; toggleCollapse: []; toggleTheme: [] }>();
const router = useRouter();
const route = useRoute();
const auth = useAuthStore();
const dark = ref(false);
const meta = computed(() => ({
  "/": ["仪表盘", "控制台"],
  "/workspace": ["配置工作台", "配置"],
  "/profiles": ["配置档案", "配置"],
  "/inbounds": ["入站配置", "资源"],
  "/subscriptions": ["订阅配置", "资源"],
  "/rules": ["规则集中心", "资源"],
  "/core": ["核心管理", "系统"],
  "/connections": ["活动连接", "运行数据"],
  "/runtime": ["运行数据", "系统"],
  "/settings": ["设置", "系统"]
}[route.path] ?? ["管理面板", "控制台"]));

async function logout(): Promise<void> {
  await auth.logout();
  await router.replace({ name: "login" });
}

function toggleTheme(): void {
  dark.value = !dark.value;
  emit("toggleTheme");
}

onMounted(() => { dark.value = document.documentElement.dataset.theme === "dark"; });
</script>

<template>
  <header class="app-topbar topbar">
    <button class="mobile-menu icon-button" type="button" aria-label="打开菜单" @click="$emit('toggleMenu')"><AppIcon name="menu" /></button>
    <button class="desktop-sidebar-toggle icon-button" type="button" :aria-label="collapsed ? '展开侧栏' : '收起侧栏'" :title="collapsed ? '展开侧栏' : '收起侧栏'" @click="$emit('toggleCollapse')"><AppIcon name="menu" /></button>
    <div class="page-heading"><strong>{{ meta[0] }}</strong><div class="breadcrumbs"><span>{{ meta[1] }}</span><i>/</i><span class="current">{{ meta[0] }}</span></div></div>
    <div class="topbar-actions">
      <button class="top-search" type="button" disabled title="一期暂未提供全局搜索"><span>搜索或跳转...</span><kbd>⌘ K</kbd></button>
      <button class="icon-button" type="button" aria-label="切换主题" title="切换主题" @click="toggleTheme"><AppIcon :name="dark ? 'sun' : 'moon'" /></button>
      <button class="icon-button notification" type="button" aria-label="通知" title="一期暂未提供通知"><AppIcon name="bell" /><i /></button>
      <button class="avatar" type="button" title="退出登录" aria-label="退出登录" @click="logout">AD</button>
    </div>
  </header>
</template>

<style scoped>
.topbar { position: sticky; z-index: 20; inset-block-start: 0; display: flex; min-height: var(--topbar-height); align-items: center; gap: var(--section-gap); padding: 0 var(--page-padding); border-bottom: 1px solid var(--line); background: color-mix(in srgb, var(--surface) 90%, transparent); backdrop-filter: blur(12px); }.icon-button { display: inline-grid; width: 34px; min-height: 34px; place-items: center; padding: 0; border: 1px solid transparent; border-radius: 8px; color: var(--text-soft); background: transparent; }.icon-button:hover { color: var(--text); border-color: var(--line); background: var(--surface); }.mobile-menu { display: none; }.page-heading { min-width: 0; }.page-heading strong { display: block; overflow: hidden; margin: 0; color: var(--text); font-size: 17px; font-weight: 730; letter-spacing: -.3px; text-overflow: ellipsis; white-space: nowrap; }.breadcrumbs { display: flex; align-items: center; gap: 6px; margin-top: 2px; color: var(--text-faint); font-size: 10px; }.breadcrumbs i { color: var(--line-strong); font-style: normal; }.breadcrumbs .current { color: var(--text-soft); }.topbar-actions { display: flex; align-items: center; gap: 6px; margin-left: auto; }.top-search { display: flex; min-height: 32px; align-items: center; gap: 30px; padding: 6px 8px 6px 11px; border: 1px solid var(--line); border-radius: 8px; color: var(--text-faint); background: var(--surface-soft); font-size: 10px; }.top-search kbd { padding: 1px 5px; border: 1px solid var(--line); border-radius: 4px; color: var(--text-faint); background: var(--surface); font-size: 9px; }.notification { position: relative; }.notification i { position: absolute; top: 7px; right: 7px; width: 5px; height: 5px; border: 1px solid var(--surface); border-radius: 50%; background: var(--red); }.avatar { display: grid; width: 30px; min-height: 30px; place-items: center; margin-left: 4px; border-radius: 50%; color: #fff; background: linear-gradient(145deg, #6a7fe8, #a26ddd); font-size: 9px; font-weight: 760; letter-spacing: .35px; }
@media (max-width: 1100px) { .desktop-sidebar-toggle { display: none; }.mobile-menu { display: inline-grid; } }.top-search:disabled { cursor: not-allowed; opacity: 1; }
@media (max-width: 680px) { .topbar { padding-inline: var(--page-padding); }.top-search, .notification { display: none; }.page-heading strong { font-size: 15px; }.breadcrumbs { display: none; }.topbar-actions { gap: 2px; }.avatar { margin-left: 2px; } }
</style>
