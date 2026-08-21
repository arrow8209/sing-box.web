<script setup lang="ts">
import { onMounted, ref, watch } from "vue";

type Theme = "light" | "dark";
const allowedThemes: Theme[] = ["light", "dark"];
const allowedAccents = ["blue", "violet", "green"];
const theme = ref<Theme>("light");
const accent = ref("blue");
const collapsed = ref(false);
function read(key: "singbox-theme" | "singbox-accent" | "singbox-sidebar-collapsed"): string | null { return localStorage.getItem(key); }
function apply(): void { document.documentElement.dataset.theme = theme.value === "dark" ? "dark" : ""; document.documentElement.dataset.accent = accent.value; }
onMounted(() => {
  const savedTheme = read("singbox-theme"); const savedAccent = read("singbox-accent"); const savedCollapsed = read("singbox-sidebar-collapsed");
  if (savedTheme && allowedThemes.includes(savedTheme as Theme)) theme.value = savedTheme as Theme;
  if (savedAccent && allowedAccents.includes(savedAccent)) accent.value = savedAccent;
  collapsed.value = savedCollapsed === "true"; apply();
});
watch(theme, () => { localStorage.setItem("singbox-theme", theme.value); apply(); });
watch(accent, () => { localStorage.setItem("singbox-accent", accent.value); apply(); });
watch(collapsed, () => localStorage.setItem("singbox-sidebar-collapsed", String(collapsed.value)));
</script>

<template>
  <section class="panel"><h2>外观与布局</h2><label>主题<select v-model="theme"><option value="light">浅色</option><option value="dark">深色</option></select></label><label>强调色<select v-model="accent"><option value="blue">蓝色</option><option value="violet">紫色</option><option value="green">绿色</option></select></label><label class="check"><input v-model="collapsed" type="checkbox"> 收起侧栏</label></section>
</template>

<style scoped>
.panel { display: grid; gap: 14px; padding: 20px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); } h2 { margin: 0; font-size: 17px; } label { display: grid; gap: 6px; max-width: 360px; } .check { display: block; }
</style>
