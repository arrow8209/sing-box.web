<script setup lang="ts">
import { onMounted, ref } from "vue";
import { RouterView } from "vue-router";
import AppSidebar from "./AppSidebar.vue";
import AppTopbar from "./AppTopbar.vue";
import { useCapabilitiesStore } from "@/features/capabilities/store";

const menuOpen = ref(false);
const collapsed = ref(false);
const capabilities = useCapabilitiesStore();

function applyTheme(theme: string | null): void {
  document.documentElement.dataset.theme = theme === "dark" ? "dark" : "";
}

function toggleTheme(): void {
  const next = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
  localStorage.setItem("singbox-theme", next);
  applyTheme(next);
}

function toggleCollapsed(): void {
  collapsed.value = !collapsed.value;
  localStorage.setItem("singbox-sidebar-collapsed", String(collapsed.value));
}

onMounted(() => {
	void capabilities.refresh().catch(() => undefined);
  collapsed.value = localStorage.getItem("singbox-sidebar-collapsed") === "true";
  applyTheme(localStorage.getItem("singbox-theme"));
});
</script>

<template>
  <div class="app-shell" :class="{ 'app-shell--collapsed': collapsed }">
    <AppSidebar :open="menuOpen" :collapsed="collapsed" @close="menuOpen = false" />
    <button v-if="menuOpen" class="sidebar-overlay" type="button" aria-label="关闭菜单" @click="menuOpen = false" />
    <section class="app-main">
      <AppTopbar :collapsed="collapsed" @toggle-menu="menuOpen = !menuOpen" @toggle-collapse="toggleCollapsed" @toggle-theme="toggleTheme" />
      <main class="app-content"><RouterView /></main>
    </section>
  </div>
</template>

<style scoped>
.app-shell { min-height: 100vh; }
.app-main { min-height: 100vh; margin-inline-start: var(--sidebar-width); transition: margin-inline-start 180ms ease; }
.app-shell--collapsed .app-main { margin-inline-start: 72px; }
.sidebar-overlay { position: fixed; z-index: 35; inset: 0; display: none; border: 0; border-radius: 0; background: rgb(8 14 27 / 42%); backdrop-filter: blur(2px); }
@media (max-width: 1100px) { .app-main, .app-shell--collapsed .app-main { margin-inline-start: 0; } .sidebar-overlay { display: block; } }
</style>
