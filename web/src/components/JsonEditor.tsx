import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { applyCompletion, enumCompletion, type EnumCompletion } from '../lib/enums';
import { highlightJson } from '../lib/highlight';
import type { GrpcEnum } from '../types';
import { useVariableHover, useVariables } from './Variables';

const runCls = { key: 'json-key', str: 'json-str', num: 'json-num', lit: 'json-lit', var: 'json-var' };
// The textarea and the coloured copy under it must lay text out identically.
const textCls = 'm-0 px-2 py-1.5 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-words [scrollbar-gutter:stable]';

// A textarea for JSON: the text is drawn coloured underneath a transparent
// textarea, so editing stays native. With enums, the values of an enum field
// are offered at the caret; arrows choose, Enter or Tab accepts, Esc closes,
// Ctrl+Space opens it again.
export function JsonEditor(p: { value: string; onChange: (value: string) => void; placeholder?: string; label: string; enums?: GrpcEnum[] }) {
  const ta = useRef<HTMLTextAreaElement>(null);
  const under = useRef<HTMLPreElement>(null);
  const marker = useRef<HTMLSpanElement>(null);
  const list = useRef<HTMLUListElement>(null);
  const caretAfter = useRef<number | null>(null);
  const [caret, setCaret] = useState<number | null>(null);
  const [closedAt, setClosedAt] = useState<number | null>(null);
  const [chosen, setChosen] = useState(0);
  const [at, setAt] = useState({ top: 0, left: 0 });
  const vars = useVariables();
  const hover = useVariableHover();
  const box = useRef<HTMLDivElement>(null);

  const runs = useMemo(() => highlightJson(p.value), [p.value]);
  const completion: EnumCompletion | null = useMemo(
    () => (caret === null || caret === closedAt || !p.enums?.length ? null : enumCompletion(p.value, caret, p.enums)),
    [p.value, p.enums, caret, closedAt],
  );

  // Put the caret after a value that was just inserted.
  useLayoutEffect(() => {
    if (caretAfter.current === null || !ta.current) return;
    ta.current.setSelectionRange(caretAfter.current, caretAfter.current);
    setCaret(caretAfter.current);
    caretAfter.current = null;
  }, [p.value]);

  // Place the list under the caret, measured on a hidden copy of the text before it.
  useLayoutEffect(() => {
    if (!completion || !marker.current || !ta.current) return;
    setAt({ top: marker.current.offsetTop - ta.current.scrollTop + 18, left: Math.min(marker.current.offsetLeft, ta.current.clientWidth - 200) });
  }, [completion]);

  useEffect(() => {
    list.current?.children[chosen]?.scrollIntoView({ block: 'nearest' });
  }, [chosen]);

  const track = () => {
    const t = ta.current;
    if (!t) return;
    setCaret(t.selectionStart === t.selectionEnd ? t.selectionStart : null);
    setChosen(0);
  };

  // The textarea is on top, so the coloured spans under it are hit-tested by hand.
  const onMouseMove = (e: React.MouseEvent) => {
    if (hover.busy(e)) return;
    const spans = under.current?.querySelectorAll<HTMLElement>('[data-var]') ?? [];
    for (const span of spans) {
      for (const rect of span.getClientRects()) {
        if (e.clientX >= rect.left && e.clientX <= rect.right && e.clientY >= rect.top && e.clientY <= rect.bottom) {
          const outer = box.current!.getBoundingClientRect();
          hover.show({ name: span.dataset.var!, top: rect.bottom - outer.top + 4, left: rect.left - outer.left });
          return;
        }
      }
    }
    hover.hideSoon();
  };

  const accept = (value: string) => {
    if (!completion) return;
    const next = applyCompletion(p.value, completion, value);
    caretAfter.current = next.caret;
    p.onChange(next.text);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === ' ' && e.ctrlKey) {
      e.preventDefault();
      setClosedAt(null);
      track();
      return;
    }
    if (!completion) return;
    const n = completion.values.length;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      setChosen((i) => (i + (e.key === 'ArrowDown' ? 1 : n - 1)) % n);
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault();
      accept(completion.values[Math.min(chosen, n - 1)]);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      setClosedAt(caret);
    }
  };

  return (
    <div
      ref={box}
      onMouseMove={onMouseMove}
      onMouseLeave={hover.hideSoon}
      className="relative min-h-24 flex-1 rounded-md border border-line bg-raised transition-[border-color,box-shadow] duration-100 hover:border-line-strong focus-within:border-focus focus-within:ring-2 focus-within:ring-focus/25">
      <pre ref={under} aria-hidden className={`${textCls} pointer-events-none absolute inset-0 overflow-hidden`}>
        {runs.map((r, i) =>
          r.kind === 'var' ? (
            <span key={i} data-var={r.name} className={vars.lookup(r.name!) ? 'json-var' : 'json-var-missing'}>
              {r.text}
            </span>
          ) : r.kind ? (
            <span key={i} className={runCls[r.kind]}>
              {r.text}
            </span>
          ) : (
            r.text
          ),
        )}
        {'\n'}
      </pre>
      {completion && (
        <pre aria-hidden className={`${textCls} invisible absolute inset-0 overflow-hidden`}>
          {p.value.slice(0, completion.from)}
          <span ref={marker} />
        </pre>
      )}
      <textarea
        ref={ta}
        className={`${textCls} absolute inset-0 h-full w-full resize-none bg-transparent text-transparent caret-fg outline-none placeholder:text-faint`}
        aria-label={p.label}
        aria-autocomplete={p.enums ? 'list' : undefined}
        spellCheck={false}
        placeholder={p.placeholder}
        value={p.value}
        onChange={(e) => {
          p.onChange(e.target.value);
          track();
        }}
        onSelect={track}
        onBlur={() => setCaret(null)}
        onKeyDown={onKeyDown}
        onScroll={(e) => {
          if (under.current) under.current.scrollTop = e.currentTarget.scrollTop;
          hover.hide();
        }}
      />
      {(!completion || hover.editing.current) && hover.popover}
      {completion && (
        <ul
          ref={list}
          role="listbox"
          aria-label="Enum values"
          className="absolute z-10 max-h-48 min-w-40 overflow-auto rounded-md border border-line bg-raised py-1 font-mono text-xs shadow-lg"
          style={{ top: at.top, left: Math.max(at.left, 4) }}
        >
          {completion.values.map((v, i) => (
            <li
              key={v}
              role="option"
              aria-selected={i === chosen}
              className={`cursor-default px-2.5 py-0.5 ${i === chosen ? 'bg-hover text-fg' : 'text-muted'}`}
              onMouseDown={(e) => {
                e.preventDefault();
                accept(v);
              }}
              onMouseEnter={() => setChosen(i)}
            >
              {v}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
