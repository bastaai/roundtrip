import { describe, expect, it } from 'vitest';
import { portablePath, protoFilePath, relativeTo } from './paths';

describe('relativeTo', () => {
  it('returns the path below the parent with forward slashes', () => {
    expect(relativeTo('/code/svc', '/code/svc/protos/v1')).toBe('protos/v1');
    expect(relativeTo('/code/svc/', '/code/svc/protos')).toBe('protos');
    expect(relativeTo('C:\\code\\svc', 'C:\\code\\svc\\protos\\v1')).toBe('protos/v1');
  });

  it('returns "." for the same folder and null outside it', () => {
    expect(relativeTo('/code/svc', '/code/svc')).toBe('.');
    expect(relativeTo('/code/svc', '/code/svc-other/protos')).toBeNull();
    expect(relativeTo('/code/svc', '/code')).toBeNull();
    expect(relativeTo('protos', '/code/svc/protos')).toBeNull();
  });
});

describe('portablePath', () => {
  it('is relative inside the workspace and absolute outside it', () => {
    expect(portablePath('/code/svc/api/protos', '/code/svc/api')).toBe('protos');
    expect(portablePath('/code/svc/api', '/code/svc/api')).toBe('.');
    expect(portablePath('/code/shared/protos', '/code/svc/api')).toBe('/code/shared/protos');
  });
});

describe('protoFilePath', () => {
  const workspace = '/code/svc/api';

  it('is relative to the import root that contains the file', () => {
    expect(protoFilePath('/code/svc/api/protos/billing/v1/pay.proto', ['protos'], workspace)).toBe('billing/v1/pay.proto');
    expect(protoFilePath('/code/shared/protos/common/money.proto', ['protos', '/code/shared/protos'], workspace)).toBe('common/money.proto');
    expect(protoFilePath('/code/svc/api/x.proto', ['.'], workspace)).toBe('x.proto');
  });

  it('ignores roots it cannot resolve and falls back to a workspace or absolute path', () => {
    expect(protoFilePath('/code/svc/api/protos/a.proto', ['{{protoRoot}}', '~/protos'], workspace)).toBe('protos/a.proto');
    expect(protoFilePath('/elsewhere/a.proto', ['protos'], workspace)).toBe('/elsewhere/a.proto');
    expect(protoFilePath('/code/svc/api/a.proto', [], workspace)).toBe('a.proto');
  });
});
