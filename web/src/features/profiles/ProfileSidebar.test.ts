import { beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import ProfileSidebar from "./ProfileSidebar.vue";
import { useProfilesStore } from "../../stores/profiles";
import { useDraftStore } from "../../stores/draft";
import type { Profile } from "../../api/types";

const profileA: Profile = {
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

describe("ProfileSidebar", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.restoreAllMocks();
  });

  it("shows display name, immutable slug and revision badges", () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA];
    profiles.activeId = profileA.id;

    const wrapper = mount(ProfileSidebar);

    expect(wrapper.text()).toContain("家庭网关");
    expect(wrapper.text()).toContain("home-gateway");
    expect(wrapper.text()).toContain("草稿 r3");
    expect(wrapper.text()).toContain("已发布 r2");
  });

  it("exposes 新建档案 opening a dialog named 创建配置档案", async () => {
    const wrapper = mount(ProfileSidebar);

    await wrapper.get('button[name="新建档案"]').trigger("click");

    expect(wrapper.get('[role="dialog"][aria-label="创建配置档案"]').isVisible()).toBe(true);
    expect(wrapper.text()).toContain("显示名称");
    expect(wrapper.text()).toContain("slug");
    expect(wrapper.get('button[name="创建"]').isVisible()).toBe(true);
  });

  it("offers 重命名档案 on the selected profile and edits name only", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA];
    profiles.activeId = profileA.id;

    const wrapper = mount(ProfileSidebar);
    await wrapper.get('button[name="重命名档案"]').trigger("click");

    const dialog = wrapper.get('[role="dialog"][aria-label="重命名档案"]');
    const input = dialog.get('input[type="text"]');
    expect((input.element as HTMLInputElement).value).toBe("家庭网关");
    expect(dialog.text()).not.toContain("slug");
  });

  it("shows a conflict dialog named 草稿冲突 with reload and overwrite actions", async () => {
    const draft = useDraftStore();
    draft.draft = {
      profileId: profileA.id,
      revision: 3,
      content: { config: { log: { level: "info" } }, editorBuffers: {} },
      savedAt: "2026-08-18T10:00:00Z",
    };
    draft.conflict = {
      localContent: { config: { log: { level: "debug" } }, editorBuffers: {} },
      serverRevision: 4,
    };
    draft.status = "conflict";

    const wrapper = mount(ProfileSidebar);

    const dialog = wrapper.get('[role="dialog"][aria-label="草稿冲突"]');
    expect(dialog.get('button[name="重新载入服务器版本"]').isVisible()).toBe(true);
    expect(dialog.get('button[name="用本地内容覆盖"]').isVisible()).toBe(true);
  });

  it("copies directoryHint and checkCommand verbatim", async () => {
    const profiles = useProfilesStore();
    profiles.items = [profileA];
    profiles.activeId = profileA.id;

    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });

    const wrapper = mount(ProfileSidebar);
    const buttons = wrapper.findAll('button[name="复制"]');
    await buttons[0].trigger("click");
    await buttons[1].trigger("click");

    expect(writeText).toHaveBeenCalledWith("/etc/sing-box/home-gateway");
    expect(writeText).toHaveBeenCalledWith("sing-box check -C /etc/sing-box/home-gateway");
  });

  it("renders save state in an element with role=status", () => {
    const draft = useDraftStore();
    draft.status = "dirty";

    const wrapper = mount(ProfileSidebar);
    const status = wrapper.get('[role="status"]');

    expect(status.text()).toBe("草稿保存中");
  });

  it("shows the safe repair message for a read-only draft and disables the editor entry", () => {
    const readOnlyProfile: Profile = {
      ...profileA,
      draftReadOnly: true,
      draftError: "草稿 JSON 损坏，需要管理员在服务器修复",
    };
    const profiles = useProfilesStore();
    profiles.items = [readOnlyProfile];
    profiles.activeId = readOnlyProfile.id;

    const wrapper = mount(ProfileSidebar);

    expect(wrapper.text()).toContain("草稿 JSON 损坏，需要管理员在服务器修复");
  });
});
