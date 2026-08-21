<script setup lang="ts">
import { ref } from "vue";
import { RouterView } from "vue-router";
import AppSidebar from "./AppSidebar.vue";
import AppTopbar from "./AppTopbar.vue";

const menuOpen = ref(false);
</script>

<template>
  <AppSidebar :open="menuOpen" @close="menuOpen = false" />
  <section class="shell">
    <AppTopbar @toggle-menu="menuOpen = !menuOpen" />
    <main class="content"><RouterView /></main>
  </section>
</template>

<style scoped>
.shell { min-height: 100vh; margin-left: var(--sidebar-width); }
.content { padding: 28px; }
@media (max-width: 760px) { .shell { margin-left: 0; } .content { padding: 20px 16px; } }
</style>
