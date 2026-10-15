import { useEffect, useRef, useState } from 'react';
import { kindOf, METHODS, parseQuery, prettyJson, setKind, setQuery } from '../lib/request';
import { HINTS } from '../lib/hints';
import { variableAt } from '../lib/variables';
import { isBlankMessage, type GrpcSource } from '../lib/proto';
import type { Auth, Body, GrpcMethodInfo, Kind, Req } from '../types';
import { JsonEditor } from './JsonEditor';
import { KVEditor } from './KVEditor';
import { TestsEditor } from './TestsEditor';
import { useVariableHover } from './Variables';
import { btnCls, Count, Dot, fieldCls, ghostBtnCls, inputCls, keyLabel, KEYS, methodColor, primaryBtnCls, Segmented, Select, Spinner, Tabs } from './ui';

type BarProps = {
  request: Req;
  sending: boolean;
  onChange: (request: Req) => void;
  onSend: () => void;
  grpcSource: GrpcSource['kind'];
  onProtoSettings: () => void;
  onLoadMethods: () => Promise<string[]>;
  // Message types and an example message for a method of this request's server.
  onDescribe: (method: string) => Promise<GrpcMethodInfo>;
  // The address of a gRPC server listening locally on the usual port, or "".
  onLocalGrpc: () => Promise<string>;
};

type TabId = 'params' | 'headers' | 'auth' | 'body' | 'tests' | 'docs';

const CONTENT_TYPES = ['application/json', 'text/plain', 'application/xml', 'text/html'];
const textareaCls = `${fieldCls} flex-1 min-h-24 py-1.5 font-mono text-xs resize-none`;
const KIND_COLOR: Record<Kind, string> = { http: '', graphql: 'text-m-graphql', grpc: 'text-m-grpc' };

