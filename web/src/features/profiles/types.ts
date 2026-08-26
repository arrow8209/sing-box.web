export interface ValidationSummary {
  valid: boolean;
  revision: number;
  checkedAt: string;
  stage: string;
  message: string;
}

export interface RuleBinding {
  resourceId: string;
  tag: string;
  enabled: boolean;
}

export interface ProfileSummary {
  id: string;
  name: string;
  draftRevision: number;
  appliedRevision: number | null;
  appliedDigest: string;
  appliedAt: string | null;
  needsApply: boolean;
  lastValidation: ValidationSummary | null;
  quarantinedError?: string;
}

export interface Draft {
  profileId: string;
  revision: number;
  config: Record<string, unknown>;
  buffers: Record<string, string>;
  editorState?: Record<string, unknown>;
  bindings: RuleBinding[];
  savedAt: string;
}

// The server deliberately rebuilds config from buffers. Keep config out of saves:
// JSON in config can contain values that browsers cannot round-trip safely.
export type DraftInput = Pick<Draft, "buffers" | "editorState" | "bindings">;

export type SaveState = "idle" | "dirty" | "saving" | "saved" | "offline" | "conflict" | "error";

export interface DraftConflict {
  latestRevision?: number;
}
