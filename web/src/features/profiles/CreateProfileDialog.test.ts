import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, createRouter } from "vue-router";
import CreateProfileDialog from "./CreateProfileDialog.vue";
import { useProfilesStore } from "../../stores/profiles";

function mountCreateProfileDialog() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: "/", component: { template: "<div />" } },
      { path: "/config/:id/general", component: { template: "<div />" } },
    ],
  });
  void router.push("/config/test-profile/general");
  return {
    router,
    wrapper: mount(CreateProfileDialog, { global: { plugins: [router] } }),
  };
}

describe("CreateProfileDialog", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("passes the router to create so a successful creation opens its general module", async () => {
    const profiles = useProfilesStore();
    const create = vi.spyOn(profiles, "create").mockResolvedValue({ id: "a" } as never);
    const { router, wrapper } = mountCreateProfileDialog();

    const inputs = wrapper.findAll('input[type="text"]');
    await inputs[0].setValue("家庭网关");
    await inputs[1].setValue("home-gateway");
    await wrapper.get('button[name="创建"]').trigger("click");

    expect(create).toHaveBeenCalledWith({ name: "家庭网关", slug: "home-gateway" }, router);
  });

  it("keeps the dialog open and shows an error when creation fails", async () => {
    const profiles = useProfilesStore();
    vi.spyOn(profiles, "create").mockRejectedValue(new Error("create failed"));
    const { wrapper } = mountCreateProfileDialog();

    const inputs = wrapper.findAll('input[type="text"]');
    await inputs[0].setValue("家庭网关");
    await inputs[1].setValue("home-gateway");
    await wrapper.get('button[name="创建"]').trigger("click");

    expect(wrapper.emitted("close")).toBeUndefined();
    expect(wrapper.get('[role="alert"]').text()).toBe("创建失败，请检查名称与 slug");
  });
});
