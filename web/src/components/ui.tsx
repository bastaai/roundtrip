import { useEffect, useRef, useState, type ReactNode, type SelectHTMLAttributes } from 'react';
import { kindOf } from '../lib/request';
import type { Req } from '../types';

export const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);
// Shortcut labels as each OS writes them; the handlers bind both ⌘ and Ctrl.
export const KEYS = {
  send: isMac ? ['⌘', '↵'] : ['Ctrl', 'Enter'],
  save: isMac ? ['⌘', 'S'] : ['Ctrl', 'S'],
  quickOpen: isMac ? ['⌘', 'K'] : ['Ctrl', 'K'],
  sidebar: isMac ? ['⌘', 'B'] : ['Ctrl', 'B'],
};
// The platform's own modifier only: on macOS Ctrl+K and Ctrl+B are text-editing keys.
export const isMod = (e: { metaKey: boolean; ctrlKey: boolean }) => (isMac ? e.metaKey : e.ctrlKey);
export const keyLabel = (keys: string[]) => keys.join(isMac ? '' : '+');

const focusField =
  'outline-none hover:border-line-strong focus:border-focus focus:ring-2 focus:ring-focus/25 transition-[border-color,box-shadow] duration-100';
// fieldCls has no height (textareas); inputCls is the 28px single-line control.
export const fieldCls = `min-w-0 rounded-md border border-line bg-raised px-2 placeholder:text-faint disabled:opacity-50 ${focusField}`;
export const inputCls = `${fieldCls} h-7`;

const btnBase =
  'inline-flex shrink-0 items-center justify-center gap-1.5 h-7 px-2.5 rounded-md whitespace-nowrap select-none transition-colors duration-100 disabled:opacity-45 disabled:pointer-events-none';
export const btnCls = `${btnBase} border border-line bg-raised hover:bg-hover hover:border-line-strong active:bg-active`;
export const ghostBtnCls = `${btnBase} text-muted hover:bg-hover hover:text-fg active:bg-active`;
export const primaryBtnCls = `${btnBase} bg-accent text-accent-fg font-medium hover:bg-accent-hover active:bg-accent`;

const METHOD_COLORS: Record<string, string> = {
  GET: 'text-m-get',
  POST: 'text-m-post',
  PUT: 'text-m-put',
  PATCH: 'text-m-patch',
  DELETE: 'text-m-delete',
};
const METHOD_SHORT: Record<string, string> = { DELETE: 'DEL', OPTIONS: 'OPT' };

export const methodColor = (method: string) => METHOD_COLORS[method] ?? 'text-muted';

export function Method({ request }: { request: Pick<Req, 'method' | 'protocol'> & { body?: Pick<Req['body'], 'mode'> } }) {
  const kind = kindOf(request);
  const [color, label] =
    kind === 'grpc'
      ? ['text-m-grpc', 'gRPC']
      : kind === 'graphql'
        ? ['text-m-graphql', 'GQL']
        : [methodColor(request.method), METHOD_SHORT[request.method] ?? request.method];
  return <span className={`${color} w-9 shrink-0 text-[11px] leading-none font-medium tracking-tight`}>{label}</span>;
}

export const statusColor = (status: number, grpc = false) =>
  grpc ? (status === 0 ? 'text-ok' : 'text-danger') : status >= 500 ? 'text-danger' : status >= 400 ? 'text-warn' : status >= 300 ? 'text-info' : 'text-ok';

const ICONS = {
  plus: 'M5 12h14M12 5v14',
  x: 'M18 6 6 18M6 6l12 12',
  check: 'M20 6 9 17l-5-5',
  chevron: 'm9 18 6-6-6-6',
  chevronDown: 'm6 9 6 6 6-6',
  trash: 'M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2',
  pencil: 'M12 20h9M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z',
  folder:
    'M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2Z',
  folderPlus:
    'M4 20h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.93a2 2 0 0 1-1.66-.9l-.82-1.2A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13c0 1.1.9 2 2 2ZM12 10v6M9 13h6',
  filePlus: 'M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2zM12 18v-6M9 15h6',
  lock: 'M5 11h14v10H5zM8 11V7a4 4 0 0 1 8 0v4',
  refresh: 'M21 12a9 9 0 1 1-3-6.7L21 8M21 3v5h-5',
  repeat: 'm17 2 4 4-4 4M3 11v-1a4 4 0 0 1 4-4h14M7 22l-4-4 4-4M21 13v1a4 4 0 0 1-4 4H3',
  copy: 'M10 9h10a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H10a1 1 0 0 1-1-1V10a1 1 0 0 1 1-1zM5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1',
  branch: 'M6 3v12M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18 9a9 9 0 0 1-9 9',
  alert: 'M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0zM12 9v4M12 17h.01',
  clock: 'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20zM12 6v6l4 2',
  search: 'M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.3-4.3',
  sidebar: 'M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2zM9 3v18',
  columns: 'M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2zM12 3v18',
  rows: 'M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2zM3 12h18',
  sliders: 'M4 7h9M19 7h1M13 7a3 3 0 1 0 6 0a3 3 0 1 0-6 0M4 17h1M11 17h9M5 17a3 3 0 1 0 6 0a3 3 0 1 0-6 0',
};
export type IconName = keyof typeof ICONS;

