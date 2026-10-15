import type { GrpcEnum } from '../types';

// Enum values to offer at the caret of a gRPC message being typed.
// from..to is the text a chosen value replaces: the inside of the string the
// caret is in, or nothing when the caret is where a value is still missing.
export type EnumCompletion = {
  from: number;
  to: number;
  quoted: boolean; // the caret is inside a string
  closed: boolean; // that string has its closing quote
  prefix: string; // what is typed before the caret
  values: string[]; // matching values, those starting with prefix first
};

type Frame = { kind: 'obj'; key?: string; state: 'key' | 'colon' | 'value' | 'after' } | { kind: 'arr'; index: number; state: 'value' | 'after' };

const DELIM = /[\s,:{}[\]"]/;

// Reads the message up to the caret, tolerating what is not valid JSON yet,
// to find where the caret is. Null when it is not at an enum field's value.
export function enumCompletion(raw: string, caret: number, enums: GrpcEnum[]): EnumCompletion | null {
  if (!enums.length) return null;
  const stack: Frame[] = [];
  const top = () => stack[stack.length - 1];
  const done = () => {
    const f = top();
    if (f) f.state = 'after';
  };
  const inValue = () => {
    const f = top();
    return !!f && f.state === 'value';
  };
  const offer = (from: number, to: number, quoted: boolean, closed: boolean) => {
    const path = stack.map((f) => (f.kind === 'obj' ? (f.key ?? '') : String(f.index)));
    const e = enums.find((x) => x.path.length === path.length && x.path.every((seg, i) => seg === '*' || seg === path[i]));
    if (!e) return null;
    const prefix = raw.slice(from, caret);
    const p = prefix.toLowerCase();
    const starts = e.values.filter((v) => v.toLowerCase().startsWith(p));
    const values = [...starts, ...e.values.filter((v) => !starts.includes(v) && v.toLowerCase().includes(p))];
    return values.length ? { from, to, quoted, closed, prefix, values } : null;
  };

  let i = 0;
  while (i < caret) {
    const c = raw[i];
    if (/\s/.test(c)) {
      i++;
    } else if (c === '{') {
      stack.push({ kind: 'obj', state: 'key' });
      i++;
    } else if (c === '[') {
      stack.push({ kind: 'arr', index: 0, state: 'value' });
      i++;
    } else if (c === '}' || c === ']') {
      stack.pop();
      done();
      i++;
    } else if (c === ',') {
      const f = top();
      if (f?.kind === 'obj') {
        f.state = 'key';
        f.key = undefined;
      } else if (f) {
        f.index++;
        f.state = 'value';
      }
      i++;
    } else if (c === ':') {
      const f = top();
      if (f?.kind === 'obj') f.state = 'value';
      i++;
    } else if (c === '"') {
      let j = i + 1;
      while (j < raw.length && raw[j] !== '"' && raw[j] !== '\n') j += raw[j] === '\\' ? 2 : 1;
      const closed = raw[j] === '"';
      if (caret <= j) return inValue() ? offer(i + 1, j, true, closed) : null;
      const f = top();
      if (f?.kind === 'obj' && f.state === 'key') {
        try {
          f.key = JSON.parse(raw.slice(i, j + 1));
        } catch {
          f.key = raw.slice(i + 1, j);
        }
        f.state = 'colon';
      } else {
        done();
      }
      i = j + 1;
    } else {
      // A number or literal; the caret inside one is not offered anything.
      let j = i;
      while (j < raw.length && !DELIM.test(raw[j])) j++;
      if (caret <= j) return null;
      done();
      i = j;
    }
  }
  return inValue() ? offer(caret, caret, false, false) : null;
}

// The message with value put in place, and where the caret goes: after the closing quote.
export function applyCompletion(raw: string, c: EnumCompletion, value: string): { text: string; caret: number } {
  const insert = c.quoted ? value + (c.closed ? '' : '"') : `"${value}"`;
  const text = raw.slice(0, c.from) + insert + raw.slice(c.to);
  return { text, caret: c.from + insert.length + (c.quoted && c.closed ? 1 : 0) };
}
