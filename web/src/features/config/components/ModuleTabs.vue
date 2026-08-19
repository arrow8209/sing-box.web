<script setup lang="ts">
import { useRoute, useRouter } from "vue-router";
import { modules } from "../modules";

defineProps<{ profileId: string }>();

const route = useRoute();
const router = useRouter();

function activeKey(): string {
  const module = route.params.module;
  if (typeof module === "string") {
    return module;
  }
  return "general";
}

async function select(module: string): Promise<void> {
  await router.push(`/config/${route.params.profileId as string}/${module}`);
}
</script>

<template>
  <div class="module-tabs" role="tablist" aria-label="配置模块">
    <button
      v-for="descriptor in modules"
      :key="descriptor.key"
      class="module-tab"
      :class="{ active: descriptor.key === activeKey() }"
      role="tab"
      :aria-selected="descriptor.key === activeKey() ? 'true' : 'false'"
      @click="select(descriptor.key)"
    >
      {{ descriptor.shortLabel }}
    </button>
  </div>
</template>

<style scoped>
.module-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 10px 0;
  border-bottom: 1px solid var(--line);
}

.module-tab {
  padding: 7px 14px;
  border: 1px solid var(--line);
  border-radius: 999px;
  background: var(--surface);
  color: var(--text-soft);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.module-tab.active {
  border-color: var(--primary);
  background: var(--primary-soft);
  color: var(--primary);
  font-weight: 600;
}
</style>
