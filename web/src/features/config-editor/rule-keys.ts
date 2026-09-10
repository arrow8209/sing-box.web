import { createScanner } from "jsonc-parser";

function ruleSignature(text: string): string {
  const scanner = createScanner(text, true);
  const tokens: string[] = [];
  for (scanner.scan(); scanner.getTokenLength() > 0; scanner.scan()) {
    tokens.push(text.slice(scanner.getTokenOffset(), scanner.getTokenOffset() + scanner.getTokenLength()));
  }
  return tokens.join("");
}

// Resolve against the complete visible list, including disabled rules. Active
// array indexes shift on toggles and must not identify native checkbox nodes.
export function createRuleKeyResolver() {
  let sequence = 0;
  let previous = new Map<string, string[]>();

  return function resolveRuleKeys<T extends { text: string }>(items: readonly T[]): Array<T & { key: string }> {
    const current = new Map<string, string[]>();
    const result = items.map((item) => {
      const signature = ruleSignature(item.text);
      const keys = current.get(signature) ?? [];
      const key = previous.get(signature)?.[keys.length] ?? `rule-view-${++sequence}`;
      keys.push(key);
      current.set(signature, keys);
      return { ...item, key };
    });
    previous = current;
    return result;
  };
}
