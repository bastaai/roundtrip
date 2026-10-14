import type { Collection, Item } from '../types';
import { isFolder } from './tree';

// A saved request and where it lives: path is the collection name followed by its folders.
export type RequestEntry = { collection: Collection; item: Item; path: string[] };

export function listRequests(collections: Collection[]): RequestEntry[] {
  const walk = (collection: Collection, items: Item[], path: string[]): RequestEntry[] =>
    items.flatMap((item) => (isFolder(item) ? walk(collection, item.items ?? [], [...path, item.name]) : [{ collection, item, path }]));
  return collections.flatMap((c) => walk(c, c.items, [c.name]));
}

const wordStart = (text: string, word: string) => {
  for (let i = text.indexOf(word); i >= 0; i = text.indexOf(word, i + 1)) {
    if (i === 0 || !/[a-z0-9]/.test(text[i - 1])) return true;
  }
  return false;
};

// "List users" -> "lu", "GetInvoice" -> "gi", "create-draft v2" -> "cdv2".
export const initials = (name: string) =>
  (name.match(/[A-Z]+(?![a-z])|[A-Za-z][a-z]*|[0-9]+/g) ?? []).map((w) => w[0].toLowerCase()).join('');

// How well one typed word fits. The name counts for much more than the context around it.
function wordScore(word: string, name: string, context: string, short: string): number {
  if (name === word) return 100;
  if (name.startsWith(word)) return 80;
  if (wordStart(name, word)) return 60;
  if (name.includes(word)) return 40;
  if (wordStart(context, word)) return 25;
  if (context.includes(word)) return 15;
  // The first letters of the name's words, e.g. "lu" for "List users".
  if (word.length > 1 && short.includes(word)) return short.startsWith(word) ? 10 : 6;
  return 0;
}

// 0 means no match. Every whitespace-separated word of the query has to fit somewhere.
export function score(query: string, name: string, context = ''): number {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return 1;
  const n = name.toLowerCase();
  const c = context.toLowerCase();
  const short = initials(name);
  let total = 0;
  let inName = 0;
  for (const word of words) {
    const s = wordScore(word, n, c, short);
    if (!s) return 0;
    total += s;
    if (s >= 40) inName += word.length;
  }
  const phrase = words.join(' ');
  if (words.length > 1 && n.includes(phrase)) total += 30;
  // Between equal matches the tighter name wins.
  return total + Math.min(inName / (n.length || 1), 1) * 10;
}

// Best first; equal scores keep their original order. A blank query keeps everything.
export function rank<T>(query: string, items: T[], text: (item: T) => { name: string; context?: string }): T[] {
  if (!query.trim()) return items;
  return items
    .map((item, index) => {
      const t = text(item);
      return { item, index, score: score(query, t.name, t.context) };
    })
    .filter((x) => x.score > 0)
    .sort((a, b) => b.score - a.score || a.index - b.index)
    .map((x) => x.item);
}
