<script setup lang="ts">
import { useRouter } from "vue-router";
import { useAuthStore } from "@/features/auth/store";

const router = useRouter();
const auth = useAuthStore();

async function logout(): Promise<void> {
  await auth.logout();
  await router.replace({ name: "login" });
}
</script>

<template>
  <header class="app-topbar topbar">
    <button class="menu-button" type="button" aria-label="切换导航菜单" @click="$emit('toggle-menu')">☰</button>
    <div class="topbar-title"><span>sing-box</span><small>管理面板</small></div>
    <button class="logout" type="button" @click="logout">退出</button>
  </header>
</template>

<style scoped>
.topbar { display: flex; height: 64px; align-items: center; gap: 14px; padding: 0 22px; border-bottom: 1px solid var(--line); background: color-mix(in srgb, var(--surface) 90%, transparent); backdrop-filter: blur(12px); }
.topbar-title { display: grid; line-height: 1.1; font-weight: 750; }
.topbar-title small { margin-top: 3px; color: var(--text-faint); font-size: 11px; font-weight: 500; }
.menu-button { display: none; width: auto; padding: 7px 10px; background: transparent; color: var(--text); }
.logout { width: auto; margin-left: auto; padding: 8px 13px; background: var(--surface-soft); color: var(--text-soft); }
@media (max-width: 760px) { .menu-button { display: inline-flex; } }
</style>
