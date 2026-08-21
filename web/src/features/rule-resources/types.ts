export type RuleFormat = "source" | "binary";

export interface RuleResource {
  id: string;
  name: string;
  url: string;
  format: RuleFormat;
  updateInterval: string;
  disabled: boolean;
  localPath: string;
  lastAttemptAt?: string;
  lastSuccessAt?: string;
  lastDigest?: string;
  lastError?: string;
  nextUpdateAt?: string;
}

export interface RuleResourceInput {
  name: string;
  url: string;
  format: RuleFormat;
  updateInterval: string;
  disabled: boolean;
}

export type RuleResourcePatch = Partial<RuleResourceInput>;

export interface RuleUpdateResult {
  changed: boolean;
  digest: string;
  message: string;
}

export interface ProfileReference { id: string; name: string; }