// Where the round trip starts: method, address and Send in one field across the whole main area.
export function AddressBar({ request, sending, onChange, onSend, grpcSource, onProtoSettings, onLoadMethods, onDescribe, onLocalGrpc }: BarProps) {
  const kind = kindOf(request);
  // {{variables}} in the address show their value on hover. The text is monospaced,
  // so the character under the pointer follows from one character's width.
  const hover = useVariableHover();
  const frame = useRef<HTMLDivElement>(null);
  const urlInput = useRef<HTMLInputElement>(null);
  const onUrlMouseMove = (e: React.MouseEvent) => {
    const input = urlInput.current;
    if (hover.busy(e) || !input || !frame.current) return;
    const style = getComputedStyle(input);
    const ctx = document.createElement('canvas').getContext('2d')!;
    ctx.font = style.font;
    const width = ctx.measureText('0').width;
    const box = input.getBoundingClientRect();
    const left = box.left + parseFloat(style.paddingLeft) - input.scrollLeft;
    const found = e.target === input ? variableAt(request.url, Math.floor((e.clientX - left) / width)) : undefined;
    if (!found) return hover.hideSoon();
    const outer = frame.current.getBoundingClientRect();
    hover.show({ name: found.name, top: outer.height + 4, left: left - outer.left + found.start * width });
  };
  // A gRPC request with no address yet gets the local server's, when one is running.
  const latest = useRef(request);
  latest.current = request;
  const noAddress = kind === 'grpc' && !request.url.trim();
  useEffect(() => {
    if (!noAddress) return;
    onLocalGrpc().then(
      (url) => {
        const now = latest.current;
        if (url && kindOf(now) === 'grpc' && !now.url.trim()) onChange({ ...now, url });
      },
      () => {},
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [noAddress]);
  // One menu for the HTTP methods and the other two protocols.
  const verb = kind === 'http' ? request.method : kind;
  const chooseVerb = (v: string) => {
    if (v === 'graphql' || v === 'grpc') return onChange(setKind(request, v));
    onChange({ ...(kind === 'http' ? request : setKind(request, 'http')), method: v });
  };
  // Choosing a known method fills in an example of its request message, as
  // long as that does not overwrite something the user wrote.
  const chooseMethod = async (method: string, known: boolean) => {
    onChange({ ...request, method });
    if (!known || !isBlankMessage(request.body.raw)) return;
    try {
      const { template } = await onDescribe(method);
      onChange({ ...request, method, body: { ...request.body, raw: template } });
    } catch {
      // The example is a convenience; sending the request reports real problems.
    }
  };
  return (
    <div className="shrink-0 border-b border-line">
      <div className="flex items-center px-3 pb-3 pt-2.5">
        <div
          ref={frame}
          onMouseMove={onUrlMouseMove}
          onMouseLeave={hover.hideSoon}
          className="relative flex h-9 min-w-0 flex-1 items-stretch rounded-lg border border-line bg-raised p-[3px] transition-[border-color,box-shadow] duration-100 hover:border-line-strong focus-within:border-focus focus-within:ring-2 focus-within:ring-focus/25">
          <Select
            bare
            className="shrink-0"
            selectClassName={`font-medium ${kind === 'http' ? methodColor(request.method) : KIND_COLOR[kind]}`}
            value={verb}
            onChange={(e) => chooseVerb(e.target.value)}
            title="Method or protocol"
            aria-label="Method or protocol"
          >
            {(kind !== 'http' || METHODS.includes(request.method) ? METHODS : [...METHODS, request.method]).map((m) => (
              <option key={m}>{m}</option>
            ))}
            <option disabled>──────</option>
            <option value="graphql">GraphQL</option>
            <option value="grpc">gRPC</option>
          </Select>
          <input
            ref={urlInput}
            autoFocus
            className="min-w-0 flex-1 bg-transparent px-2.5 font-mono text-[13px] outline-none placeholder:font-sans placeholder:text-faint"
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
          {hover.popover}
          <button className={`${primaryBtnCls} h-full w-[72px] rounded-[5px]`} disabled={sending} onClick={onSend} title={`Send (${keyLabel(KEYS.send)})`}>
            {sending ? <Spinner size={13} /> : 'Send'}
          </button>
        </div>
      </div>

      {kind === 'grpc' && (
        <GrpcMethod
          key={grpcSource}
          method={request.method}
          source={grpcSource}
          onChange={chooseMethod}
          onSettings={onProtoSettings}
          onLoad={onLoadMethods}
        />
      )}
    </div>
  );
}

// What goes out: everything about the request except its address.
export function RequestEditor({
  request,
  onChange,
  grpcSource,
  onDescribe,
}: {
  request: Req;
  onChange: (request: Req) => void;
  grpcSource: GrpcSource['kind'];
  onDescribe: (method: string) => Promise<GrpcMethodInfo>;
}) {
  const [tab, setTab] = useState<TabId>('params');
  const [example, setExample] = useState<{ busy?: boolean; info?: GrpcMethodInfo; error?: string }>({});
  const insertExample = async () => {
    setExample({ busy: true });
    try {
      const info = await onDescribe(request.method);
      setExample({ info });
      onChange({ ...request, body: { ...request.body, raw: info.template } });
    } catch (e) {
      setExample({ error: e instanceof Error ? e.message : String(e) });
    }
  };
  const kind = kindOf(request);
  // The method's enum fields, so the message editor can offer their values.
  const [described, setDescribed] = useState<{ method: string; info: GrpcMethodInfo }>();
  useEffect(() => {
    const method = request.method.trim();
    if (kind !== 'grpc' || !/^[^/]+\/[^/]+$/.test(method) || described?.method === method) return;
    const timer = setTimeout(() => {
      onDescribe(method).then(
        (info) => setDescribed({ method, info }),
        () => {},
      );
    }, 400);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, request.method, grpcSource]);
  const enums = described?.method === request.method.trim() ? (described.info.enums ?? []) : (example.info?.enums ?? []);
  const params = parseQuery(request.url);
  const count = (n: number) => n > 0 && <Count n={n} />;
  const dot = (on: boolean) => on && <Dot />;
  const setBody = (body: Body) => onChange({ ...request, body });

  const headersTab = { id: 'headers' as const, title: kind === 'grpc' ? HINTS.metadata : HINTS.headers, label: <>{kind === 'grpc' ? 'Metadata' : 'Headers'}{count(request.headers.filter((h) => h.enabled).length)}</> };
  const authTab = { id: 'auth' as const, title: HINTS.auth, label: <>Auth{dot(request.auth.type !== 'none')}</> };
  const testsTab = { id: 'tests' as const, title: HINTS.tests, label: <>Tests{count(request.tests.length + request.extract.length)}</> };
  const docsTab = { id: 'docs' as const, title: HINTS.docs, label: <>Docs{dot(!!request.docs?.trim())}</> };
  const tabs: { id: TabId; label: React.ReactNode; title?: string }[] =
    kind === 'grpc'
      ? [{ id: 'body', label: 'Message', title: HINTS.message }, headersTab, authTab, testsTab, docsTab]
      : kind === 'graphql'
        ? [{ id: 'body', label: 'Query', title: HINTS.query }, headersTab, authTab, testsTab, docsTab]
        : [{ id: 'params', label: <>Params{count(params.length)}</>, title: HINTS.params }, headersTab, authTab, { id: 'body', label: <>Body{dot(request.body.mode !== 'none')}</>, title: HINTS.body }, testsTab, docsTab];
  const current = tabs.some((t) => t.id === tab) ? tab : tabs[0].id;

  return (
    <section aria-label="Request" className="@container flex h-full min-h-0 flex-col">
      <Tabs active={current} onChange={setTab} tabs={tabs} />

      <div className="p-3 overflow-auto flex-1 min-h-0">
        {current === 'docs' && (
          <div className="flex h-full flex-col gap-2">
            <textarea
              className={`${fieldCls} flex-1 min-h-24 resize-none py-1.5 leading-relaxed`}
              value={request.docs ?? ''}
              onChange={(e) => onChange({ ...request, docs: e.target.value || undefined })}
              placeholder="What this request is for, what it needs and what comes back."
              aria-label="Documentation"
              spellCheck
            />
            <p className="text-xs text-muted">Saved in the request's file, so it is shared through git with the request.</p>
          </div>
        )}
        {current === 'tests' && (
          <TestsEditor tests={request.tests} extract={request.extract} onChange={(tests, extract) => onChange({ ...request, tests, extract })} />
        )}
        {current === 'params' && (
          <KVEditor
            toggle={false}
            rows={params}
            onChange={(rows) => onChange({ ...request, url: setQuery(request.url, rows) })}
            keyPlaceholder="Query param"
            keyHint={HINTS.paramKey}
            valueHint={HINTS.paramValue}
          />
        )}
        {current === 'headers' && (
          <KVEditor
            rows={request.headers}
            onChange={(headers) => onChange({ ...request, headers })}
            keyPlaceholder={kind === 'grpc' ? 'Metadata key' : 'Header'}
            keyHint={kind === 'grpc' ? HINTS.metadataKey : HINTS.headerKey}
            valueHint={HINTS.headerValue}
          />
        )}
        {current === 'auth' && <AuthEditor auth={request.auth} grpc={kind === 'grpc'} onChange={(auth) => onChange({ ...request, auth })} />}
        {current === 'body' && kind === 'http' && <BodyEditor body={request.body} onChange={setBody} />}
        {current === 'body' && kind === 'graphql' && (
          <div className="flex h-full flex-col gap-3 @2xl:flex-row">
            <label className="flex flex-col gap-1.5 flex-[3] min-w-0">
              <span className="cursor-help self-start text-xs font-medium" title={HINTS.gqlQuery}>
                Query
              </span>
              <textarea
                className={textareaCls}
                spellCheck={false}
                placeholder={'query ($id: ID!) {\n  user(id: $id) {\n    name\n  }\n}'}
                value={request.body.raw}
                onChange={(e) => setBody({ ...request.body, raw: e.target.value })}
              />
            </label>
            <label className="flex flex-col gap-1.5 flex-[2] min-w-0">
              <span className="cursor-help self-start text-xs font-medium" title={HINTS.gqlVariables}>
                Variables (JSON)
              </span>
              <JsonEditor
                label="Variables"
                placeholder={'{\n  "id": "{{userId}}"\n}'}
                value={request.body.variables ?? ''}
                onChange={(variables) => setBody({ ...request.body, variables })}
              />
            </label>
          </div>
        )}
        {current === 'body' && kind === 'grpc' && (
          <div className="flex flex-col gap-2 h-full">
            <div className="flex min-h-7 items-center gap-2">
              <button
                className={`${ghostBtnCls} -ml-2 px-2`}
                disabled={example.busy || !request.method.trim()}
                onClick={insertExample}
                title="Replace the message with an example that has every field of this method's request type"
              >
                {example.busy ? <Spinner size={13} /> : 'Insert example message'}
              </button>
              {example.info && <span className="min-w-0 truncate font-mono text-xs text-muted">{example.info.input}</span>}
              {example.error && (
                <span className="min-w-0 truncate text-xs text-danger" title={example.error}>
                  {example.error}
                </span>
              )}
            </div>
            <JsonEditor
              label="Message"
              placeholder={'{\n  "field": "value"\n}'}
              enums={enums}
              value={request.body.raw}
              onChange={(raw) => setBody({ ...request.body, raw })}
            />
            <p className="text-muted text-xs leading-relaxed">
              The request message as JSON. Unary calls only. {GRPC_SOURCE[grpcSource].help}
            </p>
          </div>
        )}
      </div>
    </section>
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
  // known is true when the method is one of the loaded ones.
  onChange: (method: string, known: boolean) => void;
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
      if (!p.method && found.length) p.onChange(found[0], true);
    } catch (e) {
      setStatus({ text: e instanceof Error ? e.message : String(e), kind: 'error' });
    }
  };
  // Proto files are local, so their methods can be offered as soon as the
  // request opens. Reflection needs the server and stays behind the button.
  useEffect(() => {
    if (p.source !== 'proto') return;
    p.onLoad().then(
      (found) => {
        setMethods(found);
        setStatus({ text: `${found.length} ${found.length === 1 ? 'method' : 'methods'}`, kind: 'ok' });
      },
      (e) => setStatus({ text: e instanceof Error ? e.message : String(e), kind: 'error' }),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="flex items-center gap-2 px-3 pb-2.5">
      <input
        className={`${inputCls} h-8 flex-1 font-mono text-xs placeholder:font-sans placeholder:text-[13px]`}
        aria-label="Method"
        spellCheck={false}
        placeholder="package.Service/Method"
        list="grpc-methods"
        value={p.method}
        onChange={(e) => p.onChange(e.target.value, methods.includes(e.target.value))}
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

function Field({ label, hint, children }: { label: string; hint: string; children: React.ReactNode }) {
  return (
    <label className="flex items-center gap-3">
      <span className="w-20 shrink-0 cursor-help text-muted" title={hint}>
        {label}
      </span>
      {children}
    </label>
  );
}

const AUTH_HINTS: Record<Auth['type'], string> = { none: HINTS.authNone, bearer: HINTS.bearer, basic: HINTS.basic, apikey: HINTS.apikey };

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
      <Field label="Type" hint={HINTS.authType}>
        <Select className="w-44" title={AUTH_HINTS[auth.type]} value={auth.type} onChange={(e) => onChange({ ...auth, type: e.target.value as Auth['type'] })}>
          <option value="none" title={AUTH_HINTS.none}>
            No auth
          </option>
          <option value="bearer" title={AUTH_HINTS.bearer}>
            Bearer token
          </option>
          <option value="basic" title={AUTH_HINTS.basic}>
            Basic auth
          </option>
          <option value="apikey" title={AUTH_HINTS.apikey}>
            API key
          </option>
        </Select>
      </Field>
      {auth.type === 'none' && <p className="text-muted">This request is sent without credentials.</p>}
      {auth.type === 'bearer' && <Field label="Token" hint={HINTS.token}>{text('token', '{{token}}')}</Field>}
      {auth.type === 'basic' && (
        <>
          <Field label="Username" hint={HINTS.username}>{text('username', 'Username')}</Field>
          <Field label="Password" hint={HINTS.password}>{text('password', '{{password}}')}</Field>
        </>
      )}
      {auth.type === 'apikey' && (
        <>
          <Field label="Key" hint={HINTS.apiKeyName}>{text('key', 'X-API-Key')}</Field>
          <Field label="Value" hint={HINTS.apiKeyValue}>{text('value', '{{apiKey}}')}</Field>
          {!grpc && (
            <Field label="Add to" hint={HINTS.apiKeyIn}>
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
  const modes: { id: Body['mode']; label: string; title: string }[] = [
    { id: 'none', label: 'None', title: HINTS.bodyNone },
    { id: 'raw', label: 'Raw', title: HINTS.bodyRaw },
    { id: 'urlencoded', label: 'URL-encoded', title: HINTS.bodyUrlencoded },
    { id: 'form', label: 'Form data', title: HINTS.bodyForm },
  ];
  const types = CONTENT_TYPES.includes(body.contentType) || !body.contentType ? CONTENT_TYPES : [...CONTENT_TYPES, body.contentType];
  const pretty = body.mode === 'raw' ? prettyJson(body.raw) : null;

  return (
    <div className="flex flex-col gap-3 h-full">
      <div className="flex flex-wrap items-center gap-2">
        <Segmented label="Body type" options={modes} value={body.mode} onChange={(mode) => onChange({ ...body, mode })} />
        {body.mode === 'raw' && (
          <>
            <Select
              className="w-40 @xl:ml-auto"
              aria-label="Content type"
              title={HINTS.contentType}
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
      {body.mode === 'raw' && body.contentType.includes('json') && (
        <JsonEditor label="Request body" placeholder='{"hello": "{{name}}"}' value={body.raw} onChange={(raw) => onChange({ ...body, raw })} />
      )}
      {body.mode === 'raw' && !body.contentType.includes('json') && (
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
        <KVEditor
          rows={body.form}
          onChange={(form) => onChange({ ...body, form })}
          keyPlaceholder="Field"
          keyHint={HINTS.formKey}
          valueHint={HINTS.formValue}
        />
      )}
      {body.mode === 'none' && <p className="text-muted">This request has no body.</p>}
    </div>
  );
}
