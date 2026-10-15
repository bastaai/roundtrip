import { describe, expect, it } from 'vitest';
import type { GrpcEnum } from '../types';
import { applyCompletion, enumCompletion } from './enums';

const KIND = ['KIND_UNSPECIFIED', 'BOOK', 'FILM'];
const ROLE = ['ROLE_UNSPECIFIED', 'ADMIN', 'VIEWER'];
const ENUMS: GrpcEnum[] = [
  { path: ['kind'], enum: 'Kind', values: KIND },
  { path: ['owner', 'role'], enum: 'Role', values: ROLE },
  { path: ['roles', '*'], enum: 'Role', values: ROLE },
  { path: ['items', '*', 'kind'], enum: 'Kind', values: KIND },
  { path: ['byName', '*'], enum: 'Kind', values: KIND },
];

// The caret is where | is.
const at = (text: string) => {
  const caret = text.indexOf('|');
  return { raw: text.slice(0, caret) + text.slice(caret + 1), caret };
};
const complete = (text: string) => {
  const { raw, caret } = at(text);
  return enumCompletion(raw, caret, ENUMS);
};

describe('enumCompletion', () => {
  it('offers values inside the string of an enum field', () => {
    expect(complete('{"kind": "|BOOK"}')).toMatchObject({ from: 10, to: 14, quoted: true, closed: true, prefix: '', values: KIND });
    expect(complete('{"kind": "BO|OK"}')?.values).toEqual(['BOOK']);
  });

  it('puts values starting with what is typed first, then those containing it', () => {
    expect(complete('{"owner": {"role": "v|"}}')?.values).toEqual(['VIEWER']);
    expect(complete('{"owner": {"role": "ad|"}}')?.values).toEqual(['ADMIN']);
    expect(complete('{"kind": "UNSP|"}')?.values).toEqual(['KIND_UNSPECIFIED']);
    expect(complete('{"kind": "zzz|"}')).toBeNull();
  });

  it('follows lists, maps and nesting', () => {
    expect(complete('{"roles": ["ADMIN", "|"]}')?.values).toEqual(ROLE);
    expect(complete('{"items": [{"kind": "FILM"}, {"name": "x", "kind": "|"}]}')?.values).toEqual(KIND);
    expect(complete('{"byName": {"a": "|"}}')?.values).toEqual(KIND);
  });

  it('offers a value where one is still missing', () => {
    expect(complete('{"kind": |}')).toMatchObject({ from: 9, to: 9, quoted: false, prefix: '' });
    expect(complete('{\n  "kind": |\n}')?.quoted).toBe(false);
  });

  it('works while the message is still being typed', () => {
    expect(complete('{"name": "x", "kind": "B|')).toMatchObject({ quoted: true, closed: false, values: ['BOOK'] });
  });

  it('offers nothing on keys, other fields or literals', () => {
    expect(complete('{"ki|nd": "BOOK"}')).toBeNull();
    expect(complete('{"name": "|"}')).toBeNull();
    expect(complete('{"kind": 1|}')).toBeNull();
    expect(complete('{"owner": {"name": "|"}}')).toBeNull();
    expect(complete('{"name": "a \\"kind\\": |"}')).toBeNull();
  });
});

describe('applyCompletion', () => {
  it('replaces the inside of the string and leaves the caret after it', () => {
    const { raw, caret } = at('{"kind": "BO|OK", "x": 1}');
    const c = enumCompletion(raw, caret, ENUMS)!;
    expect(applyCompletion(raw, c, 'FILM')).toEqual({ text: '{"kind": "FILM", "x": 1}', caret: 15 });
  });

  it('adds the quotes when there were none', () => {
    const { raw, caret } = at('{"kind": |}');
    const c = enumCompletion(raw, caret, ENUMS)!;
    expect(applyCompletion(raw, c, 'BOOK')).toEqual({ text: '{"kind": "BOOK"}', caret: 15 });
  });

  it('closes an unterminated string', () => {
    const { raw, caret } = at('{"kind": "B|');
    const c = enumCompletion(raw, caret, ENUMS)!;
    expect(applyCompletion(raw, c, 'BOOK').text).toBe('{"kind": "BOOK"');
  });
});
