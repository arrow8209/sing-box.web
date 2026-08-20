export interface ValidationIssue {
  stage: string;
  path: string;
  code: string;
  message: string;
}

export interface APIError {
  code: string;
  message: string;
  fieldIssues?: ValidationIssue[];
  currentRevision?: number;
  stderr?: string;
}

export interface SessionResponse {
  username: "admin";
  csrfToken: string;
  expiresAt: string;
}

export interface CapabilitySnapshot {
  capabilities: Record<
    | "config.manage"
    | "config.publish"
    | "subscription.sync"
    | "ruleset.sync"
    | "core.lifecycle"
    | "core.metrics",
    boolean
  >;
  singBox: {
    available: boolean;
    version: string;
    targetVersion: string;
    warning: string;
  };
}

export interface Profile {
  id: string;
  name: string;
  slug: string;
  draftRevision: number;
  publishedRevision: number | null;
  publishedDigest: string;
  publishedAt: string | null;
  lastValidation: null | {
    revision: number;
    status: string;
    stage: string;
    message: string;
    checkedAt: string;
  };
  directoryHint: string;
  checkCommand: string;
  draftReadOnly: boolean;
  draftError: string;
}

export interface EditorBuffer {
  text: string;
  valid: boolean;
  applied: boolean;
  error?: string;
}

export interface Draft {
  profileId: string;
  revision: number;
  content: {
    config: Record<string, unknown>;
    editorBuffers: Record<string, EditorBuffer>;
  };
  savedAt: string;
}

export interface ValidationResult {
  valid: boolean;
  issues: ValidationIssue[];
  targetVersion: string;
  installedVersion: string;
  checkedAt: string;
}

export interface PublicationReceipt {
  profileId: string;
  revision: number;
  digest: string;
  directory: string;
  publishedAt: string;
  validation: ValidationResult;
}
