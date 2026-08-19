import { watch } from "vue";
import { useDraftStore } from "../../stores/draft";

const GUARDED_STATUSES = new Set(["dirty", "saving", "offline", "conflict"]);

/**
 * 草稿保护：仅在草稿有未保存变更或处于保存流程中时拦截页面关闭，
 * 并在网络恢复后按当前 ETag 重试一次保存。发布请求绝不自动重试。
 */
export function useDraftAutosave(): void {
  const draft = useDraftStore();

  function beforeunload(event: BeforeUnloadEvent): void {
    event.preventDefault();
  }

  async function retryOnce(): Promise<void> {
    if (draft.status === "offline") {
      await draft.save();
    }
  }

  watch(
    () => draft.status,
    (status, previous) => {
      const guardedBefore = previous !== undefined && GUARDED_STATUSES.has(previous);
      const guardedNow = GUARDED_STATUSES.has(status);
      if (guardedNow && !guardedBefore) {
        window.addEventListener("beforeunload", beforeunload);
      } else if (!guardedNow && guardedBefore) {
        window.removeEventListener("beforeunload", beforeunload);
      }
    },
    { flush: "sync" },
  );

  window.addEventListener("online", retryOnce);
}
