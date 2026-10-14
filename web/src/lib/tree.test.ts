import { describe, expect, it } from 'vitest';
import type { Item } from '../types';
import { emptyRequest } from './request';
import { addItem, countRequests, filterCollections, findItem, listFolders, mapItem, removeItem, sortItems } from './tree';

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

describe('filterCollections', () => {
  const cols = () => [
    { id: 'c1', name: 'Billing API', items: [folder('f1', [req('r1', 'List invoices'), req('r2', 'Create invoice')], 'Invoices'), req('r3', 'Health')] },
    { id: 'c2', name: 'Users', items: [folder('f2', [], 'Empty admin'), req('r4', 'List users')] },
  ];
  const names = (cs: { name: string; items: Item[] }[]) => {
    const walk = (items: Item[]): string[] => items.flatMap((i) => [i.name, ...walk(i.items ?? [])]);
    return cs.flatMap((c) => [c.name, ...walk(c.items)]);
  };

  it('returns the same array for a blank filter', () => {
    const input = cols();
    expect(filterCollections(input, '')).toBe(input);
    expect(filterCollections(input, '   ')).toBe(input);
  });

  it('keeps matching requests with their ancestors and drops the rest', () => {
    expect(names(filterCollections(cols(), 'create'))).toEqual(['Billing API', 'Invoices', 'Create invoice']);
  });

  it('ignores case and matches across collections', () => {
    expect(names(filterCollections(cols(), 'LIST'))).toEqual(['Billing API', 'Invoices', 'List invoices', 'Users', 'List users']);
  });

  it('keeps everything inside a matching folder or collection', () => {
    expect(names(filterCollections(cols(), 'invoices'))).toEqual(['Billing API', 'Invoices', 'List invoices', 'Create invoice']);
    expect(names(filterCollections(cols(), 'billing'))).toEqual(['Billing API', 'Invoices', 'List invoices', 'Create invoice', 'Health']);
  });

  it('needs every word, found anywhere in the path', () => {
    expect(names(filterCollections(cols(), 'billing list'))).toEqual(['Billing API', 'Invoices', 'List invoices']);
    expect(names(filterCollections(cols(), 'users invoice'))).toEqual([]);
  });

  it('keeps a matching empty folder and does not change the input', () => {
    const input = cols();
    expect(names(filterCollections(input, 'admin'))).toEqual(['Users', 'Empty admin']);
    expect(names(input)).toHaveLength(8);
  });

  it('counts requests, not folders', () => {
    expect(countRequests(cols()[0].items)).toBe(3);
    expect(countRequests(filterCollections(cols(), 'create')[0].items)).toBe(1);
  });
});
