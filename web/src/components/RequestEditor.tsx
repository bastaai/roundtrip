import { useState } from 'react';
import { kindOf, METHODS, parseQuery, prettyJson, setKind, setQuery } from '../lib/request';
import type { GrpcSource } from '../lib/proto';
import type { Auth, Body, Kind, Req } from '../types';
import { KVEditor } from './KVEditor';
import { TestsEditor } from './TestsEditor';
import { btnCls, Count, Dot, fieldCls, ghostBtnCls, inputCls, keyLabel, KEYS, methodColor, primaryBtnCls, Segmented, Select, Spinner, Tabs } from './ui';

type Props = {
  request: Req;
  sending: boolean;
  dirty: boolean;
  onChange: (request: Req) => void;
  onSend: () => void;
  onSave: () => void;
  onCurl: () => void;
  grpcSource: GrpcSource['kind'];
  onProtoSettings: () => void;
  onLoadMethods: () => Promise<string[]>;
};

type TabId = 'params' | 'headers' | 'auth' | 'body' | 'tests';

const CONTENT_TYPES = ['application/json', 'text/plain', 'application/xml', 'text/html'];
const textareaCls = `${fieldCls} flex-1 min-h-24 py-1.5 font-mono text-xs resize-none`;
const KIND_COLOR: Record<Kind, string> = { http: '', graphql: 'text-m-graphql', grpc: 'text-m-grpc' };

