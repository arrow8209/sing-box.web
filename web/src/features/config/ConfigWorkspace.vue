<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute } from "vue-router";
import { useDraftStore } from "../../stores/draft";
import { modules, topLevelKey } from "./modules";
import ModuleTabs from "./components/ModuleTabs.vue";
import RawModuleEditor from "./components/RawModuleEditor.vue";
import PublicationBar from "../publication/PublicationBar.vue";
import GeneralEditor from "./editors/GeneralEditor.vue";
import DNSEditor from "./editors/DNSEditor.vue";
import AdvancedEditor from "./editors/AdvancedEditor.vue";
import InboundsEditor from "./editors/InboundsEditor.vue";
import OutboundsEditor from "./editors/OutboundsEditor.vue";
import RouteEditor from "./editors/RouteEditor.vue";
import CollectionJSONEditor from "./editors/CollectionJSONEditor.vue";

const route = useRoute();
const draft = useDraftStore();

const rawMode = ref(false);

const profileId = computed(() => {
  const value = route.params.profileId;
  return typeof value === "string" ? value : "";
});

const activeModule = computed(() => {
  const value = route.params.module;
  if (typeof value === "string" && modules.some((descriptor) => descriptor.key === value)) {
    return value;
  }
  return "general";
});

const activeDescriptor = computed(
  () => modules.find((descriptor) => descriptor.key === activeModule.value) ?? modules[0],
);

const draftError = computed(() => {
  if (draft.status === "error") {
    return draft.lastError?.message ?? "草稿不可编辑";
  }
  return "";
});

const structuredAvailable = computed(() => {
  if (draft.status === "error" || draft.draft === null) {
    return false;
  }
  const topKey = topLevelKey(activeModule.value);
  const buffer = draft.draft.content.editorBuffers[topKey];
  return buffer === undefined || (buffer.valid && buffer.applied);
});

function rawEditorAccessibleName(): string {
  return `${activeDescriptor.value.shortLabel} JSON`;
}

const collectionRows = computed(() => {
  const topKey = topLevelKey(activeModule.value);
  const value = draft.draft?.content.config[topKey];
  return Array.isArray(value) ? value : [];
});

function replaceCollection(rows: unknown[]): void {
  // 集合编辑器已在 draft store 内提交 canonical 变更，这里只保留视图同步。
  void rows;
}
</script>

<template>
  <section class="config-workspace">
    <header class="workspace-header">
      <h2 class="workspace-title">{{ activeDescriptor.label }}</h2>
      <button
        class="btn"
        :name="`编辑${activeDescriptor.shortLabel} JSON`"
        type="button"
        @click="rawMode = !rawMode"
      >
        {{ rawMode ? "返回表单" : `编辑${activeDescriptor.shortLabel} JSON` }}
      </button>
    </header>

    <ModuleTabs :profile-id="profileId" />

    <PublicationBar v-if="draft.status !== 'error'" />

    <p v-if="draftError" class="workspace-error" role="alert">{{ draftError }}</p>

    <template v-if="draft.status === 'error'">
      <p class="workspace-note">草稿只读，编辑已禁用。</p>
    </template>
    <template v-else>
      <section v-if="rawMode" :aria-label="rawEditorAccessibleName()">
        <RawModuleEditor :module="activeModule" />
      </section>
      <section v-else-if="structuredAvailable" class="structured-pane">
        <GeneralEditor v-if="activeModule === 'general'" />
        <DNSEditor v-else-if="activeModule === 'dns'" />
        <InboundsEditor v-else-if="activeModule === 'inbounds'" />
        <OutboundsEditor v-else-if="activeModule === 'outbounds'" />
        <RouteEditor v-else-if="activeModule === 'routing'" />
        <AdvancedEditor v-else-if="activeModule === 'advanced'" />
        <CollectionJSONEditor
          v-else-if="activeModule === 'endpoints' || activeModule === 'services'"
          :module="topLevelKey(activeModule)"
          :model-value="collectionRows"
          @update:model-value="replaceCollection"
        />
        <p v-else class="workspace-note">
          {{ activeDescriptor.label }} 模块可使用右上角的 JSON 编辑。
        </p>
      </section>
      <p v-else class="workspace-note">
        该模块存在未应用的 JSON 缓冲，请先在 JSON 编辑器中处理或放弃未应用内容。
      </p>
    </template>
  </section>
</template>

<style scoped>
.config-workspace {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.workspace-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.workspace-title {
  margin: 0;
  font-size: 18px;
  color: var(--text);
}

.workspace-error {
  margin: 0;
  padding: 10px 12px;
  border: 1px solid var(--orange);
  border-radius: var(--radius-sm);
  background: var(--orange-soft);
  color: var(--orange);
  font-size: 13px;
}

.workspace-note {
  margin: 0;
  color: var(--text-soft);
  font-size: 14px;
}

.structured-pane {
  min-height: 200px;
}
</style>
