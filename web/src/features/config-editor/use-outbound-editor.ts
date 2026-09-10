import { computed, onMounted, ref } from "vue";
import type { Draft } from "@/features/profiles/types";
import { subscriptionApi } from "@/features/subscriptions/api";
import type { SubscriptionDetail, SubscriptionSummary } from "@/features/subscriptions/types";
import { outboundEditorGroups, restoreOutboundEditorState } from "./outbound";

/** Keep outbound cards and reference options on the same editor model. */
export function useOutboundEditor(draft: () => Draft) {
  const subscriptionSummaries = ref<SubscriptionSummary[]>([]);
  const subscriptionDetails = ref<SubscriptionDetail[]>([]);
  const subscriptionError = ref("");
  let summaryRequest: Promise<void> | null = null;
  let detailRequest: Promise<void> | null = null;

  const rawItems = computed<unknown[]>(() => {
    const outbounds = draft().config.outbounds;
    return Array.isArray(outbounds) ? outbounds : [];
  });
  const editorState = computed(() => restoreOutboundEditorState(rawItems.value, subscriptionDetails.value, draft().editorState?.outbounds));
  const groups = computed(() => outboundEditorGroups(rawItems.value, editorState.value));
  const tags = computed(() => groups.value.map((group) => group.tag));

  function refreshSubscriptions(): Promise<void> {
    if (summaryRequest) return summaryRequest;
    summaryRequest = (async () => {
      try {
        subscriptionSummaries.value = await subscriptionApi.list();
        subscriptionError.value = "";
      } catch (cause) {
        subscriptionError.value = cause instanceof Error ? cause.message : "无法读取订阅列表";
      } finally {
        summaryRequest = null;
      }
    })();
    return summaryRequest;
  }

  function loadSubscriptionDetails(): Promise<void> {
    if (detailRequest) return detailRequest;
    detailRequest = (async () => {
      await refreshSubscriptions();
      const cached = new Map(subscriptionDetails.value.map((detail) => [detail.id, detail]));
      const missing = subscriptionSummaries.value.filter((summary) => !cached.has(summary.id));
      if (!missing.length) return;
      const results = await Promise.allSettled(missing.map((summary) => subscriptionApi.get(summary.id)));
      let failures = 0;
      results.forEach((result) => {
        if (result.status === "fulfilled") cached.set(result.value.id, result.value);
        else failures++;
      });
      subscriptionDetails.value = subscriptionSummaries.value.flatMap((summary) => {
        const detail = cached.get(summary.id);
        return detail ? [detail] : [];
      });
      if (failures) subscriptionError.value = `有 ${failures} 个订阅暂时无法读取，仍可编辑其他引用。`;
    })().finally(() => { detailRequest = null; });
    return detailRequest;
  }

  onMounted(() => { void loadSubscriptionDetails(); });

  return { rawItems, editorState, groups, tags, subscriptionDetails, subscriptionError, loadSubscriptionDetails };
}
