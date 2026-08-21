<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { isApiError } from "./api";
import ConflictDialog from "./ConflictDialog.vue";
import ProfileDeleteDialog from "./ProfileDeleteDialog.vue";
import ProfileDialog from "./ProfileDialog.vue";
import SaveStatus from "./SaveStatus.vue";
import { useProfileStore } from "./store";

const store = useProfileStore();
const dialog = ref<"create" | "rename" | null>(null);
const deletingId = ref<string | null>(null);
const requiresCoreDecision = ref(false);
const busy = ref(false);
const error = ref("");
const deleting = computed(() => store.profiles.value.find((item) => item.id === deletingId.value) ?? null);
const activeProfile = computed(() => store.profiles.value.find((item) => item.id === store.activeProfileId.value) ?? null);

onMounted(async () => {
  try {
    await store.refresh();
    if (store.profiles.value[0] && !store.activeProfileId.value) await store.open(store.profiles.value[0].id);
  } catch { error.value = "无法加载档案列表"; }
});

async function select(id: string): Promise<void> {
  error.value = "";
  if (!await store.open(id)) error.value = store.hasPendingChanges.value ? "请先处理未保存的本地修改，再切换档案。" : "该草稿已隔离，不能打开编辑。";
}
async function submitName(name: string): Promise<void> {
  busy.value = true; error.value = "";
  try { if (dialog.value === "create") await store.create(name); else if (store.activeProfileId.value) await store.rename(store.activeProfileId.value, name); dialog.value = null; } catch (cause) { error.value = cause instanceof Error ? cause.message : "操作失败"; } finally { busy.value = false; }
}
async function deleteProfile(stopCurrent: boolean): Promise<void> {
  if (!deleting.value) return;
  busy.value = true; error.value = "";
  try { await store.remove(deleting.value.id, deleting.value.name, stopCurrent); deletingId.value = null; requiresCoreDecision.value = false; } catch (cause) { if (isApiError(cause) && cause.status === 409 && cause.body.code === "current_profile_requires_core_decision") { requiresCoreDecision.value = true; } else error.value = cause instanceof Error ? cause.message : "删除失败"; } finally { busy.value = false; }
}
</script>

<template>
  <section class="profiles-view">
    <header><div><h1>配置档案</h1><SaveStatus :state="store.saveState.value" :needs-apply="activeProfile?.needsApply" /></div><button @click="dialog = 'create'">新建档案</button></header>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div class="layout"><aside class="profile-list" aria-label="档案列表"><button v-for="item in store.profiles.value" :key="item.id" class="profile" :class="{ active: item.id === store.activeProfileId.value, quarantined: item.quarantinedError }" :disabled="Boolean(item.quarantinedError)" @click="select(item.id)"><strong>{{ item.name }}</strong><small v-if="item.quarantinedError">草稿已隔离</small><small v-else-if="item.needsApply">需要应用</small></button></aside>
      <article class="editor"><template v-if="store.draft.value"><div class="editor-title"><h2>{{ activeProfile?.name }}</h2><div><button class="secondary" @click="dialog = 'rename'">重命名</button><button class="danger" @click="deletingId = store.activeProfileId.value">删除</button></div></div><ConflictDialog :open="store.saveState.value === 'conflict'" :latest-revision="store.conflict.value?.latestRevision" :busy="busy" @load="store.loadServerVersion" @overwrite="store.overwriteServerVersion" /><p class="hint">草稿已在浏览器内自动保存；关闭页面前会提示未确认的修改。</p></template><p v-else>选择一个可用档案开始编辑。</p></article>
    </div>
    <ProfileDialog :open="dialog !== null" :title="dialog === 'create' ? '新建档案' : '重命名档案'" :initial-name="dialog === 'rename' ? activeProfile?.name : ''" :busy="busy" @close="dialog = null" @submit="submitName" />
    <ProfileDeleteDialog :open="deleting !== null" :name="deleting?.name ?? ''" :requires-core-decision="requiresCoreDecision" :busy="busy" @close="deletingId = null; requiresCoreDecision = false" @submit="deleteProfile" />
  </section>
</template>

<style scoped>
.profiles-view { max-width: 1100px; } header, .editor-title { display: flex; align-items: center; justify-content: space-between; gap: 16px; } h1, h2 { margin: 0; } h1 { font-size: 24px; } .layout { display: grid; grid-template-columns: minmax(190px, 250px) 1fr; gap: 18px; margin-top: 22px; }.profile-list, .editor { padding: 12px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }.profile-list { display: grid; align-content: start; gap: 6px; }.profile { display: grid; min-height: 60px; padding: 10px; border-radius: var(--radius-sm); color: var(--text); text-align: left; background: transparent; }.profile.active { background: var(--primary-soft); color: var(--primary-strong); }.profile.quarantined { opacity: .75; }.profile small { color: var(--orange); }.editor { min-height: 250px; }.editor-title div { display: flex; gap: 8px; }.secondary { color: var(--text); background: var(--surface-soft); }.danger { background: var(--red); }.hint { color: var(--text-soft); }.error { padding: 10px 12px; border-radius: var(--radius-sm); color: var(--red); background: var(--red-soft); } @media (max-width: 760px) { .layout { grid-template-columns: 1fr; } }
</style>
