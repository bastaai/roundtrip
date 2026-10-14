import { useMemo, useState } from 'react';
import { formatSize, highlightJson, prettyJson } from '../lib/request';
import type { Res } from '../types';
import { Count, ghostBtnCls, Icon, Kbd, KEYS, Segmented, Spinner, statusColor, Tabs, useCopied } from './ui';

const HIGHLIGHT_LIMIT = 500_000;
const preCls = 'p-3 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-all';

const SHORTCUTS: [string[], string][] = [
  [KEYS.send, 'Send'],
  [KEYS.save, 'Save'],
  [KEYS.quickOpen, 'Go to a request'],
  [KEYS.sidebar, 'Hide or show the sidebar'],
];

const formatTime = (ms: number) => (ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(2)} s`);

export function ResponseView({ res, sending }: { res?: Res; sending: boolean }) {
  const [tab, setTab] = useState<'body' | 'headers' | 'tests'>('body');
  const [pretty, setPretty] = useState(true);
  const [copied, copy] = useCopied();

  const formatted = useMemo(() => (res && !res.base64 ? prettyJson(res.body) : null), [res]);
  const text = pretty && formatted !== null ? formatted : (res?.body ?? '');
  const html = useMemo(
    () => (formatted !== null && text.length < HIGHLIGHT_LIMIT ? highlightJson(text) : null),
    [formatted, text],
  );

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
        ) : res.body === '' ? (
          <p className="p-3 text-muted">The response body is empty.</p>
        ) : html !== null && pretty ? (
          <pre className={preCls} dangerouslySetInnerHTML={{ __html: html }} />
        ) : (
          <pre className={preCls}>{text}</pre>
        )}
      </div>
    </Column>
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
