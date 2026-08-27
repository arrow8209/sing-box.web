import type { ApiErrorBody } from "@/api/types";

export interface GeneratedFile { name: string; content: unknown; }
export interface PreviewResult { revision: number; files: GeneratedFile[]; digest: string; errors?: ApiErrorBody[]; }
export interface ValidationResult extends PreviewResult {
  valid: boolean; stage: string; message?: string; code?: string; stdout?: string; stderr?: string;
  exitCode: number; timedOut?: boolean; outputTruncated?: boolean; command?: string[]; configDirectory?: string;
}
export interface ApplicationError { code: string; message: string; stage?: string; fieldPath?: string; output?: string; }

export const stageLabels: Record<string, string> = {
  semantic_check: "语义检查", core_check: "核心检查"
};
