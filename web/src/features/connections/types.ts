export type ConnectionMetadata = Record<string, unknown> & {
  type?: string;
  network?: string;
  host?: string;
  sourceIP?: string;
  sourcePort?: string;
  destinationIP?: string;
  destinationPort?: string;
  inboundIP?: string;
  inboundPort?: string;
  inboundName?: string;
  inboundUser?: string;
  process?: string;
  processPath?: string;
  dnsMode?: string;
};

export interface RuntimeConnection {
  id: string;
  metadata: ConnectionMetadata;
  upload: number;
  download: number;
  start: string;
  chains: string[];
  rule: string;
  rulePayload: string;
}

export interface ConnectionsSnapshot {
  memory: number;
  uploadTotal: number;
  downloadTotal: number;
  connections: RuntimeConnection[];
}

export interface DisplayConnection extends RuntimeConnection {
  uploadSpeed: number;
  downloadSpeed: number;
  closedAt?: string;
}
