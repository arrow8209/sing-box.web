import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import CreateProfileDialog from "./CreateProfileDialog.vue";
import { useProfilesStore } from "../../stores/profiles";

const router = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("vue-router", () => ({
  useRouter: () => router,
}));

describe("CreateProfileDialog", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("passes the router to create so a successful creation opens its general module", async () => {
    const profiles = useProfilesStore();
    const create = vi.spyOn(profiles, "create").mockResolvedValue({ id: "a" } as never);
    const wrapper = mount(CreateProfileDialog);

    const inputs = wrapper.findAll('input[type="text"]');
    await inputs[0].setValue("家庭网关");
    await inputs[1].setValue("home-gateway");
    await wrapper.get('button[name="创建"]').trigger("click");

    expect(create).toHaveBeenCalledWith({ name: "家庭网关", slug: "home-gateway" }, router);
  });
});
