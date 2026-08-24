import type { SubscriptionJsonValidation, SubscriptionOutbound } from "./types";

function isObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

export function validateSubscriptionJson(source: string): SubscriptionJsonValidation {
  let parsed: unknown;
  try {
    parsed = JSON.parse(source) as unknown;
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : "无法解析 JSON";
    return { valid: false, message: `JSON 语法错误：${message}` };
  }

  const nodes = Array.isArray(parsed)
    ? parsed
    : isObject(parsed) && Array.isArray(parsed.outbounds)
      ? parsed.outbounds
      : isObject(parsed) && Array.isArray(parsed.nodes)
        ? parsed.nodes
        : null;
  if (!nodes) return { valid: false, message: "JSON 必须是节点数组，或包含 outbounds 数组的完整配置对象。" };
  if (!nodes.length) return { valid: false, message: "JSON 中没有可导入的节点，请至少添加一个出站对象。" };
  if (nodes.length > 10000) return { valid: false, message: "单个订阅最多包含 10000 个节点。" };

  const outbounds: SubscriptionOutbound[] = [];
  const tags = new Set<string>();
  const protocols = new Set<string>();
  for (let index = 0; index < nodes.length; index++) {
    const node = nodes[index];
    if (!isObject(node) || typeof node.type !== "string" || !node.type.trim() || typeof node.tag !== "string" || !node.tag.trim()) {
      return { valid: false, message: `第 ${index + 1} 个节点必须是包含非空 type 和 tag 的对象。` };
    }
    const type = node.type.trim();
    const tag = node.tag.trim();
    if (tags.has(tag)) return { valid: false, message: `节点标签不能重复：${tag}` };
    tags.add(tag);
    protocols.add(type);
    outbounds.push({ ...node, type, tag });
  }

  const name = isObject(parsed) && typeof parsed.name === "string" ? parsed.name.trim() : "";
  if (Array.from(name).length > 80) return { valid: false, message: "订阅名称不能超过 80 个字符。" };
  return {
    valid: true,
    parsed,
    name,
    outbounds,
    message: `JSON 有效 · 将导入 ${outbounds.length} 个节点 · ${protocols.size} 种协议`
  };
}
