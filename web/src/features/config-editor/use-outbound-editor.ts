import { computed, ref } from "vue";
import type { Draft } from "@/features/profiles/types";
import { subscriptionApi } from "@/features/subscriptions/api";
import type { SubscriptionDetail } from "@/features/subscriptions/types";
import { cloneValue, isManagedOutbound, isOutboundConfig, outboundReferenceTags, parseOutboundEditorState, type OutboundEditorGroup, type OutboundReference } from "./outbound";

/** Keep outbound cards and reference options on the same editor model. */
export function useOutboundEditor(draft: () => Draft) {
  const subscriptionDetails = ref<SubscriptionDetail[]>([]);
  const subscriptionError = ref("");
  let detailRequest: Promise<void> | null = null;

  const rawItems = computed<unknown[]>(() => {
    const outbounds = draft().config.outbounds;
    return Array.isArray(outbounds) ? outbounds : [];
  });
  const groups = computed<OutboundEditorGroup[]>(() => {
    const value = draft();
    const legacy = parseOutboundEditorState(value.editorState?.outbounds);
    const business = new Map((value.schemaVersion === 2 ? value.references?.outbounds ?? [] : []).map((group) => [group.tag, group]));
    const uiState = value.editorState?.outbounds as { groups?: Array<{ id: string; tag: string; hidden?: boolean; icon?: string }> } | undefined;
    const ui = new Map((uiState?.groups ?? []).map((group) => [group.tag, group]));
    const generated = new Set(legacy?.generatedTags ?? []);
    const orderedTags = value.schemaVersion === 2 ? value.references?.outbounds.map((group) => group.tag) ?? [] : [];
    const rank = new Map(orderedTags.map((tag, index) => [tag, index]));
    return rawItems.value.filter(isOutboundConfig).filter(isManagedOutbound).filter((config) => !generated.has(config.tag)).map((config) => {
      const stored = business.get(config.tag);
      const old = legacy?.groups.find((group) => group.tag === config.tag);
      const presentation = ui.get(config.tag);
      const fallbackReferences: OutboundReference[] = outboundReferenceTags(config).map((tag) => ({ kind: "raw", tag }));
      return { id: stored?.id ?? old?.id ?? `raw-${config.tag}`, tag: config.tag, type: config.type as OutboundEditorGroup["type"],
        hidden: presentation?.hidden ?? old?.hidden ?? config.hidden === true, icon: presentation?.icon ?? old?.icon ?? "",
        include: stored?.include ?? old?.include ?? "", exclude: stored?.exclude ?? old?.exclude ?? "",
        references: cloneValue((stored?.references ?? old?.references ?? fallbackReferences) as OutboundReference[]),
        runtimeTags: outboundReferenceTags(config), config: cloneValue(config) };
    }).sort((left, right) => (rank.get(left.tag) ?? Number.MAX_SAFE_INTEGER) - (rank.get(right.tag) ?? Number.MAX_SAFE_INTEGER));
  });
  const tags = computed(() => groups.value.map((group) => group.tag));

  function loadSubscriptionDetails(): Promise<void> {
    if (detailRequest) return detailRequest;
    detailRequest = (async () => {
      subscriptionDetails.value = [];
      const summaries = await subscriptionApi.list();
      const results = await Promise.allSettled(summaries.map((summary) => subscriptionApi.get(summary.id)));
      subscriptionDetails.value = results.flatMap((result) => result.status === "fulfilled" ? [result.value] : []);
      const failures = results.length - subscriptionDetails.value.length;
      subscriptionError.value = failures ? `有 ${failures} 个订阅暂时无法读取，仍可编辑其他引用。` : "";
    })().finally(() => { detailRequest = null; });
    return detailRequest;
  }

  return { rawItems, groups, tags, subscriptionDetails, subscriptionError, loadSubscriptionDetails };
}
