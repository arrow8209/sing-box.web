<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/features/auth/store";

const auth = useAuthStore(); const router = useRouter();
const currentPassword = ref(""); const newPassword = ref(""); const confirmation = ref(""); const error = ref(""); const submitting = ref(false);
async function submit(): Promise<void> {
  error.value = "";
  if (newPassword.value !== confirmation.value) { error.value = "两次输入的新密码不一致"; return; }
  submitting.value = true;
  try { await auth.changePassword(currentPassword.value, newPassword.value); await router.replace({ name: "login" }); }
  catch (cause) { error.value = cause instanceof Error ? cause.message : "修改密码失败"; }
  finally { submitting.value = false; }
}
</script>

<template>
  <section class="panel"><h2>修改密码</h2><p>修改成功后将退出当前会话，需要使用新密码重新登录。</p><form @submit.prevent="submit"><label>当前密码<input v-model="currentPassword" autocomplete="current-password" required type="password"></label><label>新密码<input v-model="newPassword" autocomplete="new-password" minlength="6" required type="password"></label><label>确认新密码<input v-model="confirmation" autocomplete="new-password" minlength="6" required type="password"></label><p v-if="error" class="error" role="alert">{{ error }}</p><button :disabled="submitting" type="submit">{{ submitting ? "正在修改" : "修改密码" }}</button></form></section>
</template>

<style scoped>
.panel { padding: var(--card-padding); border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-sm); } h2 { margin: 0; font-size: 15px; } p { color: var(--text-soft); font-size: var(--font-size-sm); } form { display: grid; gap: var(--section-gap); min-width: 0; } label { display: grid; gap: 6px; color: var(--text-soft); font-size: var(--font-size-base); font-weight: 650; } input { min-width: 0; width: 100%; min-height: 38px; font-size: var(--font-size-base); } button { width: fit-content; min-height: 35px; padding: 7px 12px; border-radius: 8px; font-size: var(--font-size-sm); } .error { color: var(--red); }
</style>