export function Icon({ name, size = 14, strokeWidth = 2 }: { name: IconName; size?: number; strokeWidth?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="round"
      strokeLinejoin="round"
      className="shrink-0"
      aria-hidden
    >
      <path d={ICONS[name]} />
    </svg>
  );
}

export function Spinner({ size = 14 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" className="shrink-0 animate-spin" aria-hidden>
      <path d="M12 3a9 9 0 1 0 9 9" />
    </svg>
  );
}

export function IconButton(p: { icon: IconName; title: string; onClick: () => void; active?: boolean; danger?: boolean; disabled?: boolean }) {
  return (
    <button
      type="button"
      title={p.title}
      aria-label={p.title}
      disabled={p.disabled}
      onClick={(e) => {
        e.stopPropagation();
        p.onClick();
      }}
      className={`grid size-6 shrink-0 place-items-center rounded-md transition-colors duration-100 hover:bg-active disabled:opacity-40 disabled:pointer-events-none ${
        p.danger ? 'hover:text-danger' : 'hover:text-fg'
      } ${p.active ? 'text-fg' : 'text-muted'}`}
    >
      <Icon name={p.icon} />
    </button>
  );
}

export function Kbd({ keys }: { keys: string[] }) {
  return (
    <span className="inline-flex gap-0.5">
      {keys.map((k) => (
        <kbd key={k} className="min-w-5 rounded bg-hover px-1 text-center font-sans text-[11px] leading-[18px] text-muted">
          {k}
        </kbd>
      ))}
    </span>
  );
}

export function Count({ n }: { n: number }) {
  return <span className="ml-1.5 text-[11px] font-normal tabular-nums text-muted">{n}</span>;
}

export function Dot() {
  return <span className="ml-1.5 inline-block size-1 rounded-full bg-muted align-middle" />;
}

