import type { Assertion, Extract } from '../types';
import { gridActionsCls, gridCellCls, gridHeadCls, gridRowCls, KVEditor } from './KVEditor';
import { HINTS, OP_HINTS } from '../lib/hints';
import { IconButton, Select } from './ui';

const OPS: [Assertion['op'], string][] = [
  ['eq', 'equals'],
  ['ne', 'does not equal'],
  ['contains', 'contains'],
  ['exists', 'exists'],
  ['lt', 'less than'],
  ['gt', 'greater than'],
  ['matches', 'matches regex'],
];

const labelCls = 'text-xs font-medium text-fg';

type Props = {
  tests: Assertion[];
  extract: Extract[];
  onChange: (tests: Assertion[], extract: Extract[]) => void;
};

export function TestsEditor({ tests, extract, onChange }: Props) {
  // Like KVEditor: one blank row at the end, typing into it adds a test.
  const all: Assertion[] = [...tests, { target: '', op: 'eq', value: '' }];
  const set = (i: number, patch: Partial<Assertion>) => {
    const next = all.map((t, j) => (j === i ? { ...t, ...patch } : t));
    onChange(
      next.filter((t, j) => j < tests.length || t.target || t.value),
      extract,
    );
  };

  return (
    <div className="flex max-w-4xl flex-col gap-5">
      <div className="flex flex-col gap-1">
        <span className={labelCls}>Checks on the response</span>
        <div className="-mx-2">
          <div className={gridHeadCls}>
            <span className="flex-[3] cursor-help px-2" title={HINTS.target}>
              Target
            </span>
            <span className="w-36 shrink-0 cursor-help px-2.5" title={HINTS.check}>
              Check
            </span>
            <span className="flex-[3] cursor-help px-2" title={HINTS.expected}>
              Expected
            </span>
            <span className="w-6 shrink-0" />
          </div>
          {all.map((t, i) => {
            const blank = i === tests.length;
            return (
              <div key={i} className={`${gridRowCls} ${blank ? 'border-transparent' : ''}`}>
                <input
                  className={`${gridCellCls} flex-[3]`}
                  placeholder={blank ? 'Add target: status, body.user.id, header.Content-Type, time' : 'Target'}
                  aria-label="Target"
                  spellCheck={false}
                  value={t.target}
                  onChange={(e) => set(i, { target: e.target.value })}
                />
                <Select
                  bare
                  className="h-6 w-36 shrink-0"
                  selectClassName={`text-xs focus-visible:shadow-focus ${blank ? 'text-faint' : ''}`}
                  aria-label="Check"
                  title={OP_HINTS[t.op]}
                  value={t.op}
                  onChange={(e) => set(i, { op: e.target.value as Assertion['op'] })}
                >
                  {OPS.map(([op, label]) => (
                    <option key={op} value={op} title={OP_HINTS[op]}>
                      {label}
                    </option>
                  ))}
                </Select>
                <input
                  className={`${gridCellCls} flex-[3] disabled:hover:bg-transparent`}
                  placeholder={t.op === 'exists' ? '' : 'Expected value'}
                  aria-label="Expected value"
                  spellCheck={false}
                  disabled={t.op === 'exists'}
                  value={t.op === 'exists' ? '' : (t.value ?? '')}
                  onChange={(e) => set(i, { value: e.target.value })}
                />
                <div className={`flex w-6 shrink-0 justify-end ${blank ? 'invisible' : gridActionsCls}`}>
                  <IconButton icon="trash" title="Remove" danger onClick={() => onChange(tests.filter((_, j) => j !== i), extract)} />
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex flex-col gap-1">
        <span className={labelCls}>Save values from the response as variables</span>
        <KVEditor
          toggle={false}
          rows={extract.map((e) => ({ key: e.var, value: e.from, enabled: true }))}
          onChange={(rows) => onChange(tests, rows.map((r) => ({ var: r.key, from: r.value })))}
          keyPlaceholder="Variable"
          valuePlaceholder="From (body.access_token, header.Location, status)"
          keyHint={HINTS.extractVar}
          valueHint={HINTS.extractFrom}
        />
      </div>

      <p className="text-xs leading-relaxed text-muted">
        Checks run every time the request is sent, here and in <code className="font-mono">roundtrip run</code>. Saved values are available to later
        requests as <code className="font-mono">{'{{variable}}'}</code> until you clear them or switch workspace; they are never written to disk.
      </p>
    </div>
  );
}
