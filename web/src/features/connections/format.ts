import type { DisplayConnection, RuntimeConnection } from "./types";

function text(value: unknown): string {
  if (value === null || value === undefined) return "";
  return typeof value === "string" ? value : String(value);
}

export function formatBytes(value: number, digits = 1): string {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0;
  if (safe < 1024) return `${Math.round(safe)} B`;
  const units = ["KB", "MB", "GB", "TB", "PB"];
  let amount = safe / 1024;
  let index = 0;
  while (amount >= 1024 && index < units.length - 1) {
    amount /= 1024;
    index += 1;
  }
  return `${amount.toFixed(amount >= 100 ? 0 : digits)} ${units[index]}`;
}

export function formatSpeed(value: number): string { return `${formatBytes(value)}/s`; }

export function formatAddress(ipValue: unknown, portValue: unknown): string {
  const ip = text(ipValue);
  const port = text(portValue);
  if (!ip) return port || "--";
  const host = ip.includes(":") && !ip.startsWith("[") ? `[${ip}]` : ip;
  return port ? `${host}:${port}` : host;
}

export function connectionHost(connection: RuntimeConnection): string {
  return text(connection.metadata.host) || text(connection.metadata.destinationIP) || "--";
}

export function connectionType(connection: RuntimeConnection): string {
  const type = text(connection.metadata.type) || "未知";
  const network = text(connection.metadata.network);
  return network ? `${type} · ${network.toUpperCase()}` : type;
}

export function connectionSource(connection: RuntimeConnection): string {
  return formatAddress(connection.metadata.sourceIP, connection.metadata.sourcePort);
}

export function connectionDestination(connection: RuntimeConnection): string {
  return formatAddress(connection.metadata.destinationIP, connection.metadata.destinationPort);
}

export function connectionRule(connection: RuntimeConnection): string {
  if (!connection.rule) return "--";
  return connection.rulePayload ? `${connection.rule} · ${connection.rulePayload}` : connection.rule;
}

export function connectionChains(connection: RuntimeConnection): string {
  return connection.chains.length ? [...connection.chains].reverse().join(" › ") : "--";
}

export function formatStarted(value: string): string {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return value || "--";
  return date.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

export function formatDuration(start: string, now: number, end?: string): string {
  const startedAt = new Date(start).getTime();
  const endedAt = end ? new Date(end).getTime() : now;
  if (!Number.isFinite(startedAt) || !Number.isFinite(endedAt)) return "--";
  const seconds = Math.max(0, Math.floor((endedAt - startedAt) / 1000));
  if (seconds < 60) return `${seconds} 秒`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分 ${seconds % 60} 秒`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时 ${minutes % 60} 分`;
  return `${Math.floor(hours / 24)} 天 ${hours % 24} 小时`;
}

export function metadataValue(value: unknown): string {
  if (value === null || value === undefined || value === "") return "--";
  if (Array.isArray(value)) return value.map(text).join(", ") || "--";
  if (typeof value === "object") {
    try { return JSON.stringify(value); } catch { return String(value); }
  }
  return text(value);
}

export function connectionSearchText(connection: DisplayConnection): string {
  return [
    connection.id,
    connection.rule,
    connection.rulePayload,
    ...connection.chains,
    ...Object.values(connection.metadata).map(metadataValue)
  ].join("\n").toLocaleLowerCase();
}
