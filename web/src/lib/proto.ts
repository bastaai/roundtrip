import type { Collection, ProtoCheck, ProtoConfig } from '../types';

// Trims, drops blank rows and repeats; order is kept because roots are searched in order.
// True when a gRPC message editor holds nothing worth keeping, so an example
// message can replace it without asking.
export const isBlankMessage = (raw: string) => raw.replace(/\s+/g, '') === '' || raw.replace(/\s+/g, '') === '{}';

export function cleanPaths(paths?: string[] | null): string[] {
  const seen = new Set<string>();
  return (paths ?? []).map((p) => p.trim()).filter((p) => p && !seen.has(p) && seen.add(p));
}

// undefined means "no proto config": the backend then uses server reflection.
// Empty lists are left out so the saved collection file stays small.
export function normalizeProto(proto?: ProtoConfig | null): ProtoConfig | undefined {
  const roots = cleanPaths(proto?.roots);
  const files = cleanPaths(proto?.files);
  if (!roots.length && !files.length) return undefined;
  return { ...(roots.length ? { roots } : {}), ...(files.length ? { files } : {}) };
}

export const sameProto = (a?: ProtoConfig | null, b?: ProtoConfig | null) =>
  JSON.stringify(normalizeProto(a)) === JSON.stringify(normalizeProto(b));

// Sets or removes the collection's proto config.
export function withProto(collection: Collection, proto?: ProtoConfig | null): Collection {
  const { proto: _old, ...rest } = collection;
  const next = normalizeProto(proto);
  return next ? { ...rest, proto: next } : rest;
}

// Where a gRPC request gets its methods and message types from.
// A tab that is not saved in a collection has no proto config to use.
export type GrpcSource = { kind: 'proto'; proto: ProtoConfig } | { kind: 'reflection' } | { kind: 'unsaved' };

export function grpcSource(collections: Collection[], source?: { collectionId: string } | null): GrpcSource {
  const collection = source && collections.find((c) => c.id === source.collectionId);
  if (!collection) return { kind: 'unsaved' };
  const proto = normalizeProto(collection.proto);
  return proto ? { kind: 'proto', proto } : { kind: 'reflection' };
}

export const protoOf = (source: GrpcSource) => (source.kind === 'proto' ? source.proto : undefined);

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`;

export function checkSummary(check: Partial<ProtoCheck>): string {
  return [plural(check.files ?? 0, 'file'), plural(check.services?.length ?? 0, 'service'), plural(check.methods?.length ?? 0, 'method')].join(' · ');
}

// "pkg.Service/Method" names under their service; a method of an unlisted service gets its own group.
export function groupMethods(check: Partial<ProtoCheck>): { service: string; methods: string[] }[] {
  const groups = new Map<string, string[]>((check.services ?? []).map((s) => [s, []]));
  for (const full of check.methods ?? []) {
    const i = full.lastIndexOf('/');
    const service = i >= 0 ? full.slice(0, i) : '';
    if (!groups.has(service)) groups.set(service, []);
    groups.get(service)!.push(i >= 0 ? full.slice(i + 1) : full);
  }
  return [...groups].map(([service, methods]) => ({ service, methods }));
}
