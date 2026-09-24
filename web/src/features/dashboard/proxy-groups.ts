import type { RuntimeProxyGroup } from "./api";
import type { Draft } from "@/features/profiles/types";

type OutboundConfig = Record<string, unknown> & { type: string; tag: string };
function isOutboundConfig(value: unknown): value is OutboundConfig { return Boolean(value && typeof value === "object" && !Array.isArray(value) && typeof (value as Record<string, unknown>).type === "string" && typeof (value as Record<string, unknown>).tag === "string"); }
function isPolicyOutbound(value: OutboundConfig): boolean { return value.type === "selector" || value.type === "urltest"; }
function memberTags(value: OutboundConfig): string[] { return Array.isArray(value.outbounds) ? value.outbounds.filter((tag): tag is string => typeof tag === "string") : []; }
function presentationByTag(value: unknown): Map<string, { id: string; hidden: boolean; icon: string }> {
  const state = value && typeof value === "object" && !Array.isArray(value) ? value as { groups?: Array<{ id?: string; tag?: string; hidden?: boolean; icon?: string }> } : null;
  return new Map((state?.groups ?? []).filter((group) => typeof group.tag === "string").map((group) => [group.tag!, { id: group.id ?? group.tag!, hidden: group.hidden === true, icon: group.icon ?? "" }]));
}

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

/** Match the outbound editor's v2 reference order, then retain raw-only items. */
export function outboundOrderFromDraft(draft: Draft): string[] {
  const configured = (Array.isArray(draft.config.outbounds) ? draft.config.outbounds : [])
    .filter(isOutboundConfig).map((item) => item.tag);
  const references = draft.schemaVersion === 2 ? draft.references?.outbounds.map((group) => group.tag) ?? [] : [];
  return [...new Set([...references, ...configured])];
}

/** Builds display groups from a server candidate or applied outbound list. */
export function buildDashboardProxyGroups(outbounds: unknown[], storedState?: unknown, outboundOrder: string[] = []): DashboardProxyGroup[] {
  const display = presentationByTag(storedState);
  const configs = outbounds.filter(isOutboundConfig);
  const configuredGroups: Array<OutboundConfig & { id: string; hidden: boolean; icon: string }> = configs.filter(isPolicyOutbound).map((config) => ({ ...config, id: display.get(config.tag)?.id ?? config.tag, hidden: display.get(config.tag)?.hidden ?? false, icon: display.get(config.tag)?.icon ?? "" }));
  const rank = new Map(outboundOrder.map((tag, index) => [tag, index]));
  configuredGroups.sort((left, right) => (rank.get(left.tag) ?? Number.MAX_SAFE_INTEGER) - (rank.get(right.tag) ?? Number.MAX_SAFE_INTEGER));
  const groupByTag = new Map(configuredGroups.map((group) => [group.tag, group]));
  const configByTag = new Map(configs.map((config) => [config.tag, config]));

  return configuredGroups.filter((group) => !group.hidden).map((group, index) => {
    const presentation = groupPresentation(group.tag, group.type);
    const tags = [...new Set(memberTags(group))];
    const members = tags.map((tag) => {
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
    const configuredDefault = typeof group.default === "string" ? group.default : "";
    return {
      id: group.id,
      tag: group.tag,
      icon: group.icon || presentation.icon,
      name: presentation.name,
      type: typeLabel(group.type),
      selected: tags.includes(configuredDefault) ? configuredDefault : tags[0] ?? "",
      expanded: index < 2,
      members
    };
  });
}

/** Runtime membership and selection take precedence over an unapplied draft. */
export function buildRuntimeDashboardProxyGroups(runtime: RuntimeProxyGroup[], outbounds: unknown[], storedState?: unknown, outboundOrder: string[] = []): DashboardProxyGroup[] {
  const configured = new Map(outbounds.filter(isOutboundConfig).map((config) => [config.tag, config]));
  const runtimeConfigs = new Map<string, OutboundConfig>();
  for (const group of runtime) {
    for (const member of group.members) {
      const type = member.type === "Reject" ? "block" : member.type.toLowerCase();
      const config = configured.get(member.tag);
      runtimeConfigs.set(member.tag, { tag: member.tag, type, ...(config?.type === type ? { network: config.network } : {}) });
    }
  }
  const groupConfigs = runtime.map((group) => ({
    tag: group.tag,
    type: group.type.toLowerCase(),
    outbounds: group.members.map((member) => member.tag),
    default: group.selected
  }));
  const groupTags = new Set(runtime.map((group) => group.tag));
  const configs = [...groupConfigs, ...[...runtimeConfigs.values()].filter((config) => !groupTags.has(config.tag))];
  const selectedByTag = new Map(runtime.map((group) => [group.tag, group.selected]));
  return buildDashboardProxyGroups(configs, storedState, outboundOrder).map((group) => ({ ...group, selected: selectedByTag.get(group.tag) ?? "" }));
}
