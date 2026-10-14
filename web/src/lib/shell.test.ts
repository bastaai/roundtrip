import { expect, it } from 'vitest';
import { folderName } from './shell';

it('folderName', () => {
  expect(folderName('/Users/you/code/api-requests')).toBe('api-requests');
  expect(folderName('/Users/you/code/api-requests/')).toBe('api-requests');
  expect(folderName('C:\\Users\\you\\api-requests')).toBe('api-requests');
  expect(folderName('requests')).toBe('requests');
  expect(folderName('/')).toBe('/');
});
