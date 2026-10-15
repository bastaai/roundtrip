import { describe, expect, it } from 'vitest';
import { highlightJson } from './highlight';

const kinds = (src: string) => highlightJson(src).map((r) => [r.text, r.kind ?? '']);

describe('highlightJson', () => {
  it('colours keys, strings, numbers and literals', () => {
    expect(kinds('{"a": "x", "b" : -1.5e3, "c": [true, null]}')).toEqual([
      ['{', ''],
      ['"a"', 'key'],
      [': ', ''],
      ['"x"', 'str'],
      [', ', ''],
      ['"b"', 'key'],
      [' : ', ''],
      ['-1.5e3', 'num'],
      [', ', ''],
      ['"c"', 'key'],
      [': [', ''],
      ['true', 'lit'],
      [', ', ''],
      ['null', 'lit'],
      [']}', ''],
    ]);
  });

  it('handles escapes, unfinished strings and stray text', () => {
    expect(kinds('"a\\"b"')).toEqual([['"a\\"b"', 'str']]);
    expect(kinds('{"a": "unfinished\n}')).toEqual([
      ['{', ''],
      ['"a"', 'key'],
      [': ', ''],
      ['"unfinished', 'str'],
      ['\n}', ''],
    ]);
    expect(kinds('{{token}} - x')).toEqual([['{{token}} - x', '']]);
  });

  it('marks {{variables}} inside strings', () => {
    expect(highlightJson('{"id": "acc-{{ accountId }}/{{x}}"}')).toEqual([
      { text: '{', kind: undefined },
      { text: '"id"', kind: 'key' },
      { text: ': ', kind: undefined },
      { text: '"acc-', kind: 'str' },
      { text: '{{ accountId }}', kind: 'var', name: 'accountId' },
      { text: '/', kind: 'str' },
      { text: '{{x}}', kind: 'var', name: 'x' },
      { text: '"', kind: 'str' },
      { text: '}', kind: undefined },
    ]);
  });

  it('always gives back the input', () => {
    for (const src of ['', '{"a": [1, 2, {"b": "\\u00e9"}]}', '{\n  "x": tru', '"\\', '{{baseUrl}}/x?q="1"']) {
      expect(highlightJson(src).map((r) => r.text).join('')).toBe(src);
    }
  });
});
