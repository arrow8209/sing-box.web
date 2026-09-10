import { describe, expect, it } from "vitest";
import { createRuleKeyResolver } from "@/features/config-editor/rule-keys";

describe("规则列表行标识", () => {
  it("启停、重排和 JSON 格式变化后仍识别同一条规则", () => {
    const resolve = createRuleKeyResolver();
    const first = { text: '{"protocol":"dns"}', enabled: true };
    const second = { text: '{"protocol":"quic"}', enabled: true };
    const initial = resolve([first, second]);
    const next = resolve([second, { text: '{\n  "protocol": "dns"\n}', enabled: false }]);

    expect(next.map((item) => item.key)).toEqual([initial[1]!.key, initial[0]!.key]);
    expect(next[1]!.enabled).toBe(false);
  });

  it("内容相同的多条规则有独立标识，启停其中一条不改变相邻标识", () => {
    const resolve = createRuleKeyResolver();
    const text = '{"domain_suffix":"example.test"}';
    const initial = resolve([{ text, enabled: true }, { text, enabled: true }]);
    const next = resolve([{ text, enabled: false }, { text, enabled: true }]);

    expect(new Set(initial.map((item) => item.key)).size).toBe(2);
    expect(next.map((item) => item.key)).toEqual(initial.map((item) => item.key));
  });

  it("识别规则时保留大整数和字符串内空白的差异", () => {
    const resolve = createRuleKeyResolver();
    const rules = [
      { text: '{"future":9007199254740992}' },
      { text: '{"future":9007199254740993}' },
      { text: '{"process_name":"a b"}' },
      { text: '{"process_name":"ab"}' }
    ];
    const initial = resolve(rules);
    const next = resolve([...rules].reverse());

    expect(new Set(initial.map((item) => item.key)).size).toBe(4);
    expect(next.map((item) => item.key)).toEqual(initial.map((item) => item.key).reverse());
  });
});