export function RequestEditor({ request, sending, dirty, onChange, onSend, onSave, onCurl, grpcSource, onProtoSettings, onLoadMethods }: Props) {
  const [tab, setTab] = useState<TabId>('params');
  const kind = kindOf(request);
  const params = parseQuery(request.url);
  const count = (n: number) => n > 0 && <Count n={n} />;
  const dot = (on: boolean) => on && <Dot />;
  const setBody = (body: Body) => onChange({ ...request, body });

  const headersTab = { id: 'headers' as const, label: <>{kind === 'grpc' ? 'Metadata' : 'Headers'}{count(request.headers.filter((h) => h.enabled).length)}</> };
  const authTab = { id: 'auth' as const, label: <>Auth{dot(request.auth.type !== 'none')}</> };
  const testsTab = { id: 'tests' as const, label: <>Tests{count(request.tests.length + request.extract.length)}</> };
  const tabs: { id: TabId; label: React.ReactNode }[] =
    kind === 'grpc'
      ? [{ id: 'body', label: 'Message' }, headersTab, authTab, testsTab]
      : kind === 'graphql'
        ? [{ id: 'body', label: 'Query' }, headersTab, authTab, testsTab]
        : [{ id: 'params', label: <>Params{count(params.length)}</> }, headersTab, authTab, { id: 'body', label: <>Body{dot(request.body.mode !== 'none')}</> }, testsTab];
  const current = tabs.some((t) => t.id === tab) ? tab : tabs[0].id;

  return (
    <div className="flex flex-col min-h-0 h-full">
      <div className="flex items-center gap-2 px-3 pt-3 pb-1.5">
        <div className="flex h-8 min-w-0 flex-1 items-stretch rounded-md border border-line bg-raised p-px transition-[border-color,box-shadow] duration-100 hover:border-line-strong focus-within:border-focus focus-within:ring-2 focus-within:ring-focus/25">
          <Select
            bare
            className="shrink-0"
            selectClassName={KIND_COLOR[kind] || 'text-muted'}
            value={kind}
            onChange={(e) => onChange(setKind(request, e.target.value as Kind))}
            title="Protocol"
            aria-label="Protocol"
          >
            <option value="http">HTTP</option>
            <option value="graphql">GraphQL</option>
            <option value="grpc">gRPC</option>
          </Select>
          {kind !== 'grpc' && (
            <>
              <Select
                bare
                className="shrink-0"
                selectClassName={`font-medium ${methodColor(request.method)}`}
                value={request.method}
                onChange={(e) => onChange({ ...request, method: e.target.value })}
                title="Method"
                aria-label="Method"
              >
                {(METHODS.includes(request.method) ? METHODS : [...METHODS, request.method]).map((m) => (
                  <option key={m}>{m}</option>
                ))}
              </Select>
            </>
          )}
          <input
            autoFocus
            className="min-w-0 flex-1 bg-transparent px-2.5 font-mono text-[12.5px] outline-none placeholder:font-sans placeholder:text-[13px] placeholder:text-faint"
            aria-label={kind === 'grpc' ? 'Server address' : 'URL'}
            spellCheck={false}
            placeholder={
              kind === 'grpc'
                ? 'grpc://localhost:50051 (grpcs:// for TLS)'
                : kind === 'graphql'
                  ? 'https://api.example.com/graphql'
                  : 'https://api.example.com/users or {{baseUrl}}/users'
            }
            value={request.url}
            onChange={(e) => onChange({ ...request, url: e.target.value })}
            onKeyDown={(e) => e.key === 'Enter' && onSend()}
          />
        </div>
        <button className={`${primaryBtnCls} h-8 w-[76px]`} disabled={sending} onClick={onSend} title={`Send (${keyLabel(KEYS.send)})`}>
          {sending ? <Spinner size={13} /> : 'Send'}
        </button>
        <button className={`${btnCls} h-8 w-[66px]`} onClick={onSave} title={`Save${dirty ? ' changes' : ''} (${keyLabel(KEYS.save)})`}>
          Save
          {dirty && <span className="size-1.5 rounded-full bg-muted" />}
        </button>
        {kind !== 'grpc' && (
          <button className={`${ghostBtnCls} h-8`} onClick={onCurl} title="Copy as cURL">
            cURL
          </button>
        )}
      </div>

      {kind === 'grpc' && (
        <GrpcMethod
          key={grpcSource}
          method={request.method}
          source={grpcSource}
          onChange={(method) => onChange({ ...request, method })}
          onSettings={onProtoSettings}
          onLoad={onLoadMethods}
        />
      )}

      <Tabs active={current} onChange={setTab} tabs={tabs} />

      <div className="p-3 overflow-auto flex-1 min-h-0">
        {current === 'tests' && (
          <TestsEditor tests={request.tests} extract={request.extract} onChange={(tests, extract) => onChange({ ...request, tests, extract })} />
        )}
        {current === 'params' && (
          <KVEditor
            toggle={false}
            rows={params}
            onChange={(rows) => onChange({ ...request, url: setQuery(request.url, rows) })}
            keyPlaceholder="Query param"
          />
        )}
        {current === 'headers' && (
          <KVEditor
            rows={request.headers}
            onChange={(headers) => onChange({ ...request, headers })}
            keyPlaceholder={kind === 'grpc' ? 'Metadata key' : 'Header'}
          />
        )}
        {current === 'auth' && <AuthEditor auth={request.auth} grpc={kind === 'grpc'} onChange={(auth) => onChange({ ...request, auth })} />}
        {current === 'body' && kind === 'http' && <BodyEditor body={request.body} onChange={setBody} />}
        {current === 'body' && kind === 'graphql' && (
          <div className="flex gap-3 h-full">
            <label className="flex flex-col gap-1.5 flex-[3] min-w-0">
              <span className="text-xs font-medium">Query</span>
              <textarea
                className={textareaCls}
                spellCheck={false}
                placeholder={'query ($id: ID!) {\n  user(id: $id) {\n    name\n  }\n}'}
                value={request.body.raw}
                onChange={(e) => setBody({ ...request.body, raw: e.target.value })}
              />
            </label>
            <label className="flex flex-col gap-1.5 flex-[2] min-w-0">
              <span className="text-xs font-medium">Variables (JSON)</span>
              <textarea
                className={textareaCls}
                spellCheck={false}
                placeholder={'{\n  "id": "{{userId}}"\n}'}
                value={request.body.variables ?? ''}
                onChange={(e) => setBody({ ...request.body, variables: e.target.value })}
              />
            </label>
          </div>
        )}
        {current === 'body' && kind === 'grpc' && (
          <div className="flex flex-col gap-2 h-full">
            <textarea
              className={textareaCls}
              aria-label="Message"
              spellCheck={false}
              placeholder={'{\n  "field": "value"\n}'}
              value={request.body.raw}
              onChange={(e) => setBody({ ...request.body, raw: e.target.value })}
            />
            <p className="text-muted text-xs leading-relaxed">
              The request message as JSON. Unary calls only. {GRPC_SOURCE[grpcSource].help}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// Where methods and message types come from, per GrpcSource kind.
const GRPC_SOURCE: Record<GrpcSource['kind'], { label: string; title: string; load: string; help: string }> = {
  proto: {
    label: 'Proto files',
    title: "Methods come from this collection's proto files. Click to edit them.",
    load: "List the methods in this collection's proto files",
    help: "Methods and message types come from this collection's proto files, so the server does not need reflection.",
  },
  reflection: {
    label: 'Server reflection',
    title: 'Methods come from the server using gRPC reflection. Click to use proto files for this collection instead.',
    load: "List the server's methods using gRPC reflection",
    help: 'Methods and message types come from the server, which must have gRPC reflection enabled. To use .proto files instead, add them to this collection.',
  },
  unsaved: {
    label: 'Server reflection',
    title: 'Proto files are set per collection. This request is not saved in one, so it uses gRPC reflection.',
    load: "List the server's methods using gRPC reflection",
    help: 'Proto files are set per collection. This request is not saved in one, so it uses server reflection, which the server must have enabled.',
  },
};

function GrpcMethod(p: {
  method: string;
  source: GrpcSource['kind'];
  onChange: (method: string) => void;
  onSettings: () => void;
  onLoad: () => Promise<string[]>;
}) {
  const source = GRPC_SOURCE[p.source];
  const [methods, setMethods] = useState<string[]>([]);
  const [status, setStatus] = useState<{ text: string; kind: 'busy' | 'ok' | 'error' }>();
  const load = async () => {
    setStatus({ text: 'Loading…', kind: 'busy' });
    try {
      const found = await p.onLoad();
      setMethods(found);
      setStatus({ text: `${found.length} ${found.length === 1 ? 'method' : 'methods'}`, kind: 'ok' });
      if (!p.method && found.length) p.onChange(found[0]);
    } catch (e) {
      setStatus({ text: e instanceof Error ? e.message : String(e), kind: 'error' });
    }
  };
  return (
    <div className="flex items-center gap-2 px-3 pb-2">
      <input
        className={`${inputCls} h-8 flex-1 font-mono text-xs placeholder:font-sans placeholder:text-[13px]`}
        aria-label="Method"
        spellCheck={false}
        placeholder="package.Service/Method"
        list="grpc-methods"
        value={p.method}
        onChange={(e) => p.onChange(e.target.value)}
      />
      <datalist id="grpc-methods">
        {methods.map((m) => (
          <option key={m} value={m} />
        ))}
      </datalist>
      {status && (
        <span className={`max-w-72 truncate text-xs ${status.kind === 'error' ? 'text-danger' : 'text-muted'}`} title={status.text}>
          {status.text}
        </span>
      )}
      {p.source === 'unsaved' ? (
        <span className="shrink-0 px-1 text-xs text-muted" title={source.title}>
          {source.label}
        </span>
      ) : (
        <button className={`${ghostBtnCls} h-8 px-2 text-xs`} onClick={p.onSettings} title={source.title}>
          {source.label}
        </button>
      )}
      <button className={`${btnCls} h-8`} disabled={status?.kind === 'busy'} onClick={load} title={source.load}>
        Load methods
      </button>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex items-center gap-3">
      <span className="w-20 shrink-0 text-muted">{label}</span>
      {children}
    </label>
  );
}

function AuthEditor({ auth, grpc, onChange }: { auth: Auth; grpc: boolean; onChange: (auth: Auth) => void }) {
  const text = (field: 'token' | 'username' | 'password' | 'key' | 'value', placeholder: string) => (
    <input
      className={`${inputCls} flex-1 font-mono text-xs`}
      spellCheck={false}
      placeholder={placeholder}
      value={auth[field]}
      onChange={(e) => onChange({ ...auth, [field]: e.target.value })}
    />
  );
  return (
    <div className="flex flex-col gap-2 max-w-xl">
      <Field label="Type">
        <Select className="w-44" value={auth.type} onChange={(e) => onChange({ ...auth, type: e.target.value as Auth['type'] })}>
          <option value="none">No auth</option>
          <option value="bearer">Bearer token</option>
          <option value="basic">Basic auth</option>
          <option value="apikey">API key</option>
        </Select>
      </Field>
      {auth.type === 'none' && <p className="text-muted">This request is sent without credentials.</p>}
      {auth.type === 'bearer' && <Field label="Token">{text('token', '{{token}}')}</Field>}
      {auth.type === 'basic' && (
        <>
          <Field label="Username">{text('username', 'Username')}</Field>
          <Field label="Password">{text('password', '{{password}}')}</Field>
        </>
      )}
      {auth.type === 'apikey' && (
        <>
          <Field label="Key">{text('key', 'X-API-Key')}</Field>
          <Field label="Value">{text('value', '{{apiKey}}')}</Field>
          {!grpc && (
            <Field label="Add to">
              <Select className="w-44" value={auth.in} onChange={(e) => onChange({ ...auth, in: e.target.value as Auth['in'] })}>
                <option value="header">Header</option>
                <option value="query">Query params</option>
              </Select>
            </Field>
          )}
        </>
      )}
      {auth.type !== 'none' && (
        <p className="mt-1 text-xs leading-relaxed text-muted">
          Requests are saved to the repo as-is. Keep credentials in an environment variable and reference it here.
        </p>
      )}
    </div>
  );
}

function BodyEditor({ body, onChange }: { body: Body; onChange: (body: Body) => void }) {
  const modes: { id: Body['mode']; label: string }[] = [
    { id: 'none', label: 'None' },
    { id: 'raw', label: 'Raw' },
    { id: 'urlencoded', label: 'URL-encoded' },
    { id: 'form', label: 'Form data' },
  ];
  const types = CONTENT_TYPES.includes(body.contentType) || !body.contentType ? CONTENT_TYPES : [...CONTENT_TYPES, body.contentType];
  const pretty = body.mode === 'raw' ? prettyJson(body.raw) : null;

  return (
    <div className="flex flex-col gap-3 h-full">
      <div className="flex items-center gap-2">
        <Segmented label="Body type" options={modes} value={body.mode} onChange={(mode) => onChange({ ...body, mode })} />
        {body.mode === 'raw' && (
          <>
            <Select
              className="ml-auto w-40"
              aria-label="Content type"
              value={body.contentType}
              onChange={(e) => onChange({ ...body, contentType: e.target.value })}
            >
              {types.map((t) => (
                <option key={t}>{t}</option>
              ))}
            </Select>
            <button className={btnCls} disabled={!pretty} onClick={() => pretty && onChange({ ...body, raw: pretty })} title="Format the body as indented JSON">
              Beautify
            </button>
          </>
        )}
      </div>
      {body.mode === 'raw' && (
        <textarea
          className={textareaCls}
          aria-label="Request body"
          spellCheck={false}
          placeholder='{"hello": "{{name}}"}'
          value={body.raw}
          onChange={(e) => onChange({ ...body, raw: e.target.value })}
        />
      )}
      {(body.mode === 'urlencoded' || body.mode === 'form') && (
        <KVEditor rows={body.form} onChange={(form) => onChange({ ...body, form })} keyPlaceholder="Field" />
      )}
      {body.mode === 'none' && <p className="text-muted">This request has no body.</p>}
    </div>
  );
}
