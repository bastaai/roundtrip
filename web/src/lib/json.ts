// A parsed JSON value laid out as the lines of JSON.stringify(value, null, 2),
// with every object and array that spans lines knowing where it ends, so the
// response view can fold it.

export type JsonLine = {
  depth: number;
  key?: string; // JSON-encoded, quotes included
  kind: 'str' | 'num' | 'lit' | 'empty' | 'open' | 'close';
  text: string; // the value, or the bracket
  comma: boolean;
  end?: number; // open lines: index of the matching close line
  size?: number; // open lines: number of keys or items
};

export function jsonLines(value: unknown): JsonLine[] {
  const out: JsonLine[] = [];
  const walk = (v: unknown, depth: number, key: string | undefined, comma: boolean) => {
    if (v !== null && typeof v === 'object') {
      const array = Array.isArray(v);
      const entries: [string | undefined, unknown][] = array ? v.map((x) => [undefined, x]) : Object.entries(v).map(([k, x]) => [JSON.stringify(k), x]);
      const [open, close] = array ? ['[', ']'] : ['{', '}'];
      if (entries.length === 0) {
        out.push({ depth, key, kind: 'empty', text: open + close, comma });
        return;
      }
      const at = out.push({ depth, key, kind: 'open', text: open, comma: false, size: entries.length }) - 1;
      entries.forEach(([k, x], i) => walk(x, depth + 1, k, i < entries.length - 1));
      out[at].end = out.push({ depth, kind: 'close', text: close, comma }) - 1;
      return;
    }
    const kind = typeof v === 'string' ? 'str' : typeof v === 'number' ? 'num' : 'lit';
    out.push({ depth, key, kind, text: JSON.stringify(v), comma });
  };
  walk(value, 0, undefined, false);
  return out;
}

// Indexes of the lines to show: a folded line stands in for everything up to its close.
export function visibleLines(lines: JsonLine[], folded: ReadonlySet<number>): number[] {
  const out: number[] = [];
  for (let i = 0; i < lines.length; i++) {
    out.push(i);
    if (folded.has(i)) i = lines[i].end!;
  }
  return out;
}

// Folds or unfolds line i. Deep applies the same to everything inside it.
export function toggleFold(lines: JsonLine[], folded: ReadonlySet<number>, i: number, deep = false): Set<number> {
  const next = new Set(folded);
  const fold = !folded.has(i);
  const last = deep ? lines[i].end! : i;
  for (let j = i; j <= last; j++) {
    if (lines[j].kind !== 'open') continue;
    if (fold) next.add(j);
    else next.delete(j);
  }
  return next;
}

// Every line that can fold below the top level, for "collapse all".
export function foldAll(lines: JsonLine[]): Set<number> {
  const out = new Set<number>();
  lines.forEach((l, i) => l.kind === 'open' && l.depth > 0 && out.add(i));
  return out;
}
