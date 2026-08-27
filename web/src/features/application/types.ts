import type { ApiErrorBody } from "@/api/types";

export interface GeneratedFile { name: string; content: unknown; }
export interface PreviewResult { revision: number; files: GeneratedFile[]; digest: string; errors?: ApiErrorBody[]; }
export interface ValidationResult extends PreviewResult {
  valid: boolean; stage: string; message?: string; code?: string; stdout?: string; stderr?: string;
  exitCode: number; timedOut?: boolean; outputTruncated?: boolean;
}
export interface CoreOperation { active: boolean; kind?: string; stage?: string; startedAt?: string; error?: string; }
export interface CoreStatus {
  installedVersion: string; installedDigest: string; state: "not_installed" | "stopped" | "starting" | "running" | "stopping" | "start_failed" | "exited";
  currentProfileId: string; appliedRevision: number | null; lastError: string; operation: CoreOperation;
}
export interface ApplyResult { profileId?: string; revision?: number; digest?: string; }
export interface CheckFailureDetails {
  command?: string[];
  checkedConfigDirectory?: string;
  reviewCommand?: string[];
  reviewConfigDirectory?: string;
  configFiles?: string[];
  profileId?: string;
  revision?: number;
  configDigest?: string;
  exitCode?: number;
  timedOut?: boolean;
  outputTruncated?: boolean;
  stdout?: string;
  stderr?: string;
  processError?: string;
  reviewError?: string;
}
export interface ApplicationError { code: string; message: string; stage?: string; fieldPath?: string; output?: string; details?: CheckFailureDetails; }

export const stageLabels: Record<string, string> = {
  semantic_check: "语义检查", core_check: "核心检查",
  snapshot: "准备资源快照", stop_core: "停止核心", generate_config: "生成配置", check_config: "检查配置",
  swap_config: "切换配置", start_core: "启动核心", commit_state: "提交状态", recover: "恢复此前配置"
};
