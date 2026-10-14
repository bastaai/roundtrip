import { useMemo, useState } from 'react';
import { formatSize, highlightJson, prettyJson } from '../lib/request';
import type { Res } from '../types';
import { Count, ghostBtnCls, Icon, Kbd, KEYS, Segmented, Spinner, statusColor, Tabs, useCopied } from './ui';

const HIGHLIGHT_LIMIT = 500_000;
const preCls = 'p-3 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-all';

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
      <Placeholder>
        {sending ? (
          <>
            <Progress />
            <span className="flex items-center gap-2 text-muted">
              <Spinner /> Sending request…
            </span>
          </>
        ) : (
          <>
            <span className="text-muted">Enter a URL and send the request to see the response here.</span>
            <span className="flex items-center gap-4 text-muted">
              <span className="flex items-center gap-1.5">
                <Kbd keys={KEYS.send} /> Send
              </span>
              <span className="flex items-center gap-1.5">
                <Kbd keys={KEYS.save} /> Save
              </span>
            </span>
          </>
        )}
      </Placeholder>
    );
  }

  // While a new request is in flight the previous response stays in place, dimmed, so nothing jumps.
  const busy = sending ? 'opacity-50 transition-opacity duration-150' : '';

  const tests = res.tests ?? [];
  const failed = tests.filter((t) => !t.passed).length;
  const view = tab === 'tests' && tests.length === 0 ? 'body' : tab;

  if (res.error) {
    return (
      <div className="relative h-full overflow-auto">
        {sending && <Progress />}
        <div className={`flex max-w-3xl flex-col gap-2.5 p-3 ${busy}`}>
          <div className="font-medium text-danger">Could not send request</div>
          <pre className="rounded-md bg-panel p-3 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-words">{res.error}</pre>
          <p className="text-muted">
            Check the URL and that the server is reachable. If the request uses <code className="font-mono text-xs">{'{{variables}}'}</code>, make sure the
            right environment is selected.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="relative flex h-full min-h-0 flex-col">
      {sending && <Progress />}
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
          <>
            <span className="flex min-w-0 items-center gap-3 pr-1 text-xs tabular-nums">
              <span className={`truncate font-medium ${statusColor(res.status, res.grpc)}`}>
                {res.grpc ? `${res.statusText} (${res.status})` : `${res.status} ${res.statusText}`}
              </span>
              <span className="shrink-0 text-muted" title="Time">
                {formatTime(res.timeMs)}
              </span>
              <span className="shrink-0 text-muted" title="Size">
                {formatSize(res.size)}
              </span>
            </span>
            {view === 'body' && !res.base64 && (
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
            )}
          </>
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
    </div>
  );
}

function Progress() {
  return (
    <div className="pointer-events-none absolute inset-x-0 top-0 z-[1] h-0.5 overflow-hidden" role="progressbar" aria-label="Sending request">
      <div className="h-full w-2/5 bg-accent animate-progress" />
    </div>
  );
}

function Placeholder({ children }: { children: React.ReactNode }) {
  return <div className="relative flex h-full flex-col items-center justify-center gap-3 px-6 text-center">{children}</div>;
}
