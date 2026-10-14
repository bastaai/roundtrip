import { describe, expect, it } from 'vitest';
import { foldAll, jsonLines, toggleFold, visibleLines, type JsonLine } from './json';

const render = (l: JsonLine) => '  '.repeat(l.depth) + (l.key ? `${l.key}: ` : '') + l.text + (l.comma ? ',' : '');

const sample = { id: 1, name: 'a "b"', tags: ['x', 'y'], none: null, empty: {}, list: [], nested: { ok: true, n: -1.5e30 } };

describe('jsonLines', () => {
  it('lays out exactly like JSON.stringify with two spaces', () => {
    for (const v of [sample, [1, [2, [3]]], 'text', 42, null, {}, [], { '2': 'b', '1': 'a', z: 0 }]) {
      expect(jsonLines(v).map(render).join('\n')).toBe(JSON.stringify(v, null, 2));
    }
  });

  it('links each open line to its close and counts its entries', () => {
    const lines = jsonLines(sample);
    const tags = lines.findIndex((l) => l.key === '"tags"');
    expect(lines[tags]).toMatchObject({ kind: 'open', text: '[', size: 2 });
    expect(lines[lines[tags].end!]).toMatchObject({ kind: 'close', text: ']', comma: true });
    expect(lines[0].end).toBe(lines.length - 1);
    expect(lines.find((l) => l.key === '"empty"')).toMatchObject({ kind: 'empty', text: '{}' });
  });
});

describe('folding', () => {
  const lines = jsonLines({ a: { b: { c: 1 } }, d: 2 });
  // 0 {   1 "a": {   2 "b": {   3 "c": 1   4 }   5 },   6 "d": 2   7 }

  it('hides what a folded line contains', () => {
    expect(visibleLines(lines, new Set())).toEqual([0, 1, 2, 3, 4, 5, 6, 7]);
    expect(visibleLines(lines, new Set([1]))).toEqual([0, 1, 6, 7]);
    expect(visibleLines(lines, new Set([0]))).toEqual([0]);
  });

  it('toggles one line, or everything inside it when deep', () => {
    expect([...toggleFold(lines, new Set(), 1)]).toEqual([1]);
    expect([...toggleFold(lines, new Set([1, 2]), 1)]).toEqual([2]);
    expect([...toggleFold(lines, new Set(), 1, true)].sort()).toEqual([1, 2]);
    expect([...toggleFold(lines, new Set([1, 2]), 1, true)]).toEqual([]);
  });

  it('folds all but the top level', () => {
    expect([...foldAll(lines)].sort()).toEqual([1, 2]);
  });
});
