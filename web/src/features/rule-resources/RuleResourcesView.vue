<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ApiError } from "@/api/client";
import { useRouter } from "vue-router";
import RuleResourceDialog from "./RuleResourceDialog.vue";
import RuleResourceTable from "./RuleResourceTable.vue";
import { useRuleResourceStore } from "./store";
import type { ProfileReference, RuleResource, RuleResourceInput } from "./types";
const store = useRuleResourceStore(); const router = useRouter();
const dialogOpen = ref(false); const editing = ref<RuleResource | null>(null); const notice = ref(""); const error = ref(""); const references = ref<ProfileReference[]>([]);
onMounted(() => { void store.refresh().catch((reason: unknown) => { error.value = reason instanceof Error ? reason.message : "加载规则资源失败"; }); });
function openCreate(): void { editing.value = null; dialogOpen.value = true; }
function openEdit(resource: RuleResource): void { editing.value = resource; dialogOpen.value = true; }
async function submit(input: RuleResourceInput): Promise<void> { error.value = ""; try { if (editing.value) await store.patch(editing.value.id, input); else await store.create(input); dialogOpen.value = false; } catch (reason) { error.value = reason instanceof Error ? reason.message : "保存失败"; } }
async function update(resource: RuleResource): Promise<void> { error.value = ""; notice.value = ""; try { const result = await store.update(resource.id); notice.value = result.changed ? (result.message || "规则资源已更新，请点击“应用”使 sing-box 使用新规则。") : (result.message || "规则资源内容未变化。"); } catch (reason) { error.value = reason instanceof Error ? reason.message : "更新失败"; } }
async function remove(resource: RuleResource): Promise<void> { error.value = ""; references.value = []; try { await store.remove(resource.id); } catch (reason) { if (reason instanceof ApiError && reason.status === 409 && reason.body.code === "rule_resource_referenced") { const details = reason.body.details?.profiles; references.value = Array.isArray(details) ? details as ProfileReference[] : []; error.value = reason.message; return; } error.value = reason instanceof Error ? reason.message : "删除失败"; } }
function openProfile(reference: ProfileReference): void { void router.push({ name: "profiles", query: { open: reference.id } }); }
</script>
<template>
  <section class="rules"><header class="page-intro"><div><h1>规则资源</h1><p>下载与周期调度由服务端持续执行，浏览器关闭不会影响调度。</p></div><button type="button" data-create-rule @click="openCreate">新增规则资源</button></header><p v-if="notice" class="notice" role="status">{{ notice }}</p><div v-if="error" class="error" role="alert"><p>{{ error }}</p><p v-if="references.length">仍在引用：<button v-for="reference in references" :key="reference.id" type="button" class="link" @click="openProfile(reference)">{{ reference.name }}</button></p></div><RuleResourceTable :resources="store.resources.value" :updating-id="store.updatingId.value" @edit="openEdit" @update="update" @remove="remove" /><RuleResourceDialog :open="dialogOpen" :resource="editing" @close="dialogOpen = false" @submit="submit" /></section>
</template>
<style scoped>
.rules { max-width: 1250px; }.page-intro { display: flex; align-items: end; justify-content: space-between; gap: 16px; margin-bottom: 14px; }.rules h1, .rules p { margin: 0; }.rules h1 { font-size: 22px; letter-spacing: -.4px; }.rules header p { margin-top: 5px; color: var(--text-soft); font-size: 12px; }.rules header button { min-height: 35px; padding: 7px 12px; border-radius: 8px; font-size: 11px; }.notice, .error { margin: 0 0 14px; padding: 10px 12px; border-radius: 9px; font-size: 11px; }.notice { color: var(--primary-strong); background: var(--primary-soft); }.error { color: var(--red); background: var(--red-soft); }.error p + p { margin-top: 6px; }.link { margin-left: 6px; padding: 0; color: inherit; text-decoration: underline; background: none; }
@media (max-width: 640px) { .page-intro { display: block; }.rules header button { width: 100%; margin-top: 12px; } }
</style>
