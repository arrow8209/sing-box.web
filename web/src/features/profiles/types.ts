export interface ValidationSummary {
  valid: boolean;
  revision: number;
  checkedAt: string;
  stage: string;
  message: string;
  resourceDigests?: Record<string,string>;
  configDigest?: string;
  stale?: boolean;
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
  configCurrent?: boolean;
  needsApply: boolean;
  lastValidation: ValidationSummary | null;
  quarantinedError?: string;
}

export interface Draft {
  profileId: string;
  revision: number;
  schemaVersion?: number;
  references?: PersistedReferences;
  config: Record<string, unknown>;
  buffers: Record<string, string>;
  editorState?: Record<string, unknown>;
  bindings: RuleBinding[];
  savedAt: string;
}

// The server deliberately rebuilds config from buffers. Keep config out of saves:
// JSON in config can contain values that browsers cannot round-trip safely.
export type DraftInput = Pick<Draft, "buffers" | "editorState" | "bindings" | "schemaVersion" | "references">;

export interface PersistedReference { kind: string; tag?: string; outboundId?: string; subscriptionId?: string; nodeTag?: string; }
export interface PersistedReferences {
  version: 2;
  outbounds: Array<{ id: string; tag: string; type: string; include?: string; exclude?: string; references: PersistedReference[] }>;
  inbounds: Array<{ tag: string; resourceId: string; sourceTag: string; position?: number }>;
}

export interface FreezeOperation {
  action: "detach" | "copy" | "restore_saved";
  module: "inbounds";
  tag: string;
  newTag?: string;
  text?: string;
  routeText?: string;
  inboundStateText?: string;
  digest?: string;
  resourceDigests?: Record<string, string>;
  token?: string;
}

export type SaveState = "idle" | "dirty" | "saving" | "saved" | "offline" | "conflict" | "error";

export interface DraftConflict {
  latestRevision?: number;
}

export interface AutoReloadOutcome {
  attempted: boolean;
  applied: boolean;
  reloaded: boolean;
  message: string;
}
