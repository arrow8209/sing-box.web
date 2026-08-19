export type JSONValue =
  | string
  | number
  | boolean
  | null
  | JSONValue[]
  | { [key: string]: JSONValue };

export type ConfigObject = Record<string, unknown>;

const FORBIDDEN_SEGMENTS = new Set(["__proto__", "prototype", "constructor"]);

function assertSafePath(path: readonly string[]): void {
  for (const segment of path) {
    if (FORBIDDEN_SEGMENTS.has(segment)) {
      throw new Error(`path segment not allowed: ${segment}`);
    }
  }
}

function clone(value: unknown): unknown {
  if (value === null || typeof value !== "object") {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map(clone);
  }
  const result: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    result[key] = clone(entry);
  }
  return result;
}

export function getAtPath(source: unknown, path: readonly string[]): unknown {
  assertSafePath(path);
  let current = source;
  for (const segment of path) {
    if (current === null || typeof current !== "object") {
      return undefined;
    }
    current = (current as Record<string, unknown>)[segment];
  }
  return current;
}

/**
 * 返回一份新对象：按路径设置值，不修改原对象，也不丢失路径之外的未知字段。
 * 值为 undefined 时删除该可选字段。
 */
export function setAtPath(
  source: ConfigObject,
  path: readonly string[],
  value: unknown,
): ConfigObject {
  assertSafePath(path);
  const result = clone(source) as ConfigObject;
  if (path.length === 0) {
    return value as ConfigObject;
  }
  let container: ConfigObject | unknown[] = result;
  for (let index = 0; index < path.length - 1; index++) {
    const segment = path[index];
    const next = Array.isArray(container)
      ? container[Number(segment)]
      : (container as ConfigObject)[segment];
    if (next === null || typeof next !== "object") {
      // 空容器：按下一段是否为数字推断对象或数组。
      const nextIsIndex = /^\d+$/.test(path[index + 1]);
      const created: unknown = nextIsIndex ? [] : {};
      if (Array.isArray(container)) {
        container[Number(segment)] = created;
      } else {
        (container as ConfigObject)[segment] = created;
      }
      container = created as ConfigObject | unknown[];
    } else {
      const cloned = clone(next) as ConfigObject | unknown[];
      if (Array.isArray(container)) {
        container[Number(segment)] = cloned;
      } else {
        (container as ConfigObject)[segment] = cloned;
      }
      container = cloned;
    }
  }
  const last = path[path.length - 1];
  if (value === undefined) {
    if (Array.isArray(container)) {
      container.splice(Number(last), 1);
    } else {
      delete container[last];
    }
  } else if (Array.isArray(container)) {
    container[Number(last)] = clone(value);
  } else {
    container[last] = clone(value);
  }
  return result;
}

export function deleteAtPath(source: ConfigObject, path: readonly string[]): ConfigObject {
  return setAtPath(source, path, undefined);
}
