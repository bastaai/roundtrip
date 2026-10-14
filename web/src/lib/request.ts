import type { HistoryEntry, KV, Kind, Req } from '../types';

export const uid = () => crypto.randomUUID();

export const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];

export const emptyRequest = (): Req => ({
  method: 'GET',
  url: '',
  headers: [],
  auth: { type: 'none', token: '', username: '', password: '', key: '', value: '', in: 'header' },
  body: { mode: 'none', raw: '', contentType: 'application/json', form: [] },
  tests: [],
  extract: [],
});

// Fills in whatever a hand-written or imported request file left out.
export function normalizeReq(r?: Partial<Req> | null): Req {
  const base = emptyRequest();
  const auth = { ...base.auth, ...r?.auth };
  const body = { ...base.body, ...r?.body };
  return {
    protocol: r?.protocol,
    method: r?.method || (r?.protocol === 'grpc' ? '' : base.method),
    url: r?.url ?? '',
    headers: r?.headers ?? [],
    auth: { ...auth, type: auth.type || 'none', in: auth.in || 'header' },
    body: { ...body, mode: body.mode || 'none', form: body.form ?? [] },
    tests: r?.tests ?? [],
    extract: r?.extract ?? [],
    docs: r?.docs || undefined,
  };
}

export const kindOf = (r: Pick<Req, 'protocol'> & { body?: Pick<Req['body'], 'mode'> }): Kind =>
  r.protocol === 'grpc' ? 'grpc' : r.body?.mode === 'graphql' ? 'graphql' : 'http';

// Switching kind keeps the URL, headers and auth and resets what can't carry over.
export function setKind(r: Req, kind: Kind): Req {
  const from = kindOf(r);
  if (from === kind) return r;
  const method = from === 'grpc' ? 'GET' : r.method;
  const body = from === 'graphql' ? { ...r.body, mode: 'none' as const, raw: '', variables: undefined } : r.body;
  switch (kind) {
    case 'grpc':
      return { ...r, protocol: 'grpc', method: '', body: { ...body, mode: 'raw', raw: from === 'http' && body.mode === 'raw' ? body.raw : '{}' } };
    case 'graphql':
      return { ...r, protocol: undefined, method: 'POST', body: { ...body, mode: 'graphql', raw: '', variables: '' } };
    case 'http':
      return { ...r, protocol: undefined, method, body: from === 'grpc' ? { ...body, mode: 'none', raw: '' } : body };
  }
}

function splitUrl(url: string) {
  const h = url.indexOf('#');
  const hash = h >= 0 ? url.slice(h) : '';
  const rest = h >= 0 ? url.slice(0, h) : url;
  const q = rest.indexOf('?');
  return { base: q >= 0 ? rest.slice(0, q) : rest, query: q >= 0 ? rest.slice(q + 1) : '', hash };
}

// Query params are a view over the URL. Nothing is decoded or encoded, so
// {{variables}} and whatever the user typed survive untouched.
export function parseQuery(url: string): KV[] {
  const { query } = splitUrl(url);
  if (!query) return [];
  return query.split('&').map((pair) => {
    const i = pair.indexOf('=');
    return { key: i >= 0 ? pair.slice(0, i) : pair, value: i >= 0 ? pair.slice(i + 1) : '', enabled: true };
  });
}

export function setQuery(url: string, rows: KV[]): string {
  const { base, hash } = splitUrl(url);
  const query = rows
    .filter((r) => r.key || r.value)
    .map((r) => (r.value ? `${r.key}=${r.value}` : r.key))
    .join('&');
  return base + (query ? '?' + query : '') + hash;
}

// "git@github.com:acme/api-requests.git" -> "acme/api-requests"
export function repoName(remote: string): string {
  const parts = remote.replace(/\/+$/, '').replace(/\.git$/, '').split(/[/:\\]/).filter(Boolean);
  return parts.slice(-2).join('/');
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export const formatTime = (ms: number) => (ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`);

const sameDay = (a: Date, b: Date) => a.toDateString() === b.toDateString();

// "14:02:11 · 200 · 38 ms" for today, with the date in front for earlier days.
export function runLabel(h: HistoryEntry, now = new Date()): string {
  const at = new Date(h.at);
  const time = at.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  const when = sameDay(at, now) ? time : `${at.toLocaleDateString([], { day: 'numeric', month: 'short' })}, ${time}`;
  const grpc = h.request.protocol === 'grpc';
  const status = h.error ? 'Error' : grpc && h.status === 0 ? 'OK' : String(h.status);
  return `${when} · ${status} · ${formatTime(h.timeMs)}`;
}

export function prettyJson(text: string): string | null {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return null;
  }
}

