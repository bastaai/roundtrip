import { useEffect, useMemo, useState } from 'react';
import { listRequests, rank, type RequestEntry } from '../lib/quickopen';
import { kindOf } from '../lib/request';
import type { Collection, Item } from '../types';
import { Icon, Kbd, Method } from './ui';

export type QuickAction = { id: string; label: string; detail?: string; keys?: string[]; run: () => void };

type Row = { id: string; request?: RequestEntry; action?: QuickAction };

const MAX_REQUESTS = 50;
const SEP = ' / ';

// Jump to any saved request by name or path, or run a common action. The text field keeps
// focus the whole time; the arrow keys move the highlighted option.
export function QuickOpen(p: { collections: Collection[]; actions: QuickAction[]; onOpen: (collection: Collection, item: Item) => void; onClose: () => void }) {
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const requests = useMemo(() => listRequests(p.collections), [p.collections]);

  const typed = query.trim() !== '';
  const found = rank(query, requests, (e) => ({ name: e.item.name, context: `${e.path.join(SEP)} ${kindOf(e.item.request!)} ${e.item.request!.method}` }));
  const requestRows: Row[] = found.slice(0, MAX_REQUESTS).map((request) => ({ id: `qo-r-${request.item.id}`, request }));
  const actionRows: Row[] = rank(query, p.actions, (a) => ({ name: a.label, context: a.detail })).map((action) => ({ id: `qo-a-${action.id}`, action }));
  // Typing means looking for a request, so those lead; an empty field offers the actions first.
  const groups = (typed ? [{ label: 'Requests', rows: requestRows }, { label: 'Actions', rows: actionRows }] : [{ label: 'Actions', rows: actionRows }, { label: 'Requests', rows: requestRows }]).filter(
    (g) => g.rows.length,
  );
  const rows = groups.flatMap((g) => g.rows);
  const current = rows[Math.min(active, rows.length - 1)];

  // Read while rendering: by the time an effect runs, the field below already has focus.
  const [before] = useState(() => document.activeElement as HTMLElement | null);
  useEffect(
    () => () => {
      // Whatever opens next may already have taken focus; only hand it back if nothing has.
      if (!document.activeElement || document.activeElement === document.body) before?.focus?.();
    },
    [before],
  );

  useEffect(() => {
    if (current) document.getElementById(current.id)?.scrollIntoView({ block: 'nearest' });
  }, [current?.id]);

  const run = (row?: Row) => {
    if (!row) return;
    p.onClose();
    if (row.request) p.onOpen(row.request.collection, row.request.item);
    else row.action?.run();
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const move = (to: number) => {
      e.preventDefault();
      if (rows.length) setActive((to + rows.length) % rows.length);
    };
    const at = Math.min(active, rows.length - 1);
    if (e.key === 'ArrowDown') move(at + 1);
    else if (e.key === 'ArrowUp') move(at - 1);
    else if (e.key === 'PageDown') move(Math.min(at + 8, rows.length - 1));
    else if (e.key === 'PageUp') move(Math.max(at - 8, 0));
    else if (e.key === 'Enter') {
      e.preventDefault();
      run(current);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      p.onClose();
    } else if (e.key === 'Tab') {
      // The field is the only stop inside the dialog.
      e.preventDefault();
    }
  };

  return (
    <div className="fixed inset-0 z-30 flex items-start justify-center bg-overlay px-4 pt-[11vh] animate-fade" onMouseDown={p.onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Go to a request or run an action"
        className="flex max-h-[min(70vh,520px)] w-full max-w-[620px] flex-col overflow-hidden rounded-xl bg-bg shadow-pop animate-pop"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <label className="flex h-12 shrink-0 items-center gap-2.5 border-b border-line px-4 text-muted">
          <Icon name="search" />
          <input
            autoFocus
            role="combobox"
            aria-expanded="true"
            aria-controls="qo-list"
            aria-autocomplete="list"
            aria-activedescendant={current?.id}
            aria-label="Go to a request or run an action"
            className="h-full min-w-0 flex-1 bg-transparent text-[14px] text-fg outline-none placeholder:text-faint"
            spellCheck={false}
            autoComplete="off"
            placeholder="Go to a request by name or path…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onKeyDown}
          />
        </label>
        <div id="qo-list" role="listbox" aria-label="Results" className="min-h-0 flex-1 overflow-auto p-1.5 scroll-py-1.5">
          {rows.length === 0 && <p className="px-2.5 py-3 text-muted">Nothing matches “{query.trim()}”.</p>}
          {groups.map((g) => (
            <div key={g.label} role="group" aria-label={g.label}>
              <div className="flex h-7 items-center px-2.5 text-[11px] font-medium text-muted" aria-hidden>
                {g.label}
              </div>
              {g.rows.map((row) => {
                const selected = row.id === current?.id;
                return (
                  <div
                    key={row.id}
                    id={row.id}
                    role="option"
                    aria-selected={selected}
                    className={`flex h-8 cursor-pointer items-center gap-2 rounded-md px-2.5 ${selected ? 'bg-active' : ''}`}
                    onMouseMove={() => !selected && setActive(rows.indexOf(row))}
                    onClick={() => run(row)}
                  >
                    {row.request ? (
                      <>
                        <Method request={row.request.item.request!} />
                        <span className="min-w-0 shrink truncate">{row.request.item.name}</span>
                        <span className="ml-auto min-w-0 max-w-[55%] shrink-[2] truncate pl-3 text-xs text-muted">{row.request.path.join(SEP)}</span>
                      </>
                    ) : (
                      <>
                        <span className="min-w-0 truncate">{row.action!.label}</span>
                        <span className="ml-auto flex shrink-0 items-center gap-2 pl-3 text-xs text-muted">
                          {row.action!.detail}
                          {row.action!.keys && <Kbd keys={row.action!.keys} />}
                        </span>
                      </>
                    )}
                  </div>
                );
              })}
              {g.label === 'Requests' && found.length > MAX_REQUESTS && (
                <p className="px-2.5 py-1.5 text-xs text-muted">{found.length - MAX_REQUESTS} more. Keep typing to narrow them down.</p>
              )}
            </div>
          ))}
        </div>
        <div className="flex h-8 shrink-0 items-center gap-4 border-t border-line px-4 text-xs text-muted" aria-hidden>
          <span className="flex items-center gap-1.5">
            <Kbd keys={['↑', '↓']} /> Move
          </span>
          <span className="flex items-center gap-1.5">
            <Kbd keys={['↵']} /> Open
          </span>
          <span className="flex items-center gap-1.5">
            <Kbd keys={['Esc']} /> Close
          </span>
        </div>
      </div>
    </div>
  );
}
