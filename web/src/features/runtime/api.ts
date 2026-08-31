import { ApiError } from "@/api/client";

export interface RuntimeStatus { available: boolean; observedAt: string | null; }
export interface TrafficSnapshot { uploadBytes: number; downloadBytes: number; observedAt: string; }
export interface ProxyGroupSummary { tag: string; selected: string; }
export interface RuntimeEvent { type: "status" | "traffic" | "connections" | "proxyGroups"; observedAt: string; }

export interface RuntimeApi {
  getStatus(): Promise<RuntimeStatus>;
  getTraffic(): Promise<TrafficSnapshot>;
  listProxyGroups(): Promise<ProxyGroupSummary[]>;
  subscribe(listener: (event: RuntimeEvent) => void): () => void;
}

export class UnavailableRuntimeApi implements RuntimeApi {
  private unavailable(): ApiError {
    return new ApiError(501, { code: "runtime_unavailable", message: "其余运行数据将在后续阶段接入" });
  }

  getStatus(): Promise<RuntimeStatus> { return Promise.reject(this.unavailable()); }
  getTraffic(): Promise<TrafficSnapshot> { return Promise.reject(this.unavailable()); }
  listProxyGroups(): Promise<ProxyGroupSummary[]> { return Promise.reject(this.unavailable()); }
  subscribe(_listener: (event: RuntimeEvent) => void): () => void { return () => undefined; }
}
