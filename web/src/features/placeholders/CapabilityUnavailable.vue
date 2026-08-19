<script setup lang="ts">
defineProps<{
  title: string;
  description: string;
}>();
</script>

<template>
  <section class="capability-unavailable">
    <header class="unavailable-header">
      <h1 class="unavailable-title">{{ title }}</h1>
      <span class="unavailable-badge">尚未接入</span>
    </header>
    <p class="unavailable-description">{{ description }}</p>
  </section>
</template>

<style scoped>
.capability-unavailable {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.unavailable-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.unavailable-title {
  margin: 0;
  font-size: 20px;
  color: var(--text);
}

.unavailable-badge {
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--surface-tint);
  color: var(--primary);
  font-size: 12px;
  font-weight: 600;
}

.unavailable-description {
  margin: 0;
  color: var(--text-soft);
  font-size: 14px;
}
</style>
