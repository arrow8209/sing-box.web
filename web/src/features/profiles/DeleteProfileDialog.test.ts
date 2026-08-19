import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import DeleteProfileDialog from "./DeleteProfileDialog.vue";
import { apiClient } from "../../api/client";
import { useProfilesStore } from "../../stores/profiles";
import type { Profile } from "../../api/types";

const profile: Profile = {
  id: "a".repeat(32),
  name: "家庭网关",
  slug: "home-gateway",
  draftRevision: 3,
  publishedRevision: 2,
  publishedDigest: "digest-a",
  publishedAt: "2026-08-18T10:00:00Z",
  lastValidation: null,
  directoryHint: "/etc/sing-box/home-gateway",
  checkCommand: "sing-box check -C /etc/sing-box/home-gateway",
  draftReadOnly: false,
  draftError: "",
};

describe("DeleteProfileDialog", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("enables the delete button only when the typed text exactly matches the display name", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;

    const wrapper = mount(DeleteProfileDialog, { props: { profileId: profile.id } });
    const input = wrapper.get('input[aria-label="档案名称确认"]');
    const button = wrapper.get('button[name="确认删除档案"]');

    expect(button.attributes("disabled")).toBeDefined();

    await input.setValue("家庭");
    expect(button.attributes("disabled")).toBeDefined();

    await input.setValue("家庭网关 ");
    expect(button.attributes("disabled")).toBeDefined();

    await input.setValue("家庭网关");
    expect(button.attributes("disabled")).toBeUndefined();
  });

  it("warns that external core usage is unknown", () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;

    const wrapper = mount(DeleteProfileDialog, { props: { profileId: profile.id } });
    expect(wrapper.text()).toContain("无法判断外部核心是否正在使用该配置路径");
  });

  it("sends confirmName and removes the profile on success", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profile];
    profiles.activeId = profile.id;
    const remove = vi.spyOn(profiles, "remove").mockImplementation(async () => undefined);
    const request = vi.spyOn(apiClient, "request").mockResolvedValue({
      data: { warning: "一期无法判断外部核心是否正在使用该路径" },
      etag: null,
    });

    const wrapper = mount(DeleteProfileDialog, { props: { profileId: profile.id } });
    await wrapper.get('input[aria-label="档案名称确认"]').setValue("家庭网关");
    await wrapper.get('button[name="确认删除档案"]').trigger("click");

    expect(request).toHaveBeenCalledWith(
      `/profiles/${profile.id}`,
      expect.objectContaining({
        method: "DELETE",
        body: JSON.stringify({ confirmName: "家庭网关" }),
      }),
    );
    expect(remove).toHaveBeenCalledWith(profile.id);
  });
});
