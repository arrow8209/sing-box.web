import { computed, onBeforeUnmount, ref, watch, type Ref } from "vue";
import type { ProxyDelayResult, ProxyGroupsApi, RuntimeProxyGroup } from "./api";

type DelayStatus = ProxyDelayResult["status"] | "untested" | "queued" | "testing" | "unavailable";
export interface DelayState { status: DelayStatus; delay?: number; testedAt?: string; }

// This graph comes exclusively from the running core; it never expands draft
// subscriptions or rebuilds configuration references in the browser.
export function runtimeDelayGraph(groups: RuntimeProxyGroup[]) {
  const byTag = new Map(groups.map((group) => [group.tag, group]));
  const types = new Map(groups.flatMap((group) => group.members.map((member) => [member.tag, member.type] as const)));
  const eligible = (tag: string): boolean => {
    const type = types.get(tag);
    return Boolean(type && !["Reject", "Block", "DNS", "Selector", "URLTest", "Fallback"].includes(type));
  };
  function targets(tags: string[]): string[] {
    const visited = new Set<string>();
    const result: string[] = [];
    const pending = [...tags].reverse();
    while (pending.length) {
      const tag = pending.pop()!;
      if (visited.has(tag)) continue;
      visited.add(tag);
      const group = byTag.get(tag);
      if (group) pending.push(...group.members.map((member) => member.tag).reverse());
      else if (eligible(tag)) result.push(tag);
    }
    return result;
  }
  function selectedLeaf(tag: string): string | undefined {
    const visited = new Set<string>();
    while (byTag.has(tag)) {
      if (visited.has(tag)) return undefined;
      visited.add(tag);
      tag = byTag.get(tag)!.selected;
    }
    return eligible(tag) ? tag : undefined;
  }
  return { targets, selectedLeaf };
}

export function useProxyDelay(options: {
  groups: Ref<RuntimeProxyGroup[] | null>;
  available: () => boolean;
  api: () => ProxyGroupsApi;
  notice: (message: string) => void;
  error: (message: string) => void;
}) {
  const graph = computed(() => runtimeDelayGraph(options.groups.value ?? []));
  const runtimeKey = computed(() => {
    const groups = options.groups.value;
    const key = groups?.[0]?.runtimeKey;
    return key && groups?.every((group) => group.runtimeKey === key) ? key : "";
  });
  const available = computed(() => options.available() && Boolean(runtimeKey.value));
  const pending = ref(new Map<string, "queued" | "testing">());
  const results = ref(new Map<string, ProxyDelayResult>());
  const completed = ref(0);
  const total = ref(0);
  const testing = computed(() => pending.value.size > 0);
  const controllers = new Map<AbortController, ReturnType<typeof setTimeout>>();
  let generation = 0;
  let alive = true;
  const histories = computed(() => {
    const latest = new Map<string, DelayState>();
    for (const group of options.groups.value ?? []) {
      for (const member of group.members) {
        const value = member.history;
        if (!value || !Number.isInteger(value.delay) || value.delay <= 0 || value.delay > 65535 || !Number.isFinite(Date.parse(value.testedAt))) continue;
        const previous = latest.get(member.tag);
        if (!previous || Date.parse(value.testedAt) > Date.parse(previous.testedAt!)) latest.set(member.tag, { status: "ok", ...value });
      }
    }
    return latest;
  });

  function result(tag: string): DelayState {
    const local = results.value.get(tag);
    const history = histories.value.get(tag);
    return local && (!history || Date.parse(local.testedAt) >= Date.parse(history.testedAt!)) ? local : history ?? { status: "untested" };
  }

  function state(tag: string): DelayState {
    const leaf = graph.value.selectedLeaf(tag);
    if (!leaf) return { status: "unavailable" };
    const queued = pending.value.get(leaf);
    return queued ? { status: queued } : result(leaf);
  }

  function cancel(): void {
    generation++;
    for (const [controller, timer] of controllers) { clearTimeout(timer); controller.abort(); }
    controllers.clear();
    pending.value = new Map();
  }

  function reset(): void {
    cancel();
    results.value = new Map();
    completed.value = 0;
    total.value = 0;
  }

  async function start(tags: string[], label: string): Promise<void> {
    if (!alive || !available.value || testing.value) return;
    const targets = graph.value.targets(tags);
    if (!targets.length) return;
    const version = ++generation;
    const key = runtimeKey.value;
    const current = (): boolean => alive && generation === version && runtimeKey.value === key && available.value;
    pending.value = new Map(targets.map((tag) => [tag, "queued"]));
    total.value = targets.length;
    completed.value = 0;
    let index = 0;
    const counts = { ok: 0, timeout: 0, failed: 0 };

    async function worker(): Promise<void> {
      while (current() && index < targets.length) {
        const tag = targets[index++]!;
        pending.value.set(tag, "testing");
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 9000);
        controllers.set(controller, timer);
        try {
          const value = await options.api().testDelay(tag, key, controller.signal);
          if (!current()) return;
          if (controller.signal.aborted) throw new Error("测速请求未完成，请重试");
          if (!value || value.tag !== tag || value.runtimeKey !== key || !["ok", "timeout", "failed"].includes(value.status)
            || !Number.isFinite(Date.parse(value.testedAt)) || (value.status === "ok" && (!Number.isInteger(value.delay) || value.delay! <= 0 || value.delay! > 65535))) {
            throw new Error("核心测速结果无效或运行配置已变化，请刷新后重试");
          }
          results.value.set(tag, value);
          counts[value.status]++;
          completed.value++;
          pending.value.delete(tag);
        } catch (cause) {
          if (!current()) return;
          const message = controller.signal.aborted ? "测速请求未完成，请重试" : cause instanceof Error ? cause.message : "无法读取核心测速结果";
          cancel();
          options.error(message);
          return;
        } finally {
          clearTimeout(timer);
          controllers.delete(controller);
        }
      }
    }

    await Promise.all(Array.from({ length: Math.min(5, targets.length) }, () => worker()));
    if (current()) options.notice(`${label}测速完成：成功 ${counts.ok}，超时 ${counts.timeout}，失败 ${counts.failed}。`);
  }

  watch([runtimeKey, available], reset, { flush: "sync" });
  onBeforeUnmount(() => { alive = false; reset(); });
  return { available, testing, completed, total, state, result, start, reset, targets: (tags: string[]) => graph.value.targets(tags) };
}
