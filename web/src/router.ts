import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "./stores/auth";
import { useCapabilitiesStore } from "./stores/capabilities";
import { useProfilesStore } from "./stores/profiles";
import LoginView from "./features/auth/LoginView.vue";
import AppShell from "./features/shell/AppShell.vue";
import ConfigWorkspace from "./features/config/ConfigWorkspace.vue";
import DashboardView from "./features/dashboard/DashboardView.vue";
import SettingsView from "./features/settings/SettingsView.vue";
import RuntimePlaceholderView from "./features/placeholders/RuntimePlaceholderView.vue";
import SubscriptionPlaceholderView from "./features/placeholders/SubscriptionPlaceholderView.vue";
import RulesetPlaceholderView from "./features/placeholders/RulesetPlaceholderView.vue";

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
        component: DashboardView,
      },
      {
        path: "config/:profileId?/:module?",
        name: "config",
        component: ConfigWorkspace,
      },
      {
        path: "runtime",
        name: "runtime",
        component: RuntimePlaceholderView,
      },
      {
        path: "subscriptions",
        name: "subscriptions",
        component: SubscriptionPlaceholderView,
      },
      {
        path: "rulesets",
        name: "rulesets",
        component: RulesetPlaceholderView,
      },
      {
        path: "settings",
        name: "settings",
        component: SettingsView,
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
  if (auth.session !== null && to.name === "config") {
    const profiles = useProfilesStore();
    try {
      if (profiles.items.length === 0) {
        await profiles.load();
      }
      const profileId = to.params.profileId;
      if (typeof profileId === "string") {
        await profiles.select(profileId);
      }
    } catch {
      return false;
    }
  }
  return true;
});
