export interface MissingReference {
  kind: "outbound" | "route-final";
  tag: string;
}

export interface ReferenceReport {
  tags: Set<string>;
  missing: MissingReference[];
  duplicates: string[];
}

/**
 * 收集已有出站 tag 集合与缺失引用/重复标签标注；只读，不修改配置。
 */
export function collectReferences(config: Record<string, unknown>): ReferenceReport {
  const tags = new Set<string>();
  const seen = new Set<string>();
  const duplicates: string[] = [];
  const missing: MissingReference[] = [];

  const outbounds = Array.isArray(config.outbounds) ? (config.outbounds as unknown[]) : [];
  for (const item of outbounds) {
    if (item === null || typeof item !== "object") {
      continue;
    }
    const tag = (item as Record<string, unknown>).tag;
    if (typeof tag !== "string") {
      continue;
    }
    if (seen.has(tag) && !duplicates.includes(tag)) {
      duplicates.push(tag);
    }
    seen.add(tag);
    tags.add(tag);
  }

  for (const item of outbounds) {
    if (item === null || typeof item !== "object") {
      continue;
    }
    const references = (item as Record<string, unknown>).outbounds;
    if (!Array.isArray(references)) {
      continue;
    }
    for (const reference of references) {
      if (typeof reference === "string" && !tags.has(reference)) {
        missing.push({ kind: "outbound", tag: reference });
      }
    }
  }

  const route = config.route;
  if (route !== null && typeof route === "object") {
    const final = (route as Record<string, unknown>).final;
    if (typeof final === "string" && !tags.has(final)) {
      missing.push({ kind: "route-final", tag: final });
    }
  }

  return { tags, missing, duplicates };
}
