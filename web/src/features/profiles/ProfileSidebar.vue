<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import ConflictDialog from "./ConflictDialog.vue";
import ProfileDialog from "./ProfileDialog.vue";
import SaveStatus from "./SaveStatus.vue";
import { useProfileStore } from "./store";
import AppIcon from "@/layout/AppIcon.vue";

const store = useProfileStore();
const route = useRoute();
const router = useRouter();
const createOpen = ref(false);
const busy = ref(false);
const error = ref("");
const activeProfile = computed(() => store.profiles.value.find((item) => item.id === store.activeProfileId.value) ?? null);

function openCreate(): void { createOpen.value = true; }
function closeCreate(): void {
  createOpen.value = false;
  if (route.query.create !== "1") return;
  const query = { ...route.query };
  delete query.create;
  void router.replace({ query });
}
watch(() => route.query.create, (value) => { if (value === "1") openCreate(); }, { immediate: true });

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
  try { await store.create(name); closeCreate(); } catch (cause) { error.value = cause instanceof Error ? cause.message : "操作失败"; } finally { busy.value = false; }
}
</script>

<template>
  <section class="profiles-view">
    <header class="page-intro"><div><h1>配置档案</h1><p>选择档案后进入配置工作台；草稿由服务端自动保存。</p><SaveStatus :state="store.saveState.value" :needs-apply="activeProfile?.needsApply" /></div><button @click="openCreate"><AppIcon name="plus" :size="16" />新建档案</button></header>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div class="layout"><aside class="profile-list" aria-label="档案列表"><div class="list-head"><span>配置树</span><small>{{ store.profiles.value.length }} 个档案</small></div><button v-for="item in store.profiles.value" :key="item.id" class="profile" :class="{ active: item.id === store.activeProfileId.value, quarantined: item.quarantinedError }" :disabled="Boolean(item.quarantinedError)" @click="select(item.id)"><span class="profile-icon"><AppIcon name="config" :size="16" /></span><span><strong>{{ item.name }}</strong><small v-if="item.quarantinedError">草稿已隔离</small><small v-else-if="item.needsApply">需要应用</small><small v-else>草稿已保存</small></span><i v-if="item.needsApply" class="dirty-dot" /></button></aside>
      <article class="editor"><template v-if="store.draft.value"><div class="editor-title"><div><span class="editor-icon"><AppIcon name="config" /></span><div><h2>{{ activeProfile?.name }}</h2><p>草稿修订 {{ store.draft.value.revision }} · 编辑后会自动保存</p></div></div></div><ConflictDialog :open="store.saveState.value === 'conflict'" :latest-revision="store.conflict.value?.latestRevision" :busy="busy" @load="store.loadServerVersion" @overwrite="store.overwriteServerVersion" /><p class="hint">档案详细内容及应用、复制、重命名、删除操作位于配置工作台。</p><a class="open-workspace" href="/workspace">打开配置工作台 <AppIcon name="chevron" :size="15" /></a></template><p v-else>选择一个可用档案开始编辑。</p></article>
    </div>
    <ProfileDialog :open="createOpen" title="新建档案" :busy="busy" @close="closeCreate" @submit="submitName" />
  </section>
</template>

<style scoped>
.profiles-view { max-width: 1120px; }.page-intro, .editor-title { display: flex; align-items: end; justify-content: space-between; gap: 16px; }.page-intro h1, h2 { margin: 0; }.page-intro h1 { font-size: 22px; letter-spacing: -.4px; }.page-intro p { margin: 5px 0 7px; color: var(--text-soft); font-size: 12px; }.page-intro > button { display: inline-flex; min-height: 35px; align-items: center; gap: 7px; padding: 7px 12px; border-radius: 8px; font-size: 11px; }.layout { display: grid; grid-template-columns: minmax(220px, 270px) 1fr; gap: 12px; margin-top: 16px; }.profile-list, .editor { border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); }.profile-list { display: grid; align-content: start; gap: 3px; padding: 9px; }.list-head { display: flex; align-items: center; justify-content: space-between; padding: 5px 7px 8px; color: var(--text-faint); font-size: 10px; font-weight: 720; }.list-head small { font-size: 9px; font-weight: 500; }.profile { display: flex; min-height: 54px; align-items: center; gap: 9px; padding: 8px; border-radius: 9px; color: var(--text); text-align: left; background: transparent; }.profile:hover { background: var(--surface-soft); }.profile.active { color: var(--primary-strong); background: var(--primary-soft); }.profile.quarantined { opacity: .75; }.profile-icon, .editor-icon { display: grid; width: 31px; height: 31px; flex: 0 0 auto; place-items: center; border-radius: 9px; color: var(--primary); background: var(--primary-soft); }.profile > span:nth-child(2) { min-width: 0; flex: 1; }.profile strong, .profile small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.profile strong { font-size: 11px; }.profile small { margin-top: 1px; color: var(--text-faint); font-size: 9px; }.profile small:not(:empty) { color: var(--orange); }.dirty-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--orange); }.editor { min-height: 245px; padding: 16px; }.editor-title { align-items: start; padding-bottom: 14px; border-bottom: 1px solid var(--line); }.editor-title > div:first-child { display: flex; min-width: 0; align-items: center; gap: 10px; }.editor-title h2 { overflow: hidden; font-size: 16px; text-overflow: ellipsis; white-space: nowrap; }.editor-title p { margin: 3px 0 0; color: var(--text-faint); font-size: 10px; }.hint { margin: 15px 0 0; color: var(--text-soft); font-size: 11px; }.open-workspace { display: inline-flex; align-items: center; gap: 4px; margin-top: 16px; padding: 8px 10px; border-radius: 8px; color: var(--primary-strong); background: var(--primary-soft); font-size: 11px; font-weight: 650; text-decoration: none; }.error { padding: 10px 12px; border-radius: var(--radius-sm); color: var(--red); background: var(--red-soft); } @media (max-width: 760px) { .page-intro { display: block; }.page-intro > button { width: 100%; justify-content: center; margin-top: 10px; }.layout { grid-template-columns: 1fr; }.editor-title { display: block; } }
</style>
