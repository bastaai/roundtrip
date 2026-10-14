import { describe, expect, it } from 'vitest';
import { emptyRequest, formatSize, runLabel, repoName, kindOf, normalizeReq, parseQuery, setKind, setQuery } from './request';

const kv = (key: string, value: string) => ({ key, value, enabled: true });

describe('query params', () => {
  it('parses without decoding', () => {
    expect(parseQuery('https://x/a?b=1&c=%20d&e&f={{var}}#frag')).toEqual([
      kv('b', '1'),
      kv('c', '%20d'),
      kv('e', ''),
      kv('f', '{{var}}'),
    ]);
    expect(parseQuery('https://x/a')).toEqual([]);
    expect(parseQuery('https://x/a#b?c=1')).toEqual([]);
  });

  it('writes back keeping base and fragment', () => {
    expect(setQuery('{{base}}/a?old=1#frag', [kv('x', '1'), kv('flag', ''), kv('', '')])).toBe('{{base}}/a?x=1&flag#frag');
    expect(setQuery('https://x/a?old=1', [])).toBe('https://x/a');
  });

  it('round trips', () => {
    const url = 'https://x/a?b=1&c=two%20words&d={{v}}';
    expect(setQuery(url, parseQuery(url))).toBe(url);
  });
});

describe('normalizeReq', () => {
  it('fills in missing fields', () => {
    const r = normalizeReq({ url: 'https://x', body: { mode: 'raw', raw: '{}' } as never });
    expect(r.method).toBe('GET');
    expect(r.headers).toEqual([]);
    expect(r.auth.type).toBe('none');
    expect(r.body).toEqual({ mode: 'raw', raw: '{}', contentType: 'application/json', form: [] });
    expect(r.tests).toEqual([]);
    expect(r.extract).toEqual([]);
  });

  it('survives nulls from the server', () => {
    const r = normalizeReq({ method: '', headers: null, body: { form: null } } as never);
    expect(r.method).toBe('GET');
    expect(r.headers).toEqual([]);
    expect(r.body.form).toEqual([]);
    expect(r.body.mode).toBe('none');
  });

  it('keeps grpc requests intact', () => {
    const r = normalizeReq({ protocol: 'grpc', method: 'a.B/C', url: 'grpc://x', body: { mode: 'raw', raw: '{}' } } as never);
    expect(r.protocol).toBe('grpc');
    expect(r.method).toBe('a.B/C');
    expect(normalizeReq({ protocol: 'grpc', url: 'x' } as never).method).toBe('');
  });
});

describe('kind', () => {
  it('is derived from protocol and body mode', () => {
    expect(kindOf(emptyRequest())).toBe('http');
    expect(kindOf({})).toBe('http');
    expect(kindOf({ body: { mode: 'graphql' } })).toBe('graphql');
    expect(kindOf({ protocol: 'grpc', body: { mode: 'raw' } })).toBe('grpc');
  });

  it('switches to graphql and back', () => {
    const http = { ...emptyRequest(), url: 'https://x/graphql', headers: [kv('A', 'b')] };
    const gql = setKind(http, 'graphql');
    expect(gql).toMatchObject({ method: 'POST', url: 'https://x/graphql', headers: [kv('A', 'b')], body: { mode: 'graphql' } });
    const back = setKind({ ...gql, body: { ...gql.body, raw: '{ me }' } }, 'http');
    expect(back.body).toMatchObject({ mode: 'none', raw: '' });
    expect(back.method).toBe('POST');
    expect(setKind(http, 'http')).toBe(http);
  });

  it('switches to grpc and back', () => {
    const grpc = setKind({ ...emptyRequest(), method: 'DELETE', url: 'localhost:50051' }, 'grpc');
    expect(grpc).toMatchObject({ protocol: 'grpc', method: '', url: 'localhost:50051', body: { mode: 'raw', raw: '{}' } });
    const back = setKind({ ...grpc, method: 'a.B/C' }, 'http');
    expect(back.protocol).toBeUndefined();
    expect(back.method).toBe('GET');
    expect(back.body.mode).toBe('none');
  });
});

it('repoName', () => {
  expect(repoName('git@github.com:acme/api-requests.git')).toBe('acme/api-requests');
  expect(repoName('https://github.com/acme/api-requests/')).toBe('acme/api-requests');
  expect(repoName('/tmp/remotes/requests.git')).toBe('remotes/requests');
  expect(repoName('')).toBe('');
});

it('formatSize', () => {
  expect(formatSize(512)).toBe('512 B');
  expect(formatSize(2048)).toBe('2.0 KB');
  expect(formatSize(5 * 1024 * 1024)).toBe('5.0 MB');
});

it('runLabel', () => {
  const now = new Date(2026, 9, 14, 15, 0, 0);
  const at = (d: Date) => d.toISOString();
  const entry = { id: 'h', request: emptyRequest(), status: 201, timeMs: 38 };
  const time = (d: Date) => d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  const earlier = new Date(2026, 9, 14, 14, 2, 11);
  expect(runLabel({ ...entry, at: at(earlier) }, now)).toBe(`${time(earlier)} · 201 · 38 ms`);
  const before = new Date(2026, 9, 12, 9, 5, 0);
  expect(runLabel({ ...entry, at: at(before), timeMs: 1500 }, now)).toBe(
    `${before.toLocaleDateString([], { day: 'numeric', month: 'short' })}, ${time(before)} · 201 · 1.50 s`,
  );
  expect(runLabel({ ...entry, at: at(now), error: 'refused' }, now)).toContain('· Error ·');
  expect(runLabel({ ...entry, at: at(now), status: 0, request: { ...emptyRequest(), protocol: 'grpc' } }, now)).toContain('· OK ·');
});
