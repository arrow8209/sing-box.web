import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "./stores/auth";
import { useCapabilitiesStore } from "./stores/capabilities";
import { useProfilesStore } from "./stores/profiles";
import LoginView from "./features/auth/LoginView.vue";
import AppShell from "./features/shell/AppShell.vue";
import CapabilityUnavailable from "./features/placeholders/CapabilityUnavailable.vue";

export const routes = [
  { path: "/login", name: "login", component: LoginView, meta: { public: true } },
  {
    path: "/",
    component: AppShell,
    children: [
      { path: "", redirect: "/dashboard" },
      {
        path: "dashboard",
        name: "dashboard",
        component: CapabilityUnavailable,
        props: { title: "配置仪表盘", description: "配置摘要将在档案功能完成后显示。" },
      },
      {
        path: "config/:profileId?/:module?",
        name: "config",
        component: CapabilityUnavailable,
        props: { title: "配置工作区", description: "请先创建或选择配置档案。" },
        beforeEnter: async (to: import("vue-router").RouteLocationNormalized) => {
          const profiles = useProfilesStore();
          if (profiles.items.length === 0) {
            await profiles.load().catch(() => undefined);
          }
          const profileId = to.params.profileId;
          if (typeof profileId === "string" && profiles.activeId !== profileId) {
            await profiles.select(profileId).catch(() => undefined);
          }
          return true;
        },
      },
      {
        path: "runtime",
        name: "runtime",
        component: CapabilityUnavailable,
        props: { title: "运行状态", description: "核心状态与生命周期控制将在二期接入。" },
      },
      {
        path: "subscriptions",
        name: "subscriptions",
        component: CapabilityUnavailable,
        props: { title: "订阅配置", description: "代理订阅同步不在一期范围内。" },
      },
      {
        path: "rulesets",
        name: "rulesets",
        component: CapabilityUnavailable,
        props: { title: "规则集中心", description: "规则资源下载与调度不在一期范围内。" },
      },
      {
        path: "settings",
        name: "settings",
        component: CapabilityUnavailable,
        props: { title: "系统设置", description: "管理员与界面设置将在后续前端任务接入。" },
      },
    ],
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

let restored = false;

router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (!restored) {
    restored = true;
    try {
      await auth.restore();
    } catch {
      // Network failure during restore: stay on the requested page for now.
    }
  }
  if (auth.session === null && to.meta.public !== true) {
    return { path: "/login" };
  }
  if (auth.session !== null && to.name === "login") {
    return { path: "/dashboard" };
  }
  if (auth.session !== null && to.meta.public !== true) {
    const capabilities = useCapabilitiesStore();
    if (capabilities.snapshot === null) {
      await capabilities.load().catch(() => undefined);
    }
  }
  return true;
});
