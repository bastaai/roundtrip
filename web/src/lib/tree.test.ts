import { describe, expect, it } from 'vitest';
import type { Item } from '../types';
import { emptyRequest } from './request';
import { addItem, findItem, listFolders, mapItem, removeItem, sortItems } from './tree';

const req = (id: string, name = id): Item => ({ id, name, request: emptyRequest() });
const folder = (id: string, items: Item[] = [], name = id): Item => ({ id, name, items });

const tree = (): Item[] => [folder('f1', [req('r2'), folder('f2', [req('r3')])]), req('r1')];

describe('tree', () => {
  it('finds nested items', () => {
    expect(findItem(tree(), 'r3')?.id).toBe('r3');
    expect(findItem(tree(), 'nope')).toBeUndefined();
  });

  it('maps a nested item without touching the original', () => {
    const before = tree();
    const after = mapItem(before, 'r3', (i) => ({ ...i, name: 'renamed' }));
    expect(findItem(after, 'r3')?.name).toBe('renamed');
    expect(findItem(before, 'r3')?.name).toBe('r3');
  });

  it('removes items and whole folders', () => {
    expect(findItem(removeItem(tree(), 'r3'), 'r3')).toBeUndefined();
    const withoutFolder = removeItem(tree(), 'f1');
    expect(withoutFolder.map((i) => i.id)).toEqual(['r1']);
  });

  it('adds at the root or inside a folder', () => {
    expect(addItem(tree(), null, req('new')).map((i) => i.id)).toEqual(['f1', 'r1', 'new']);
    const nested = addItem(tree(), 'f2', req('new'));
    expect(findItem(nested, 'f2')?.items?.map((i) => i.id)).toEqual(['r3', 'new']);
  });

  it('sorts folders first, then by name, recursively', () => {
    const sorted = sortItems([req('1', 'beta'), folder('2', [req('3', 'z'), req('4', 'A')], 'zeta'), req('5', 'Alpha'), folder('6', [], 'eta')]);
    expect(sorted.map((i) => i.name)).toEqual(['eta', 'zeta', 'Alpha', 'beta']);
    expect(sorted[1].items?.map((i) => i.name)).toEqual(['A', 'z']);
  });

  it('lists folders with their path', () => {
    expect(listFolders(tree())).toEqual([
      { id: 'f1', label: 'f1' },
      { id: 'f2', label: 'f1 / f2' },
    ]);
  });
});
