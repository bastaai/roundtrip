// JSON split into coloured runs for the editors, tolerating what is not valid
// yet. Joining the texts gives back the input exactly.

export type Run = { text: string; kind?: 'key' | 'str' | 'num' | 'lit' | 'var'; name?: string };

const VAR = /\{\{\s*([^{}\s]+)\s*\}\}/g;

export function highlightJson(src: string): Run[] {
  const out: Run[] = [];
  const push = (text: string, kind?: Run['kind']) => {
    const last = out[out.length - 1];
    if (last && !kind && !last.kind) last.text += text;
    else out.push({ text, kind });
  };
  // {{variables}} inside a string are runs of their own.
  const pushString = (text: string, kind: 'key' | 'str') => {
    let at = 0;
    for (const m of text.matchAll(VAR)) {
      if (m.index > at) push(text.slice(at, m.index), kind);
      out.push({ text: m[0], kind: 'var', name: m[1] });
      at = m.index + m[0].length;
    }
    if (at < text.length) push(text.slice(at), kind);
  };
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (c === '"') {
      let j = i + 1;
      while (j < src.length && src[j] !== '"' && src[j] !== '\n') j += src[j] === '\\' ? 2 : 1;
      if (src[j] === '"') j++;
      j = Math.min(j, src.length);
      // A string followed by a colon is a key.
      let k = j;
      while (k < src.length && (src[k] === ' ' || src[k] === '\t')) k++;
      pushString(src.slice(i, j), src[k] === ':' ? 'key' : 'str');
      i = j;
    } else if (/[-\d]/.test(c)) {
      const m = /^-?\d+(\.\d+)?([eE][+-]?\d+)?/.exec(src.slice(i));
      const len = m?.[0].length || 1;
      push(src.slice(i, i + len), m ? 'num' : undefined);
      i += len;
    } else if (/[a-z]/i.test(c)) {
      const m = /^[a-z_]\w*/i.exec(src.slice(i))!;
      push(m[0], ['true', 'false', 'null'].includes(m[0]) ? 'lit' : undefined);
      i += m[0].length;
    } else {
      push(c);
      i++;
    }
  }
  return out;
}
