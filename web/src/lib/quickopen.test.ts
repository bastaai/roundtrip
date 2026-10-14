import { describe, expect, it } from 'vitest';
import type { Collection, Item } from '../types';
import { initials, listRequests, rank, score } from './quickopen';
import { emptyRequest } from './request';

const req = (name: string): Item => ({ id: name, name, request: emptyRequest() });
const folder = (name: string, items: Item[]): Item => ({ id: name, name, items });

const collections: Collection[] = [
  { id: 'c1', name: 'Billing', items: [folder('Invoices', [req('List invoices'), folder('Drafts', [req('Create draft')])]), req('Health')] },
  { id: 'c2', name: 'Users', items: [folder('Empty', []), req('List users')] },
];

describe('listRequests', () => {
  it('flattens every request with its collection and folder path', () => {
    expect(listRequests(collections).map((e) => [...e.path, e.item.name].join('/'))).toEqual([
      'Billing/Invoices/List invoices',
      'Billing/Invoices/Drafts/Create draft',
      'Billing/Health',
      'Users/List users',
    ]);
    expect(listRequests(collections)[3].collection.id).toBe('c2');
  });

  it('survives collections without requests', () => {
    expect(listRequests([{ id: 'c', name: 'Empty', items: [] }])).toEqual([]);
  });
});

describe('score', () => {
  it('is zero when a word fits nowhere', () => {
    expect(score('zzz', 'List users', 'Users')).toBe(0);
    expect(score('list zzz', 'List users', 'Users')).toBe(0);
  });

  it('matches any case and a blank query matches everything', () => {
    expect(score('LIST', 'list users')).toBeGreaterThan(0);
    expect(score('  ', 'anything')).toBeGreaterThan(0);
  });

  it('prefers the name over the path', () => {
    expect(score('users', 'List users', 'Admin')).toBeGreaterThan(score('users', 'Health', 'Users'));
  });

  it('orders exact, prefix, word start, inside, then initials', () => {
    const s = (name: string) => score('get', name);
    expect(s('Get')).toBeGreaterThan(s('Get user'));
    expect(s('Get user')).toBeGreaterThan(s('User get'));
    expect(s('User get')).toBeGreaterThan(s('Widgets'));
    expect(s('Widgets')).toBeGreaterThan(s('Give extra time'));
    expect(s('Give extra time')).toBeGreaterThan(0);
  });

  it('reads initials across spaces, punctuation and camel case', () => {
    expect(initials('List users')).toBe('lu');
    expect(initials('GetInvoice')).toBe('gi');
    expect(initials('create-draft v2')).toBe('cdv2');
    expect(initials('HTTPServer status')).toBe('hss');
    expect(score('gi', 'GetInvoice')).toBeGreaterThan(0);
  });

  it('does not match scattered letters', () => {
    expect(score('stag', 'List collections (failing check)')).toBe(0);
  });

  it('treats punctuation as a word boundary', () => {
    expect(score('draft', 'create-draft')).toBe(score('draft', 'create draft'));
  });

  it('lets the words come from name and path in any order', () => {
    expect(score('billing list', 'List invoices', 'Billing / Invoices')).toBeGreaterThan(0);
    expect(score('list billing', 'List invoices', 'Billing / Invoices')).toBeGreaterThan(0);
  });

  it('rewards the words appearing together and a tighter name', () => {
    expect(score('list users', 'List users')).toBeGreaterThan(score('list users', 'List all users'));
    expect(score('list', 'List')).toBeGreaterThan(score('list', 'List of everything'));
  });
});

describe('rank', () => {
  const entries = listRequests(collections);
  const text = (e: (typeof entries)[number]) => ({ name: e.item.name, context: e.path.join(' / ') });
  const names = (q: string) => rank(q, entries, text).map((e) => e.item.name);

  it('returns everything in order for a blank query', () => {
    expect(rank('', entries, text)).toBe(entries);
  });

  it('puts the best match first and drops non-matches', () => {
    expect(names('list')).toEqual(['List users', 'List invoices']);
    expect(names('users')).toEqual(['List users']);
    expect(names('nothing here')).toEqual([]);
  });

  it('finds requests by folder or collection', () => {
    expect(names('drafts')).toEqual(['Create draft']);
    expect(names('billing')).toEqual(['List invoices', 'Create draft', 'Health']);
  });

  it('finds a request by its initials', () => {
    expect(names('lu')).toEqual(['List users']);
    expect(names('cd')).toEqual(['Create draft']);
  });

  it('keeps the original order between equal matches', () => {
    const items = ['Get a', 'Get b', 'Get c'];
    expect(rank('get', items, (name) => ({ name }))).toEqual(items);
  });
});
