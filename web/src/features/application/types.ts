import type { ApiErrorBody } from "@/api/types";

export interface GeneratedFile { name: string; content: unknown; }
export interface CandidateItem {
  module: "inbounds" | "outbounds"; tag: string; type: string; sourceId?: string; sourceName?: string;
  independent: boolean; generated?: boolean; members?: string[]; listen?: string; listenPort?: number;
  sniff?: boolean; text?: string; routeText?: string; inboundStateText?: string; token?: string;
}
export interface CandidateView { profileId: string; revision: number; digest: string; resourceDigests: Record<string, string>; items: CandidateItem[]; }
export interface PreviewResult { revision: number; files: GeneratedFile[]; digest: string; resourceDigests?: Record<string,string>; candidate?: CandidateView; errors?: ApiErrorBody[]; }
export interface ValidationResult extends PreviewResult {
  valid: boolean; stage: string; message?: string; code?: string; stdout?: string; stderr?: string;
  exitCode: number; timedOut?: boolean; outputTruncated?: boolean; command?: string[]; configDirectory?: string;
}
export interface ApplyResult {
  applied: boolean;
  profileId: string;
  revision: number;
  digest: string;
  validation: ValidationResult;
}
export interface ApplicationError { code: string; message: string; stage?: string; fieldPath?: string; output?: string; }

export const stageLabels: Record<string, string> = {
  resolve_references: "解析引用",
  semantic_check: "语义检查", core_check: "核心检查", check_config: "检查配置",
  copy_config: "复制配置", restore_config: "恢复此前配置"
};
