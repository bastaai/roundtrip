import type { Item } from '../types';

export const isFolder = (item: Item) => !item.request;

export function findItem(items: Item[], id: string): Item | undefined {
  for (const item of items) {
    if (item.id === id) return item;
    const found = findItem(item.items ?? [], id);
    if (found) return found;
  }
}

export function mapItem(items: Item[], id: string, fn: (item: Item) => Item): Item[] {
  return items.map((item) => {
    if (item.id === id) return fn(item);
    return item.items ? { ...item, items: mapItem(item.items, id, fn) } : item;
  });
}

export function removeItem(items: Item[], id: string): Item[] {
  return items
    .filter((item) => item.id !== id)
    .map((item) => (item.items ? { ...item, items: removeItem(item.items, id) } : item));
}

// parentId null adds at the root.
export function addItem(items: Item[], parentId: string | null, child: Item): Item[] {
  if (parentId === null) return [...items, child];
  return mapItem(items, parentId, (parent) => ({ ...parent, items: [...(parent.items ?? []), child] }));
}

// Same order the server reads files back in: folders first, then by name.
export function sortItems(items: Item[]): Item[] {
  return items
    .map((item) => (item.items ? { ...item, items: sortItems(item.items) } : item))
    .sort((a, b) => Number(isFolder(b)) - Number(isFolder(a)) || a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
}

const SEP = ' / ';
const terms = (query: string) => query.toLowerCase().split(/\s+/).filter(Boolean);
const pathMatches = (path: string, words: string[]) => words.every((w) => path.includes(w));

function filterItems(items: Item[], words: string[], prefix: string): Item[] {
  return items.flatMap((item) => {
    const path = prefix + SEP + item.name.toLowerCase();
    if (!isFolder(item)) return pathMatches(path, words) ? [item] : [];
    const kept = filterItems(item.items ?? [], words, path);
    return kept.length || pathMatches(path, words) ? [{ ...item, items: kept }] : [];
  });
}

// Narrows the tree to what matches the filter. Every word must appear somewhere in
// "collection / folder / request", in any order and any case, so a matching folder or
// collection keeps everything inside it. A blank filter returns the input untouched.
export function filterCollections<C extends { name: string; items: Item[] }>(collections: C[], query: string): C[] {
  const words = terms(query);
  if (!words.length) return collections;
  return collections.flatMap((c) => {
    const path = c.name.toLowerCase();
    const items = filterItems(c.items, words, path);
    return items.length || pathMatches(path, words) ? [{ ...c, items }] : [];
  });
}

export const countRequests = (items: Item[]): number => items.reduce((n, item) => n + (isFolder(item) ? countRequests(item.items ?? []) : 1), 0);

// Names of the folders that hold id, outermost first; undefined when id is not in items.
export function folderPath(items: Item[], id: string): string[] | undefined {
  for (const item of items) {
    if (item.id === id) return [];
    const below = folderPath(item.items ?? [], id);
    if (below) return [item.name, ...below];
  }
}

// base, or base followed by the first number that no item at this level is called yet.
export function freeName(items: Item[], base: string): string {
  const taken = new Set(items.map((i) => i.name.toLowerCase()));
  if (!taken.has(base.toLowerCase())) return base;
  let n = 2;
  while (taken.has(`${base} ${n}`.toLowerCase())) n++;
  return `${base} ${n}`;
}

// The id of the folder that holds id: null at the top level, undefined when id is not in items.
export function parentOf(items: Item[], id: string, parent: string | null = null): string | null | undefined {
  for (const item of items) {
    if (item.id === id) return parent;
    const found = parentOf(item.items ?? [], id, item.id);
    if (found !== undefined) return found;
  }
}

// The id of item and of everything inside it.
export const subtreeIds = (item: Item): string[] => [item.id, ...(item.items ?? []).flatMap(subtreeIds)];

// Moves item id of `from`, with everything in it, into folder `into` of `to`
// (null: the top level). Returns the changed collections, `from` first, or
// null when the move is impossible (a folder into itself) or changes nothing.
export function moveItem<C extends { id: string; items: Item[] }>(from: C, to: C, id: string, into: string | null): C[] | null {
  const item = findItem(from.items, id);
  if (!item) return null;
  if (into !== null && (into === id || findItem(item.items ?? [], into) || !findItem(to.items, into))) return null;
  if (from.id === to.id && parentOf(from.items, id) === into) return null;
  const rest = removeItem(from.items, id);
  if (from.id === to.id) return [{ ...from, items: addItem(rest, into, item) }];
  return [
    { ...from, items: rest },
    { ...to, items: addItem(to.items, into, item) },
  ];
}
