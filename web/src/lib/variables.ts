import type { Environment } from '../types';

export type Lookup = { value: string; secret: boolean; from: 'response' | 'environment' };

// What {{name}} becomes when the request is sent: a value saved from a
// response wins over the environment's, as on the server.
export function lookupVariable(name: string, env: Environment | undefined, runtime: Record<string, string>): Lookup | undefined {
  if (Object.hasOwn(runtime, name)) return { value: runtime[name], secret: false, from: 'response' };
  let found: Lookup | undefined;
  for (const v of env?.variables ?? []) {
    if (v.enabled && v.key === name) found = { value: v.value, secret: !!v.secret, from: 'environment' };
  }
  return found;
}

// env with {{name}} set to value: the row that wins (the last enabled one) is
// changed, or an enabled row is added when there is none.
export function withVariable(env: Environment, name: string, value: string): Environment {
  const i = env.variables.findLastIndex((v) => v.enabled && v.key === name);
  const variables = i >= 0 ? env.variables.map((v, j) => (j === i ? { ...v, value } : v)) : [...env.variables, { key: name, value, enabled: true }];
  return { ...env, variables };
}

// The {{variable}} that covers character i of text, if any.
export function variableAt(text: string, i: number): { name: string; start: number; end: number } | undefined {
  for (const m of text.matchAll(/\{\{\s*([^{}\s]+)\s*\}\}/g)) {
    if (i >= m.index && i < m.index + m[0].length) return { name: m[1], start: m.index, end: m.index + m[0].length };
  }
}
