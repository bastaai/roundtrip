import type { KV } from '../types';
import { IconButton } from './ui';

type Props = {
  rows: KV[];
  onChange: (rows: KV[]) => void;
  toggle?: boolean;
  secrets?: boolean;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  // Shown when the pointer rests on the column headings.
  keyHint?: string;
  valueHint?: string;
};

// Shared by the grids (here and in TestsEditor): rows are separated by hairlines only, no cell boxes.
export const gridHeadCls = 'flex h-7 items-center gap-1 border-b border-line text-[11px] font-medium text-muted';
export const gridRowCls = 'group flex h-8 items-center gap-1 border-b border-line/70';
export const gridCellCls =
  'h-6 min-w-0 rounded-[5px] bg-transparent px-2 font-mono text-xs outline-none transition-colors duration-100 placeholder:font-sans placeholder:text-faint hover:bg-hover/60 focus:bg-hover focus:shadow-focus';
// Row actions stay out of the way until the row is hovered or focused.
export const gridActionsCls = 'opacity-0 transition-opacity duration-100 group-hover:opacity-100 group-focus-within:opacity-100';

// One value per row, same grid as KVEditor.
export function ListEditor(p: { rows: string[]; onChange: (rows: string[]) => void; label: string; placeholder: string; autoFocus?: boolean }) {
  const all = [...p.rows, ''];
  const set = (i: number, value: string) => p.onChange(all.map((row, j) => (j === i ? value : row)).filter((row, j) => j < p.rows.length || row));
  return (
    <div className="-mx-2">
      <div className={gridHeadCls}>
        <span className="flex-1 px-2">{p.label}</span>
      </div>
      {all.map((row, i) => {
        const blank = i === p.rows.length;
        return (
          <div key={i} className={`${gridRowCls} ${blank ? 'border-transparent' : ''}`}>
            <input
              autoFocus={p.autoFocus && i === 0}
              className={`${gridCellCls} flex-1`}
              placeholder={blank ? p.placeholder : ''}
              aria-label={p.label}
              spellCheck={false}
              value={row}
              onChange={(e) => set(i, e.target.value)}
            />
            <div className={`flex w-6 shrink-0 items-center justify-end ${blank ? 'invisible' : ''}`}>
              <span className={gridActionsCls}>
                <IconButton icon="trash" title="Remove" danger onClick={() => p.onChange(p.rows.filter((_, j) => j !== i))} />
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

// Always shows one blank row at the end; typing into it adds a row.
export function KVEditor({ rows, onChange, toggle = true, secrets = false, keyPlaceholder = 'Key', valuePlaceholder = 'Value', keyHint, valueHint }: Props) {
  const all = [...rows, { key: '', value: '', enabled: true }];
  const set = (i: number, patch: Partial<KV>) => {
    const next = all.map((row, j) => (j === i ? { ...row, ...patch } : row));
    onChange(next.filter((row, j) => j < rows.length || row.key || row.value));
  };
  const actionsCls = `flex shrink-0 items-center justify-end ${secrets ? 'w-[52px]' : 'w-6'}`;

  return (
    <div className="-mx-2">
      <div className={gridHeadCls}>
        {toggle && <span className="w-6 shrink-0" />}
        <span className={`flex-[2] px-2 ${keyHint ? 'cursor-help' : ''}`} title={keyHint}>
          {keyPlaceholder}
        </span>
        <span className={`flex-[3] px-2 ${valueHint ? 'cursor-help' : ''}`} title={valueHint}>
          {valuePlaceholder}
        </span>
        <span className={actionsCls} />
      </div>
      {all.map((row, i) => {
        const blank = i === rows.length;
        const dim = row.enabled ? '' : 'text-faint line-through';
        return (
          <div key={i} className={`${gridRowCls} ${blank ? 'border-transparent' : ''}`}>
            {toggle && (
              <span className="grid w-6 shrink-0 place-items-center">
                <input
                  type="checkbox"
                  className={blank ? 'invisible' : ''}
                  aria-label={row.enabled ? 'Disable row' : 'Enable row'}
                  checked={row.enabled}
                  onChange={(e) => set(i, { enabled: e.target.checked })}
                />
              </span>
            )}
            <input
              className={`${gridCellCls} flex-[2] ${dim}`}
              placeholder={blank ? `Add ${keyPlaceholder.toLowerCase()}` : keyPlaceholder}
              aria-label={keyPlaceholder}
              spellCheck={false}
              value={row.key}
              onChange={(e) => set(i, { key: e.target.value })}
            />
            <input
              className={`${gridCellCls} flex-[3] ${dim}`}
              placeholder={valuePlaceholder}
              aria-label={valuePlaceholder}
              spellCheck={false}
              type={row.secret ? 'password' : 'text'}
              value={row.value}
              onChange={(e) => set(i, { value: e.target.value })}
            />
            <div className={`${actionsCls} ${blank ? 'invisible' : ''}`}>
              {secrets && (
                <span className={row.secret ? '' : gridActionsCls}>
                  <IconButton
                    icon="lock"
                    title={row.secret ? 'Masked on screen (click to show)' : 'Mask this value on screen'}
                    active={row.secret}
                    onClick={() => set(i, { secret: !row.secret })}
                  />
                </span>
              )}
              <span className={gridActionsCls}>
                <IconButton icon="trash" title="Remove" danger onClick={() => onChange(rows.filter((_, j) => j !== i))} />
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
