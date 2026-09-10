<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import AppIcon from "@/layout/AppIcon.vue";
import AppearanceSettings from "./AppearanceSettings.vue";
import CoreSettings from "./CoreSettings.vue";
import PasswordSettings from "./PasswordSettings.vue";
import ProxySettings from "./ProxySettings.vue";

const sectionIds = new Set(["core", "appearance", "proxy", "security"]);
const activeSection = ref("core");

function syncActiveSection(): void {
  const id = window.location.hash.replace(/^#/, "");
  activeSection.value = sectionIds.has(id) ? id : "core";
}

onMounted(() => {
  syncActiveSection();
  window.addEventListener("hashchange", syncActiveSection);
});
onBeforeUnmount(() => window.removeEventListener("hashchange", syncActiveSection));
</script>

<template>
  <section class="settings-page">
    <header class="page-intro">
      <h1>系统设置</h1>
      <p>管理核心行为、控制台偏好与安全访问方式。</p>
    </header>
    <div class="settings-layout">
      <nav class="settings-nav" aria-label="设置分类">
        <a :class="{ active: activeSection === 'core' }" href="#core" @click="activeSection = 'core'"><AppIcon name="cpu" :size="15" />核心设置</a>
        <a :class="{ active: activeSection === 'appearance' }" href="#appearance" @click="activeSection = 'appearance'"><AppIcon name="palette" :size="15" />界面外观</a>
        <a :class="{ active: activeSection === 'proxy' }" href="#proxy" @click="activeSection = 'proxy'"><AppIcon name="globe" :size="15" />代理设置</a>
        <a :class="{ active: activeSection === 'security' }" href="#security" @click="activeSection = 'security'"><AppIcon name="lock" :size="15" />安全访问</a>
      </nav>
      <div class="settings-content">
        <section id="core"><CoreSettings /></section>
        <section id="appearance"><AppearanceSettings /></section>
        <section id="proxy"><ProxySettings /></section>
        <section id="security"><PasswordSettings /></section>
      </div>
    </div>
  </section>
</template>

<style scoped>
.settings-page { display: grid; gap: var(--section-gap); min-width: 0; }
.page-intro h1 { margin: 0; font-size: 22px; letter-spacing: -.4px; }
.page-intro p { margin: 5px 0 0; color: var(--text-soft); font-size: var(--font-size-base); }
.settings-layout { display: grid; grid-template-columns: 160px minmax(0, 1fr); gap: var(--section-gap); }
.settings-nav { position: sticky; top: calc(var(--topbar-height) + var(--section-gap)); display: grid; align-content: start; height: fit-content; gap: 3px; padding: 8px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }
.settings-nav a { display: flex; align-items: center; gap: 9px; padding: 9px 10px; border-radius: 8px; color: var(--text-soft); font-size: var(--font-size-sm); font-weight: 650; text-decoration: none; }
.settings-nav a:hover { color: var(--primary-strong); background: var(--primary-soft); }
.settings-nav a.active { color: var(--primary-strong); background: var(--primary-soft); font-weight: 700; }
.settings-content { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; gap: var(--section-gap); }
.settings-content section { min-width: 0; scroll-margin-top: calc(var(--topbar-height) + var(--section-gap)); }
@media (max-width: 1280px) {
  .settings-content { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 760px) {
  .settings-layout { grid-template-columns: 1fr; }
  .settings-nav { position: static; display: flex; overflow-x: auto; scrollbar-width: none; }
  .settings-nav a { min-width: max-content; flex: 0 0 auto; }
}
</style>
