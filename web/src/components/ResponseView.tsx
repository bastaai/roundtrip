import { useMemo, useState } from 'react';
import { foldAll, jsonLines, toggleFold, visibleLines, type JsonLine } from '../lib/json';
import { formatSize, formatTime, prettyJson, runLabel } from '../lib/request';
import type { HistoryEntry, Res } from '../types';
import { Count, ghostBtnCls, Icon, Kbd, KEYS, Segmented, Select, Spinner, statusColor, Tabs, useCopied } from './ui';

// Above this the body is shown as plain text: thousands of foldable lines get slow.
const TREE_LIMIT = 500_000;
const preCls = 'p-3 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-all';

const SHORTCUTS: [string[], string][] = [
  [KEYS.send, 'Send'],
  [KEYS.quickOpen, 'Go to a request or run a command'],
  [KEYS.sidebar, 'Hide or show the sidebar'],
];

// runs are this request's earlier sends, newest first; run is the one shown.
export function ResponseView(p: { res?: Res; sending: boolean; runs: HistoryEntry[]; run?: string; onRun: (id: string) => void }) {
  const { res, sending } = p;
  const [tab, setTab] = useState<'body' | 'headers' | 'tests'>('body');
  const [pretty, setPretty] = useState(true);
  const [copied, copy] = useCopied();

  const formatted = useMemo(() => (res && !res.base64 ? prettyJson(res.body) : null), [res]);
  const text = pretty && formatted !== null ? formatted : (res?.body ?? '');
  const lines = useMemo(() => (res && formatted !== null && formatted.length < TREE_LIMIT ? jsonLines(JSON.parse(res.body)) : null), [res, formatted]);
  // Folds belong to the lines they were made on, so a new response starts unfolded.
  const [fold, setFold] = useState<{ lines: JsonLine[] | null; set: Set<number> }>({ lines: null, set: new Set() });
  const folded = fold.lines === lines ? fold.set : new Set<number>();
  const setFolded = (set: Set<number>) => setFold({ lines, set });

  if (!res) {
    return (
      <Column>
        {sending && <Progress />}
        <Heading>
          <span className="flex items-center gap-2 text-muted">
            {sending ? (
              <>
                <Spinner /> Sending request…
              </>
            ) : (
              'No response yet'
            )}
          </span>
        </Heading>
        {!sending && (
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 overflow-auto px-6 pb-10 text-center">
            <span className="max-w-xs text-muted">Send the request and the response comes back here.</span>
            <dl className="grid grid-cols-[auto_auto] items-center gap-x-3 gap-y-1.5 text-left text-muted">
              {SHORTCUTS.map(([keys, label]) => (
                <div key={label} className="contents">
                  <dt className="justify-self-end">
                    <Kbd keys={keys} />
                  </dt>
                  <dd>{label}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
      </Column>
    );
  }

  // While a new request is in flight the previous response stays in place, dimmed, so nothing jumps.
  const busy = sending ? 'opacity-50 transition-opacity duration-150' : '';

  const tests = res.tests ?? [];
  const failed = tests.filter((t) => !t.passed).length;
  const view = tab === 'tests' && tests.length === 0 ? 'body' : tab;

  if (res.error) {
    return (
      <Column>
        {sending && <Progress />}
        <Heading>
          <span className={`${resultCls} text-danger`}>Could not send request</span>
        </Heading>
        <div className={`flex min-h-0 max-w-3xl flex-1 flex-col gap-2.5 overflow-auto px-3 pb-3 pt-1.5 ${busy}`}>
          <pre className="rounded-md bg-panel p-3 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-words">{res.error}</pre>
          <p className="text-muted">
            Check the URL and that the server is reachable. If the request uses <code className="font-mono text-xs">{'{{variables}}'}</code>, make sure the
            right environment is selected.
          </p>
        </div>
      </Column>
    );
  }

  return (
    <Column>
      {sending && <Progress />}
      <Heading>
        <span className={`${resultCls} min-w-0 truncate ${statusColor(res.status, res.grpc)}`}>
          {res.grpc ? `${res.statusText} (${res.status})` : `${res.status} ${res.statusText}`}
        </span>
        <span className="shrink-0 text-muted" title="Time">
          {formatTime(res.timeMs)}
        </span>
        <span className="shrink-0 text-muted" title="Size">
          {formatSize(res.size)}
        </span>
        {p.runs.length > 1 && <Runs runs={p.runs} run={p.run} onRun={p.onRun} />}
      </Heading>
      <Tabs
        active={view}
        onChange={setTab}
        tabs={[
          { id: 'body' as const, label: 'Body' },
          { id: 'headers' as const, label: <>{res.grpc ? 'Metadata' : 'Headers'}<Count n={res.headers.length} /></> },
          ...(tests.length
            ? [
                {
                  id: 'tests' as const,
                  label: (
                    <>
                      Tests
                      <span className={`ml-1.5 text-[11px] font-normal tabular-nums ${failed ? 'text-danger' : 'text-ok'}`}>
                        {tests.length - failed}/{tests.length}
                      </span>
                    </>
                  ),
                },
              ]
            : []),
        ]}
        right={
          view === 'body' &&
          !res.base64 && (
            <>
              {lines && pretty && lines.some((l) => l.kind === 'open' && l.depth > 0) && (
                <button
                  className={ghostBtnCls}
                  onClick={() => setFolded(folded.size ? new Set() : foldAll(lines))}
                  title={folded.size ? 'Expand everything' : 'Collapse everything below the top level'}
                >
                  {folded.size ? 'Expand all' : 'Collapse all'}
                </button>
              )}
              {formatted !== null && (
                <Segmented
                  label="Body format"
                  value={pretty ? 'pretty' : 'raw'}
                  onChange={(v) => setPretty(v === 'pretty')}
                  options={[
                    { id: 'pretty', label: 'Pretty' },
                    { id: 'raw', label: 'Raw' },
                  ]}
                />
              )}
              <button className={`${ghostBtnCls} w-[72px]`} onClick={() => copy(text)} title="Copy the response body">
                <Icon name={copied ? 'check' : 'copy'} size={13} />
                {copied ? 'Copied' : 'Copy'}
              </button>
            </>
          )
        }
      />
      <div className={`min-h-0 flex-1 overflow-auto ${busy}`}>
        {view === 'tests' ? (
          <div className="py-1.5 text-xs">
            {tests.map((t, i) => (
              <div key={i} className="flex items-start gap-2.5 px-3 py-1">
                <span className={`mt-px shrink-0 ${t.passed ? 'text-ok' : 'text-danger'}`}>
                  <Icon name={t.passed ? 'check' : 'x'} size={13} strokeWidth={2.5} />
                </span>
                <span className="min-w-0 flex-1 break-words font-mono">
                  {t.op === 'extract' ? (
                    <>
                      save <span className="text-muted">{'{{'}{t.value}{'}}'}</span> from {t.target}
                    </>
                  ) : (
                    <>
                      {t.target} <span className="text-muted">{t.op}</span> {t.value}
                    </>
                  )}
                  {t.message && <span className="block font-sans text-danger">{t.message}</span>}
                </span>
              </div>
            ))}
          </div>
        ) : view === 'headers' ? (
          res.headers.length === 0 ? (
            <p className="p-3 text-muted">The response has no {res.grpc ? 'metadata' : 'headers'}.</p>
          ) : (
            <div className="py-1.5 font-mono text-xs">
              {res.headers.map((h, i) => (
                <div key={i} className="flex gap-4 px-3 py-1 hover:bg-hover/50">
                  <span className="w-[32%] max-w-64 shrink-0 break-words text-muted">{h.key}</span>
                  <span className="min-w-0 flex-1 break-all">{h.value}</span>
                </div>
              ))}
            </div>
          )
        ) : res.base64 ? (
          <p className="p-3 text-muted">Binary response ({formatSize(res.size)}), not shown.</p>
        ) : res.bodyNotKept ? (
          <p className="p-3 text-muted">The body was larger than 1 MB, so it was not kept with the history.</p>
        ) : res.body === '' ? (
          <p className="p-3 text-muted">The response body is empty.</p>
        ) : lines && pretty ? (
          <JsonTree lines={lines} folded={folded} onToggle={(i, deep) => setFolded(toggleFold(lines, folded, i, deep))} />
        ) : (
          <pre className={preCls}>{text}</pre>
        )}
      </div>
    </Column>
  );
}

const tokenCls: Record<JsonLine['kind'], string> = { str: 'json-str', num: 'json-num', lit: 'json-lit', empty: '', open: '', close: '' };

// Pretty JSON with a fold toggle on every object and array. Alt-click folds or unfolds everything inside.
function JsonTree({ lines, folded, onToggle }: { lines: JsonLine[]; folded: ReadonlySet<number>; onToggle: (i: number, deep: boolean) => void }) {
  return (
    <div className="py-3 pr-3 font-mono text-xs leading-[1.6]">
      {visibleLines(lines, folded).map((i) => {
        const l = lines[i];
        const close = folded.has(i) ? lines[l.end!] : null;
        const indent = `${l.depth * 2}ch`;
        return (
          <div key={i} className="relative whitespace-pre-wrap break-all" style={{ paddingLeft: `calc(${indent} + 1.5rem)` }}>
            {l.kind === 'open' && (
              <button
                className="absolute top-0 flex h-[1.6em] w-5 select-none items-center justify-center rounded text-faint hover:text-fg"
                style={{ left: `calc(${indent} + 0.25rem)` }}
                onClick={(e) => onToggle(i, e.altKey)}
                aria-expanded={!close}
                aria-label={close ? 'Expand' : 'Collapse'}
                title={`${close ? 'Expand' : 'Collapse'} (Alt-click: everything inside)`}
              >
                <Icon name={close ? 'chevron' : 'chevronDown'} size={11} strokeWidth={2.5} />
              </button>
            )}
            {l.key && (
              <>
                <span className="json-key">{l.key}</span>:{' '}
              </>
            )}
            <span className={tokenCls[l.kind]}>{l.text}</span>
            {close && (
              <>
                <button className="mx-1 rounded bg-hover px-1 text-muted hover:text-fg" onClick={(e) => onToggle(i, e.altKey)} title="Expand">
                  {l.size} {l.text === '{' ? (l.size === 1 ? 'key' : 'keys') : l.size === 1 ? 'item' : 'items'}
                </button>
                {close.text}
              </>
            )}
            {(close ?? l).comma && ','}
          </div>
        );
      })}
    </div>
  );
}

// The request's earlier responses. Choosing one shows it in place of the latest.
function Runs({ runs, run, onRun }: { runs: HistoryEntry[]; run?: string; onRun: (id: string) => void }) {
  const value = runs.some((h) => h.id === run) ? run! : '';
  return (
    <Select
      bare
      className="ml-auto self-center"
      selectClassName="text-xs text-muted"
      value={value}
      onChange={(e) => onRun(e.target.value)}
      title="Earlier responses to this request"
      aria-label="Earlier responses"
    >
      {!value && <option value="">This session</option>}
      {runs.map((h, i) => (
        <option key={h.id} value={h.id}>
          {i === 0 ? `Latest · ${runLabel(h)}` : runLabel(h)}
        </option>
      ))}
    </Select>
  );
}

function Progress() {
  return (
    <div className="pointer-events-none absolute inset-x-0 top-0 z-[1] h-0.5 overflow-hidden" role="progressbar" aria-label="Sending request">
      <div className="h-full w-2/5 bg-accent animate-progress" />
    </div>
  );
}

// What came back. The result leads the column; everything else sits under it.
function Column({ children }: { children: React.ReactNode }) {
  return (
    <section aria-label="Response" className="relative flex h-full min-h-0 flex-col">
      {children}
    </section>
  );
}

const resultCls = 'text-[15px] font-semibold tracking-tight';

function Heading({ children }: { children: React.ReactNode }) {
  return (
    <div role="status" className="flex h-9 shrink-0 items-baseline gap-3 px-3 leading-9 tabular-nums">
      {children}
    </div>
  );
}
