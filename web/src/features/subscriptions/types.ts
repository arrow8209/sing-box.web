export type SubscriptionOutbound = Record<string, unknown>;

export interface SubscriptionSummary {
  id: string;
  name: string;
  nodeCount: number;
  protocols: string[];
  updatedAt: string;
}

export interface SubscriptionDetail {
  id: string;
  name: string;
  outbounds: SubscriptionOutbound[];
  updatedAt: string;
}

export interface SubscriptionInput {
  name: string;
  outbounds: SubscriptionOutbound[];
}

export interface ValidSubscriptionJson {
  valid: true;
  message: string;
  name: string;
  outbounds: SubscriptionOutbound[];
  parsed: unknown;
}

export interface InvalidSubscriptionJson {
  valid: false;
  message: string;
}

export type SubscriptionJsonValidation = ValidSubscriptionJson | InvalidSubscriptionJson;
