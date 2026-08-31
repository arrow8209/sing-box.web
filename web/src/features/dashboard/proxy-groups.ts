import {
  isOutboundConfig,
  isPolicyOutbound,
  outboundEditorGroups,
  outboundReferenceTags,
  restoreOutboundEditorState,
  type OutboundConfig
} from "@/features/config-editor/outbound";

export interface DashboardProxyMember {
  id: string;
  tag: string;
  name: string;
  type: string;
  network: string;
  latency: number | null | undefined;
  icon?: string;
  groupId?: string;
}

export interface DashboardProxyGroup {
  id: string;
  tag: string;
  icon: string;
  name: string;
  type: string;
  selected: string;
  expanded: boolean;
  members: DashboardProxyMember[];
}

const typeLabels: Record<string, string> = {
  anytls: "AnyTLS",
  block: "Block",
  direct: "Direct",
  http: "HTTP",
  hysteria: "Hysteria",
  hysteria2: "Hysteria2",
  selector: "Selector",
  shadowsocks: "Shadowsocks",
  socks: "SOCKS",
  ssh: "SSH",
  trojan: "Trojan",
  tuic: "TUIC",
  urltest: "URLTest",
  vless: "VLESS",
  vmess: "VMess",
  wireguard: "WireGuard"
};

const leadingEmoji = /^((?:\p{Regional_Indicator}{2})|(?:\p{Extended_Pictographic}(?:\uFE0F|\u200D\p{Extended_Pictographic}(?:\uFE0F)?)*)+)\s*/u;

function tagEmoji(tag: string): string | undefined {
  return tag.match(leadingEmoji)?.[1];
}

function typeLabel(type: string): string {
  const normalized = type.toLowerCase();
  if (typeLabels[normalized]) return typeLabels[normalized]!;
  return normalized.split(/[-_]/).map((part) => part ? `${part[0]!.toUpperCase()}${part.slice(1)}` : "").join(" ");
}

function groupPresentation(tag: string, type: string): { icon: string; name: string } {
  const match = tag.match(leadingEmoji);
  const name = match ? tag.slice(match[0].length).trim() : tag;
  if (match && name) return { icon: match[1]!, name };
  if (tag.toUpperCase() === "GLOBAL") return { icon: "🌐", name: tag };
  return { icon: type === "urltest" ? "📍" : "🚀", name: tag };
}

function configuredNetwork(config: OutboundConfig | undefined): string {
  return typeof config?.network === "string" ? config.network : "";
}

function configuredLatency(config: OutboundConfig | undefined): number | null | undefined {
  if (config?.type === "block") return null;
  return typeof config?.latency === "number" && Number.isFinite(config.latency) ? config.latency : undefined;
}

/** Builds the dashboard shortcut groups from the profile's materialized sing-box outbounds. */
export function buildDashboardProxyGroups(outbounds: unknown[], storedState?: unknown): DashboardProxyGroup[] {
  const editorState = restoreOutboundEditorState(outbounds, [], storedState);
  const configuredGroups = outboundEditorGroups(outbounds, editorState).filter((group) => isPolicyOutbound(group.config));
  const groupByTag = new Map(configuredGroups.map((group) => [group.tag, group]));
  const configByTag = new Map(outbounds.filter(isOutboundConfig).map((config) => [config.tag, config]));

  return configuredGroups.filter((group) => !group.hidden).map((group, index) => {
    const presentation = groupPresentation(group.tag, group.type);
    const memberTags = [...new Set(outboundReferenceTags(group.config))];
    const members = memberTags.map((tag) => {
      const config = configByTag.get(tag);
      const referencedGroup = groupByTag.get(tag);
      const referencedPresentation = referencedGroup ? groupPresentation(referencedGroup.tag, referencedGroup.type) : null;
      const icon = referencedPresentation?.icon ?? tagEmoji(tag);
      return {
        id: tag,
        tag,
        name: referencedPresentation?.name ?? tag,
        type: typeLabel(config?.type ?? "unknown"),
        network: configuredNetwork(config),
        latency: configuredLatency(config),
        ...(icon ? { icon } : {}),
        ...(referencedGroup ? { groupId: referencedGroup.id } : {})
      };
    });
    const configuredDefault = typeof group.config.default === "string" ? group.config.default : "";
    return {
      id: group.id,
      tag: group.tag,
      icon: presentation.icon,
      name: presentation.name,
      type: typeLabel(group.type),
      selected: memberTags.includes(configuredDefault) ? configuredDefault : memberTags[0] ?? "",
      expanded: index < 2,
      members
    };
  });
}