// A native <select> (so the OS draws the list and type-ahead works) with our own frame and chevron.
// bare drops the frame for use inside a composite control.
export function Select({
  className = '',
  selectClassName = '',
  bare,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement> & { selectClassName?: string; bare?: boolean }) {
  return (
    <span className={`relative inline-flex min-w-0 ${className}`}>
      <select
        {...props}
        className={`w-full appearance-none truncate pr-6 ${
          bare ? 'h-full rounded-[5px] bg-transparent pl-2.5 outline-none hover:bg-hover focus-visible:bg-hover transition-colors duration-100' : inputCls
        } ${selectClassName}`}
      />
      <span className="pointer-events-none absolute right-1.5 top-1/2 -translate-y-1/2 text-muted">
        <Icon name="chevronDown" size={12} />
      </span>
    </span>
  );
}

export function Segmented<T extends string>(p: { options: { id: T; label: ReactNode }[]; value: T; onChange: (id: T) => void; label: string }) {
  return (
    <div role="radiogroup" aria-label={p.label} className="inline-flex shrink-0 gap-0.5 rounded-md bg-hover p-0.5">
      {p.options.map((o) => (
        <button
          key={o.id}
          type="button"
          role="radio"
          aria-checked={o.id === p.value}
          onClick={() => p.onChange(o.id)}
          className={`h-6 rounded-[5px] px-2 whitespace-nowrap transition-colors duration-100 ${
            o.id === p.value ? 'bg-raised text-fg shadow-seg' : 'text-muted hover:text-fg'
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Tabs<T extends string>(p: { tabs: { id: T; label: ReactNode }[]; active: T; onChange: (id: T) => void; right?: ReactNode }) {
  return (
    <div className="flex h-9 shrink-0 items-stretch gap-1 border-b border-line px-3">
      <div role="tablist" className="-mx-1.5 flex items-stretch gap-1">
        {p.tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={t.id === p.active}
            onClick={() => p.onChange(t.id)}
            className={`relative -mb-px flex items-center border-b-2 px-1.5 whitespace-nowrap transition-colors duration-100 ${
              t.id === p.active ? 'border-accent text-fg font-medium' : 'border-transparent text-muted hover:text-fg'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>
      {p.right && <div className="ml-auto flex min-w-0 items-center gap-1.5 pl-2">{p.right}</div>}
    </div>
  );
}

// A 1px divider with a wider grab area. Drag, arrow keys, or double-click to reset.
export function ResizeHandle(p: { axis: 'x' | 'y'; label: string; onMove: (position: number) => void; onNudge: (delta: number) => void; onReset: () => void }) {
  const [dragging, setDragging] = useState(false);
  const x = p.axis === 'x';
  return (
    <div
      role="separator"
      tabIndex={0}
      aria-orientation={x ? 'vertical' : 'horizontal'}
      aria-label={p.label}
      title={`${p.label}: drag to resize, double-click to reset`}
      className={`group relative z-[5] shrink-0 bg-line touch-none ${x ? 'w-px cursor-col-resize' : 'h-px cursor-row-resize'}`}
      onPointerDown={(e) => {
        e.preventDefault();
        e.currentTarget.setPointerCapture(e.pointerId);
        setDragging(true);
      }}
      onPointerMove={(e) => dragging && p.onMove(x ? e.clientX : e.clientY)}
      onPointerUp={() => setDragging(false)}
      onPointerCancel={() => setDragging(false)}
      onDoubleClick={p.onReset}
      onKeyDown={(e) => {
        const step = (x ? { ArrowLeft: -16, ArrowRight: 16 } : { ArrowUp: -16, ArrowDown: 16 })[e.key as 'ArrowLeft'];
        if (!step) return;
        e.preventDefault();
        p.onNudge(step);
      }}
    >
      <div
        className={`absolute transition-colors duration-100 group-hover:bg-focus/40 group-hover:delay-150 ${dragging ? 'bg-focus/60' : ''} ${
          x ? 'inset-y-0 -left-[3px] w-[7px]' : 'inset-x-0 -top-[3px] h-[7px]'
        }`}
      />
    </div>
  );
}

const FOCUSABLE = 'a[href],button:not(:disabled),input:not(:disabled):not([type="hidden"]),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]';

// Escape closes; Tab stays inside; focus returns to where it was.
export function Modal(p: { title: string; onClose: () => void; children: ReactNode; width?: string; footer?: ReactNode; bodyClass?: string }) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const before = document.activeElement as HTMLElement | null;
    if (!ref.current?.contains(document.activeElement)) ref.current?.focus();
    return () => before?.focus?.();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        p.onClose();
      } else if (e.key === 'Tab' && ref.current) {
        const items = [...ref.current.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.offsetParent !== null);
        if (!items.length) return;
        const first = items[0];
        const last = items[items.length - 1];
        const current = document.activeElement;
        if (!ref.current.contains(current) || current === ref.current) {
          e.preventDefault();
          (e.shiftKey ? last : first).focus();
        } else if (e.shiftKey && current === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && current === last) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  return (
    <div className="fixed inset-0 z-30 flex items-start justify-center bg-overlay px-4 pt-[11vh] animate-fade" onMouseDown={p.onClose}>
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-label={p.title}
        tabIndex={-1}
        className={`flex max-h-[78vh] w-full flex-col rounded-xl bg-bg shadow-pop outline-none animate-pop ${p.width ?? 'max-w-[560px]'}`}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="flex h-11 shrink-0 items-center justify-between border-b border-line pl-4 pr-2.5">
          <h2 className="font-medium">{p.title}</h2>
          <IconButton icon="x" title="Close (Esc)" onClick={p.onClose} />
        </div>
        <div className={`min-h-0 flex-1 overflow-auto ${p.bodyClass ?? 'p-4'}`}>{p.children}</div>
        {p.footer && <div className="flex shrink-0 items-center justify-end gap-2 border-t border-line px-4 py-3">{p.footer}</div>}
      </div>
    </div>
  );
}

export function useCopied(): [boolean, (text: string) => void] {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 1400);
    return () => clearTimeout(t);
  }, [copied]);
  return [
    copied,
    (text) => {
      navigator.clipboard.writeText(text);
      setCopied(true);
    },
  ];
}
