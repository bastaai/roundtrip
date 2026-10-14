import { normalizeProto } from './lib/proto';
import type { FsListing, GrpcMethodInfo, Collection, Environment, HistoryEntry, ProtoCheck, ProtoConfig, Req, Res, Workspace } from './types';

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch('/api' + path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : typeof body === 'string' ? body : JSON.stringify(body),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
  return res.status === 204 ? (undefined as T) : res.json();
}

export const api = {
  collections: () => call<Collection[]>('GET', '/collections'),
  putCollection: (c: Collection) => call<Collection>('PUT', `/collections/${c.id}`, c),
  deleteCollection: (id: string) => call<void>('DELETE', `/collections/${id}`),

  environments: () => call<Environment[]>('GET', '/environments'),
  putEnvironment: (e: Environment) => call<Environment>('PUT', `/environments/${e.id}`, e),
  deleteEnvironment: (id: string) => call<void>('DELETE', `/environments/${id}`),

  workspace: () => call<Workspace>('GET', '/workspace'),
  openWorkspace: (path: string, carry: boolean) => call<Workspace>('POST', '/workspace/open', { path, carry }),
  // Sub-folders of path (the workspace when empty), plus files ending in ext when given.
  listDir: (path: string, ext = '') => call<FsListing>('GET', `/fs/list?path=${encodeURIComponent(path)}&ext=${encodeURIComponent(ext)}`),
  gitSync: (message: string) => call<Workspace>('POST', '/git/sync', { message }),
  gitPull: () => call<Workspace>('POST', '/git/pull', {}),

  runtimeVariables: () => call<Record<string, string>>('GET', '/variables/runtime'),
  clearRuntimeVariables: () => call<void>('DELETE', '/variables/runtime'),

  history: () => call<HistoryEntry[]>('GET', '/history'),
  clearHistory: () => call<void>('DELETE', '/history'),
  historyResponse: (id: string) => call<Res>('GET', `/history/${id}/response`),

  // proto is the config of the request's collection; it is left out of the body when empty.
  // itemId is the saved request being sent, so its history can be shown with it.
  send: (request: Req, environmentId: string, proto?: ProtoConfig, itemId?: string) =>
    call<Res>('POST', '/send', { request, environmentId, proto: normalizeProto(proto), itemId }),
  grpcMethods: (request: Req, environmentId: string, proto?: ProtoConfig) =>
    call<string[]>('POST', '/grpc/methods', { request, environmentId, proto: normalizeProto(proto) }),
  grpcDescribe: (request: Req, environmentId: string, proto?: ProtoConfig) =>
    call<GrpcMethodInfo>('POST', '/grpc/describe', { request, environmentId, proto: normalizeProto(proto) }),
  checkProto: (proto: ProtoConfig, environmentId: string) => call<ProtoCheck>('POST', '/proto/check', { proto, environmentId }),
  parseCurl: (command: string) => call<Req>('POST', '/curl/parse', { command }),
  exportCurl: (request: Req, environmentId: string) =>
    call<{ command: string }>('POST', '/curl/export', { request, environmentId }).then((r) => r.command),
};
