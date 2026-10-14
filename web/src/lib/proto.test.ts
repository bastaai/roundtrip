import { describe, expect, it } from 'vitest';
import type { Collection } from '../types';
import { checkSummary, cleanPaths, grpcSource, groupMethods, normalizeProto, protoOf, sameProto, withProto } from './proto';

const col = (id: string, proto?: Collection['proto']): Collection => ({ id, name: id, items: [], ...(proto ? { proto } : {}) });

describe('proto config', () => {
  it('trims paths and drops blank and repeated rows, keeping order', () => {
    expect(cleanPaths([' protos ', '', '  ', '{{protoRoot}}/third_party', 'protos'])).toEqual(['protos', '{{protoRoot}}/third_party']);
    expect(cleanPaths(undefined)).toEqual([]);
    expect(cleanPaths(null)).toEqual([]);
  });

  it('treats a missing or blank config as none', () => {
    expect(normalizeProto(undefined)).toBeUndefined();
    expect(normalizeProto(null)).toBeUndefined();
    expect(normalizeProto({})).toBeUndefined();
    expect(normalizeProto({ roots: [], files: [' '] })).toBeUndefined();
  });

  it('omits the empty list', () => {
    expect(normalizeProto({ roots: ['protos '], files: [] })).toEqual({ roots: ['protos'] });
    expect(normalizeProto({ files: ['a.proto'] })).toEqual({ files: ['a.proto'] });
    expect(JSON.stringify({ proto: normalizeProto({ roots: [''] }) })).toBe('{}');
  });

  it('compares configs after normalising', () => {
    expect(sameProto(undefined, { roots: [' '] })).toBe(true);
    expect(sameProto({ roots: ['a'] }, { roots: ['a '], files: [] })).toBe(true);
    expect(sameProto({ roots: ['a', 'b'] }, { roots: ['b', 'a'] })).toBe(false);
    expect(sameProto({ roots: ['a'] }, { files: ['a'] })).toBe(false);
  });

  it('sets and removes the config on a collection', () => {
    const set = withProto(col('c'), { roots: [' protos'], files: [''] });
    expect(set).toEqual({ id: 'c', name: 'c', items: [], proto: { roots: ['protos'] } });
    const cleared = withProto(set, { roots: [], files: [] });
    expect('proto' in cleared).toBe(false);
    expect(cleared).toEqual(col('c'));
    expect(set.proto).toEqual({ roots: ['protos'] });
  });

  it('finds where a tab gets its methods from', () => {
    const collections = [col('plain'), col('blank', { roots: [' '] }), col('protos', { roots: ['protos'], files: ['a.proto '] })];
    expect(grpcSource(collections, undefined)).toEqual({ kind: 'unsaved' });
    expect(grpcSource(collections, { collectionId: 'gone' })).toEqual({ kind: 'unsaved' });
    expect(grpcSource(collections, { collectionId: 'plain' })).toEqual({ kind: 'reflection' });
    expect(grpcSource(collections, { collectionId: 'blank' })).toEqual({ kind: 'reflection' });
    const source = grpcSource(collections, { collectionId: 'protos' });
    expect(source).toEqual({ kind: 'proto', proto: { roots: ['protos'], files: ['a.proto'] } });
    expect(protoOf(source)).toEqual({ roots: ['protos'], files: ['a.proto'] });
    expect(protoOf({ kind: 'reflection' })).toBeUndefined();
    expect(protoOf({ kind: 'unsaved' })).toBeUndefined();
  });
});

describe('proto check result', () => {
  it('summarises counts with plurals', () => {
    expect(checkSummary({ files: 12, services: ['a.A', 'a.B'], methods: ['a.A/X'] })).toBe('12 files · 2 services · 1 method');
    expect(checkSummary({ files: 1, services: ['a.A'], methods: [] })).toBe('1 file · 1 service · 0 methods');
    expect(checkSummary({})).toBe('0 files · 0 services · 0 methods');
  });

  it('groups methods under their service', () => {
    expect(groupMethods({ services: ['a.Users', 'a.Empty'], methods: ['a.Users/Get', 'a.Users/List', 'b.Other/Ping'] })).toEqual([
      { service: 'a.Users', methods: ['Get', 'List'] },
      { service: 'a.Empty', methods: [] },
      { service: 'b.Other', methods: ['Ping'] },
    ]);
    expect(groupMethods({})).toEqual([]);
  });
});
