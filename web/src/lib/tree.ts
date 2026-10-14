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

export function listFolders(items: Item[], prefix = ''): { id: string; label: string }[] {
  return items.filter(isFolder).flatMap((folder) => {
    const label = prefix + folder.name;
    return [{ id: folder.id, label }, ...listFolders(folder.items ?? [], label + ' / ')];
  });
}
