export type RuleFormat = "source" | "binary";
export type RuleSourceType = "remote" | "manual";

export interface RuleResource {
  id: string;
  catalogKey?: string;
  sourceType: RuleSourceType;
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
  sizeBytes?: number;
  ruleCount?: number;
}

export interface RuleResourceDetail extends RuleResource { content?: unknown; }

export interface RemoteRuleResourceInput {
  sourceType: "remote";
  name: string;
  url: string;
  format: RuleFormat;
  updateInterval: string;
  disabled: boolean;
}

export interface ManualRuleResourceInput {
  sourceType: "manual";
  name: string;
  format: "source";
  url: "";
  updateInterval: "";
  disabled: boolean;
  content: unknown;
}

export type RuleResourceInput = RemoteRuleResourceInput | ManualRuleResourceInput;
export interface RuleResourcePatch {
  sourceType?: RuleSourceType;
  name?: string;
  url?: string;
  format?: RuleFormat;
  updateInterval?: string;
  disabled?: boolean;
  content?: unknown;
}

export interface RuleUpdateResult { changed: boolean; digest: string; message: string; }
export interface RuleCatalogItem { key: string; name: string; type: "geosite" | "geoip"; count: number; }
export interface RuleCatalog {
  available: boolean;
  source: string;
  geosite: string;
  geoip: string;
  digest?: string;
  fetchedAt?: string;
  total: number;
  geositeCount: number;
  geoipCount: number;
  items: RuleCatalogItem[];
}
export interface RuleCatalogPreview { key: string; ruleBlocks: number; ruleCount: number; truncated: boolean; content: unknown; }
export interface RuleCatalogImportResult { resource: RuleResource; update?: RuleUpdateResult; warning?: string; }
export interface RuleBatchResult { total: number; changed: number; unchanged: number; failed: number; }
export interface ProfileReference { id: string; name: string; }
