import { describe, expect, it } from 'vitest';
import type { Environment } from '../types';
import { lookupVariable, variableAt, withVariable } from './variables';

const env: Environment = {
  id: 'e',
  name: 'Local',
  variables: [
    { key: 'accountId', value: 'acc_1', enabled: true },
    { key: 'token', value: 's3cret', enabled: true, secret: true },
    { key: 'off', value: 'x', enabled: false },
    { key: 'dup', value: 'first', enabled: true },
    { key: 'dup', value: 'last', enabled: true },
  ],
};

describe('lookupVariable', () => {
  it('finds enabled environment variables, the last one winning like on the server', () => {
    expect(lookupVariable('accountId', env, {})).toEqual({ value: 'acc_1', secret: false, from: 'environment' });
    expect(lookupVariable('token', env, {})?.secret).toBe(true);
    expect(lookupVariable('dup', env, {})?.value).toBe('last');
  });

  it('skips disabled and unknown names', () => {
    expect(lookupVariable('off', env, {})).toBeUndefined();
    expect(lookupVariable('nope', env, {})).toBeUndefined();
    expect(lookupVariable('accountId', undefined, {})).toBeUndefined();
  });

  it('prefers values saved from responses', () => {
    expect(lookupVariable('accountId', env, { accountId: 'acc_2' })).toEqual({ value: 'acc_2', secret: false, from: 'response' });
    expect(lookupVariable('fresh', undefined, { fresh: '' })).toEqual({ value: '', secret: false, from: 'response' });
  });
});

describe('withVariable', () => {
  it('changes the row that is used, leaving the rest alone', () => {
    const next = withVariable(env, 'dup', 'new');
    expect(next.variables.map((v) => v.value)).toEqual(['acc_1', 's3cret', 'x', 'first', 'new']);
    expect(lookupVariable('dup', next, {})?.value).toBe('new');
    expect(env.variables[4].value).toBe('last');
  });

  it('adds an enabled row for a name that is not set, even when a disabled one exists', () => {
    expect(withVariable(env, 'off', 'y').variables.slice(-1)).toEqual([{ key: 'off', value: 'y', enabled: true }]);
    expect(lookupVariable('off', withVariable(env, 'off', 'y'), {})?.value).toBe('y');
  });
});

describe('variableAt', () => {
  const url = '{{baseUrl}}/accounts/{{ accountId }}?q=1';
  it('finds the variable under a character', () => {
    expect(variableAt(url, 0)).toEqual({ name: 'baseUrl', start: 0, end: 11 });
    expect(variableAt(url, 10)?.name).toBe('baseUrl');
    expect(variableAt(url, 25)).toEqual({ name: 'accountId', start: 21, end: 36 });
  });
  it('is undefined between variables and past the end', () => {
    expect(variableAt(url, 11)).toBeUndefined();
    expect(variableAt(url, 36)).toBeUndefined();
    expect(variableAt(url, 100)).toBeUndefined();
    expect(variableAt('{{}}', 1)).toBeUndefined();
  });
});
