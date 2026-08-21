import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import ProfileDeleteDialog from "@/features/profiles/ProfileDeleteDialog.vue";
import SaveStatus from "@/features/profiles/SaveStatus.vue";

describe("档案组件", () => {
  it("保存完成与需要应用是两个状态", () => {
    const wrapper = mount(SaveStatus, { props: { state: "saved", needsApply: true } });
    expect(wrapper.text()).toContain("草稿已保存"); expect(wrapper.text()).toContain("需要应用");
  });
  it("删除必须输入精确档案名称", async () => {
    const wrapper = mount(ProfileDeleteDialog, { props: { open: true, name: "家庭出口" } });
    expect(wrapper.get("button.danger").attributes("disabled")).toBeDefined();
    await wrapper.get("input").setValue("家庭出口"); expect(wrapper.get("button.danger").attributes("disabled")).toBeUndefined();
  });
});
