import { defineComponent, h } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import LoginView from "./features/auth/LoginView.vue";
import { useAuthStore } from "./features/auth/store";
import AppShell from "./layout/AppShell.vue";
import ProfileSidebar from "./features/profiles/ProfileSidebar.vue";
import ConfigWorkspace from "./features/config-editor/ConfigWorkspace.vue";
import RuleResourcesView from "./features/rule-resources/RuleResourcesView.vue";
import SubscriptionsView from "./features/subscriptions/SubscriptionsView.vue";
import InboundResourcesView from "./features/inbounds/InboundResourcesView.vue";
import CoreView from "./features/core/CoreView.vue";
import DashboardView from "./features/dashboard/DashboardView.vue";
import SettingsView from "./features/settings/SettingsView.vue";
import RuntimeUnavailableView from "./features/runtime/RuntimeUnavailableView.vue";

const PlaceholderView = defineComponent({
  setup() {
    return () => h("section", { class: "placeholder-view" }, [
      h("h1", "管理面板"),
      h("p", "此页面将在后续功能阶段接入真实数据和操作。")
    ]);
  }
});

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: LoginView, meta: { public: true } },
    {
      path: "/",
      component: AppShell,
      children: [
        { path: "", name: "overview", component: DashboardView },
        { path: "workspace", name: "workspace", component: ConfigWorkspace },
        { path: "profiles", name: "profiles", component: ProfileSidebar },
        { path: "inbounds", name: "inbounds", component: InboundResourcesView },
        { path: "subscriptions", name: "subscriptions", component: SubscriptionsView },
        { path: "rules", name: "rules", component: RuleResourcesView },
        { path: "core", name: "core", component: CoreView },
        { path: "runtime", name: "runtime", component: RuntimeUnavailableView },
        { path: "settings", name: "settings", component: SettingsView }
      ]
    }
  ]
});

let initialized = false;
router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (!initialized) {
    initialized = true;
    await auth.initialize();
  }
  if (!to.meta.public && auth.state.status !== "authenticated") {
    return { name: "login", query: { redirect: to.fullPath } };
  }
  if (to.meta.public && auth.state.status === "authenticated") return { name: "overview" };
  return true;
});

export function resetRouterAuthForTests(): void {
  initialized = false;
}

export default router;
