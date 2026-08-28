import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ProfileActions from "@/features/profiles/ProfileActions.vue";
import ProfileDeleteDialog from "@/features/profiles/ProfileDeleteDialog.vue";
import SaveStatus from "@/features/profiles/SaveStatus.vue";

describe("档案组件", () => {
  it("保存完成与需要应用是两个状态", () => {
    const wrapper = mount(SaveStatus, { props: { state: "saved", needsApply: true } });
    expect(wrapper.text()).toContain("草稿已保存"); expect(wrapper.text()).toContain("需要应用");
  });
  it("档案操作组依次提供重命名、复制和删除", async () => {
    const wrapper = mount(ProfileActions);
    const buttons = wrapper.findAll("button");
    expect(buttons.map((button) => button.text())).toEqual(["重命名", "复制", "删除"]);
    expect(buttons[0]?.attributes("disabled")).toBeUndefined();
    await buttons[0]?.trigger("click"); await buttons[1]?.trigger("click"); await buttons[2]?.trigger("click");
    expect(wrapper.emitted("rename")).toHaveLength(1);
    expect(wrapper.emitted("duplicate")).toHaveLength(1);
    expect(wrapper.emitted("remove")).toHaveLength(1);
  });
  it("删除必须输入精确档案名称", async () => {
    const wrapper = mount(ProfileDeleteDialog, { props: { open: true, name: "家庭出口" } });
    expect(wrapper.get("button.danger").attributes("disabled")).toBeDefined();
    await wrapper.get("input").setValue("家庭出口"); expect(wrapper.get("button.danger").attributes("disabled")).toBeUndefined();
  });
});
