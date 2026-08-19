<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../../stores/auth";
import { useCapabilitiesStore } from "../../stores/capabilities";
import { useProfilesStore } from "../../stores/profiles";
import { useDraftAutosave } from "../config/useDraftAutosave";
import ProfileSidebar from "../profiles/ProfileSidebar.vue";

const auth = useAuthStore();
const capabilities = useCapabilitiesStore();
const profiles = useProfilesStore();
const router = useRouter();

useDraftAutosave();

const sidebarOpen = ref(false);
const sidebarCollapsed = ref(false);

onMounted(async () => {
  if (profiles.items.length === 0) {
    await profiles.load().catch(() => undefined);
  }
});

const navGroups = [
  {
    label: "概览",
    items: [
      { to: "/dashboard", label: "仪表盘" },
      { to: "/config", label: "配置" },
      { to: "/runtime", label: "运行状态" },
    ],
  },
  {
    label: "资源",
    items: [
      { to: "/subscriptions", label: "订阅" },
      { to: "/rulesets", label: "规则集" },
    ],
  },
  {
    label: "系统",
    items: [{ to: "/settings", label: "设置" }],
  },
];

async function logout(): Promise<void> {
  await auth.logout();
  await router.replace("/login");
}

async function ensureCapabilities(): Promise<void> {
  if (auth.session && capabilities.snapshot === null) {
    await capabilities.load();
  }
}
void ensureCapabilities();

function closeSidebar(): void {
  sidebarOpen.value = false;
}
</script>

<template>
  <div
    class="app-shell"
    :class="{ 'sidebar-open-state': sidebarOpen, 'sidebar-collapsed-state': sidebarCollapsed }"
  >
    <aside class="sidebar" aria-label="主导航">
      <div class="brand">
        <div class="brand-mark"><span>S</span></div>
        <div class="brand-copy">
          <div class="brand-title">SingBox Control</div>
          <div class="brand-subtitle">Web panel</div>
        </div>
      </div>
      <nav class="sidebar-scroll" aria-label="主导航">
        <template v-for="group in navGroups" :key="group.label">
          <div class="nav-label">{{ group.label }}</div>
          <RouterLink
            v-for="item in group.items"
            :key="item.to"
            :to="item.to"
            class="nav-item"
            @click="closeSidebar"
          >
            <span>{{ item.label }}</span>
          </RouterLink>
        </template>
      </nav>
      <ProfileSidebar />
      <div class="sidebar-footer">
        <button class="nav-item logout-item" name="退出登录" @click="logout">
          <span>退出登录</span>
        </button>
      </div>
    </aside>
    <button
      class="sidebar-overlay"
      aria-label="关闭菜单"
      @click="closeSidebar"
    ></button>
    <main class="main-shell">
      <header class="topbar">
        <button
          class="mobile-menu"
          aria-label="打开菜单"
          @click="sidebarOpen = !sidebarOpen"
        >☰</button>
        <div class="page-heading">
          <h1>sing-box 控制台</h1>
        </div>
        <div class="topbar-actions">
          <div class="avatar" title="管理员">AD</div>
        </div>
      </header>
      <div class="page-content">
        <RouterView />
      </div>
    </main>
  </div>
</template>

<style scoped>
.logout-item {
  border: 0;
  cursor: pointer;
  font: inherit;
}
</style>
