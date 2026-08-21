<script setup lang="ts">
import { RouterLink } from "vue-router";

defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();

const links = [
  { to: "/", label: "概览", icon: "◈" },
  { to: "/profiles", label: "配置档案", icon: "▤" },
  { to: "/rules", label: "规则资源", icon: "◫" },
  { to: "/core", label: "核心管理", icon: "◉" }
];
</script>

<template>
  <aside class="sidebar" :class="{ open }">
    <div class="brand"><span class="brand-mark">S</span><span>sing-box</span></div>
    <nav aria-label="主导航">
      <RouterLink v-for="link in links" :key="link.to" :to="link.to" @click="emit('close')">
        <span aria-hidden="true">{{ link.icon }}</span>{{ link.label }}
      </RouterLink>
    </nav>
  </aside>
</template>

<style scoped>
.sidebar { position: fixed; z-index: 10; inset: 0 auto 0 0; width: var(--sidebar-width); padding: 20px 14px; border-right: 1px solid var(--line); background: var(--sidebar); transition: transform .18s ease; }
.brand { display: flex; align-items: center; gap: 10px; padding: 6px 10px 28px; font-size: 18px; font-weight: 750; }
.brand-mark { display: grid; width: 28px; height: 28px; place-items: center; border-radius: 9px; background: var(--primary); color: #fff; font-size: 14px; }
nav { display: grid; gap: 4px; } nav a { display: flex; align-items: center; gap: 11px; padding: 10px; border-radius: var(--radius-sm); color: var(--text-soft); font-weight: 600; text-decoration: none; } nav a.router-link-exact-active { background: var(--primary-soft); color: var(--primary-strong); }
@media (max-width: 760px) { .sidebar { transform: translateX(-100%); box-shadow: var(--shadow-lg); } .sidebar.open { transform: translateX(0); } }
</style>
