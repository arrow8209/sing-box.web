export interface Capabilities {
	serverReferenceResolution?: boolean;
  configuration: boolean;
  inboundResources: boolean;
  subscriptions: boolean;
  ruleResources: boolean;
  application: boolean;
  coreUpdate: boolean;
  runtime: boolean;
  traffic: boolean;
  connections: boolean;
  proxyGroups: boolean;
}
