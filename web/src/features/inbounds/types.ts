export type InboundMode = "structured" | "custom";
export type InboundConfig = Record<string, unknown> & { type: string; tag: string };

export interface InboundSummary {
  id: string;
  mode: InboundMode;
  type: string;
  tag: string;
  endpoint: string;
  fieldCount: number;
  updatedAt: string;
}

export interface InboundDetail {
  id: string;
  mode: InboundMode;
  config: InboundConfig;
  updatedAt: string;
}

export interface InboundInput {
  mode: InboundMode;
  config: InboundConfig;
}

export interface InboundRoutingIntent {
  sniff: boolean;
  sniffTimeout?: string;
}

export interface InboundEditorSubmission extends InboundInput {
  routing?: InboundRoutingIntent;
}

export interface ValidInboundJson {
  valid: true;
  message: string;
  config: InboundConfig;
}

export interface InvalidInboundJson {
  valid: false;
  message: string;
}

export type InboundJsonValidation = ValidInboundJson | InvalidInboundJson;
export type InboundEditorType = "mixed" | "http" | "socks" | "tun" | "direct" | "custom";
