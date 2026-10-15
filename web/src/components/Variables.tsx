import { createContext, useContext, useRef, useState } from 'react';
import type { Lookup } from '../lib/variables';

// How the editors find out what a {{variable}} will be sent as, and change it.
// set is missing when no environment is selected.
export const VariablesContext = createContext<{
  lookup: (name: string) => Lookup | undefined;
  environment: string;
  set?: (name: string, value: string) => void;
}>({ lookup: () => undefined, environment: '' });

export const useVariables = () => useContext(VariablesContext);

type Spot = { name: string; top: number; left: number };

// The popover for the {{variable}} under the pointer. show() it while the
// pointer is over one and hideSoon() when it leaves; hiding waits a moment so
// the pointer can cross into the popover, and never happens while editing.
export function useVariableHover() {
  const [spot, setSpot] = useState<Spot>();
  const editing = useRef(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const hideSoon = () => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => editing.current || setSpot(undefined), 200);
  };
  const show = (next: Spot) => {
    clearTimeout(timer.current);
    if (spot?.name !== next.name || Math.abs(spot.top - next.top) > 1 || Math.abs(spot.left - next.left) > 1) setSpot(next);
  };
  // True while the pointer should not move the popover: it is being edited or pointed at.
  const busy = (e: React.MouseEvent) => editing.current || (e.target instanceof HTMLElement && !!e.target.closest('[data-variable-popover]'));
  const popover = spot && (
    <VariablePopover
      key={spot.name}
      {...spot}
      onEditing={(on) => {
        editing.current = on;
        if (!on) setSpot(undefined);
      }}
      onEnter={() => clearTimeout(timer.current)}
      onLeave={hideSoon}
    />
  );
  return { show, hideSoon, hide: () => editing.current || setSpot(undefined), busy, popover, editing };
}

// What {{name}} is sent as. A value from the environment can be edited in
// place (Enter or leaving the field saves, Esc cancels) unless it is secret;
// one saved from a response cannot, since it wins over the environment.
export function VariablePopover(p: {
  name: string;
  top: number;
  left: number;
  onEditing: (editing: boolean) => void;
  onEnter: () => void;
  onLeave: () => void;
}) {
  const vars = useVariables();
  const shown = vars.lookup(p.name);
  const [draft, setDraft] = useState<string | null>(null);
  const editable = !!vars.set && (!shown || (shown.from === 'environment' && !shown.secret));
  const edit = () => {
    setDraft(shown?.value ?? '');
    p.onEditing(true);
  };
  const finish = (save: boolean) => {
    if (save && draft !== null && draft !== shown?.value) vars.set!(p.name, draft);
    setDraft(null);
    p.onEditing(false);
  };

  return (
    <div
      role="dialog"
      data-variable-popover
      aria-label={`Variable ${p.name}`}
      className="absolute z-10 w-max max-w-80 rounded-md border border-line bg-raised px-2.5 py-1.5 text-xs shadow-lg"
      style={{ top: p.top, left: Math.max(p.left, 4) }}
      onMouseEnter={p.onEnter}
      onMouseLeave={p.onLeave}
    >
      <div className="flex items-center font-mono">
        <span className="shrink-0">{p.name}</span>
        {(shown || draft !== null) && <span className="shrink-0 whitespace-pre text-muted"> = </span>}
        {draft !== null ? (
          <input
            autoFocus
            className="h-5 min-w-40 flex-1 rounded-[4px] bg-hover px-1 outline-none focus:shadow-focus"
            aria-label={`Value of ${p.name}`}
            spellCheck={false}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={() => finish(true)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') finish(true);
              if (e.key === 'Escape') {
                e.stopPropagation();
                finish(false);
              }
            }}
          />
        ) : (
          shown && (
            <button
              type="button"
              disabled={!editable}
              className={`min-w-0 break-all rounded-[4px] px-0.5 text-left ${editable ? 'hover:bg-hover' : 'cursor-default'}`}
              title={editable ? 'Click to edit' : undefined}
              onClick={edit}
            >
              {shown.secret ? '••••••' : shown.value === '' ? <span className="text-muted">(empty)</span> : shown.value}
            </button>
          )
        )}
      </div>
      <div className="mt-0.5 text-muted">
        {shown ? (
          shown.from === 'response' ? (
            'Saved from a response'
          ) : (
            `From ${vars.environment}${shown.secret ? ', secret' : ''}`
          )
        ) : draft !== null ? (
          `Adds it to ${vars.environment}`
        ) : vars.set ? (
          <>
            Not set in {vars.environment}, sent as written.{' '}
            <button type="button" className="text-fg underline underline-offset-2" onClick={edit}>
              Set it
            </button>
          </>
        ) : (
          'No environment selected, sent as written'
        )}
      </div>
    </div>
  );
}
