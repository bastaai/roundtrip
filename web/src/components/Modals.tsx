import { useEffect, useRef, useState } from 'react';
import { api } from '../api';
import { checkSummary, groupMethods, normalizeProto } from '../lib/proto';
import { repoName, uid } from '../lib/request';
import { portablePath, protoFilePath } from '../lib/paths';
import { folderName } from '../lib/shell';
import { listFolders } from '../lib/tree';
import type { Collection, Environment, ProtoCheck, ProtoConfig, Req, Workspace } from '../types';
import { KVEditor, ListEditor } from './KVEditor';
import { PathBrowser } from './PathBrowser';
import { btnCls, fieldCls, ghostBtnCls, Icon, IconButton, inputCls, isMac, Kbd, KEYS, Modal, primaryBtnCls, Select, Spinner, useCopied } from './ui';

const labelCls = 'text-xs font-medium text-muted';
const errorCls = 'rounded-md bg-danger/10 px-3 py-2 text-danger';

// Edits a local copy; everything is saved when the modal closes.
export function EnvModal(p: {
  environments: Environment[];
  runtime: Record<string, string>;
  onClearRuntime: () => void;
  onDone: (environments: Environment[]) => void;
}) {
  const captured = Object.entries(p.runtime);
  const [envs, setEnvs] = useState(p.environments);
  const [selected, setSelected] = useState(p.environments[0]?.id ?? '');
  const current = envs.find((e) => e.id === selected);
  const update = (patch: Partial<Environment>) => setEnvs(envs.map((e) => (e.id === selected ? { ...e, ...patch } : e)));

  const add = () => {
    const env: Environment = { id: uid(), name: 'New environment', variables: [] };
    setEnvs([...envs, env]);
    setSelected(env.id);
  };

  return (
    <Modal
      title="Environments"
      width="max-w-[780px]"
      bodyClass="flex min-h-[360px]"
      onClose={() => p.onDone(envs)}
      footer={
        <>
          <span className="mr-auto text-xs text-muted">Changes are saved when you close this dialog.</span>
          <button className={primaryBtnCls} onClick={() => p.onDone(envs)}>
            Done
          </button>
        </>
      }
    >
      <div className="flex w-48 shrink-0 flex-col gap-0.5 overflow-auto bg-panel p-2">
        {envs.map((e) => (
          <div
            key={e.id}
            role="button"
            tabIndex={0}
            aria-current={e.id === selected || undefined}
            className={`group flex h-7 shrink-0 items-center justify-between rounded-md pl-2 pr-0.5 ${e.id === selected ? 'bg-active font-medium' : 'hover:bg-hover'}`}
            onClick={() => setSelected(e.id)}
            onKeyDown={(ev) => ev.target === ev.currentTarget && (ev.key === 'Enter' || ev.key === ' ') && setSelected(e.id)}
          >
            <span className="truncate">{e.name || 'Untitled'}</span>
            <span className="hidden group-hover:block group-focus-within:block">
              <IconButton icon="trash" title="Delete environment" danger onClick={() => setEnvs(envs.filter((x) => x.id !== e.id))} />
            </span>
          </div>
        ))}
        <button className={`${ghostBtnCls} mt-1 justify-start px-2`} onClick={add}>
          <Icon name="plus" size={13} /> New environment
        </button>
        {captured.length > 0 && (
          <div className="mt-auto flex flex-col gap-1 pt-4">
            <div className="flex items-center justify-between pl-2">
              <span className="whitespace-nowrap text-[11px] font-medium text-muted" title="Saved from responses by the Tests tab. They override environment variables with the same name.">
                Captured this session
              </span>
              <button className={`${ghostBtnCls} h-6 px-1.5 text-xs`} onClick={p.onClearRuntime}>
                Clear
              </button>
            </div>
            {captured.map(([name, value]) => (
              <div key={name} className="truncate pl-2 font-mono text-[11px] leading-5" title={`${name} = ${value}`}>
                {name} <span className="text-muted">= {value}</span>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="min-w-0 flex-1 overflow-auto p-4">
        {current ? (
          <div className="flex flex-col gap-4">
            <label className="flex flex-col gap-1.5">
              <span className={labelCls}>Name</span>
              <input className={inputCls} value={current.name} onChange={(e) => update({ name: e.target.value })} />
            </label>
            <div className="flex flex-col gap-1">
              <span className={labelCls}>Variables</span>
              <KVEditor secrets rows={current.variables} onChange={(variables) => update({ variables })} keyPlaceholder="Variable" />
            </div>
            <p className="text-muted text-xs leading-relaxed">
              Use a variable as <code className="font-mono">{'{{name}}'}</code> in URLs, headers, auth and bodies. Environments are stored on this machine
              only: their values never leave it and are not committed to the repository. The lock only masks a value on screen.
            </p>
          </div>
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
            <p className="max-w-xs text-muted">
              An environment is a set of variables, like <code className="font-mono text-xs">baseUrl</code> or <code className="font-mono text-xs">token</code>,
              that you can switch between.
            </p>
            <button className={btnCls} onClick={add}>
              New environment
            </button>
          </div>
        )}
      </div>
    </Modal>
  );
}

export function ImportModal(p: { onClose: () => void; onCurl: (request: Req) => void }) {
  const [text, setText] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const ready = !busy && !!text.trim();

  const run = async () => {
    if (!ready) return;
    setBusy(true);
    setError('');
    try {
      p.onCurl(await api.parseCurl(text));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    setBusy(false);
  };

  return (
    <Modal
      title="Import cURL"
      onClose={p.onClose}
      footer={
        <>
          <span className="mr-auto flex items-center gap-1.5 text-xs text-muted">
            <Kbd keys={KEYS.send} /> to import
          </span>
          <button className={btnCls} onClick={p.onClose}>
            Cancel
          </button>
          <button className={`${primaryBtnCls} w-[76px]`} disabled={!ready} onClick={run}>
            {busy ? <Spinner size={13} /> : 'Import'}
          </button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <p className="text-muted">Paste a cURL command. It opens as a new request tab that you can edit, send and save.</p>
        <textarea
          autoFocus
          className={`${fieldCls} h-48 py-1.5 font-mono text-xs resize-none`}
          aria-label="cURL command"
          spellCheck={false}
          placeholder={"curl https://api.example.com/users -H 'Accept: application/json'"}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              run();
            }
          }}
        />
        {error && <p className={errorCls}>{error}</p>}
      </div>
    </Modal>
  );
}

export function CurlModal(p: { request: Req; environmentId: string; onClose: () => void }) {
  const [command, setCommand] = useState<string>();
  const [failed, setFailed] = useState('');
  const [copied, copy] = useCopied();
  useEffect(() => {
    api.exportCurl(p.request, p.environmentId).then(setCommand, (e) => setFailed(e instanceof Error ? e.message : String(e)));
  }, [p.request, p.environmentId]);

  return (
    <Modal
      title="Copy as cURL"
      onClose={p.onClose}
      width="max-w-[680px]"
      footer={
        <>
          <span className="mr-auto text-xs text-muted">Variables are resolved with the active environment, so this may contain secrets.</span>
          <button className={`${primaryBtnCls} w-[88px]`} disabled={command === undefined} autoFocus onClick={() => command !== undefined && copy(command)}>
            <Icon name={copied ? 'check' : 'copy'} size={13} />
            {copied ? 'Copied' : 'Copy'}
          </button>
        </>
      }
    >
      {failed ? (
        <p className={errorCls}>{failed}</p>
      ) : (
        <pre className="min-h-16 rounded-md bg-panel p-3 font-mono text-xs leading-[1.6] whitespace-pre-wrap break-all">{command}</pre>
      )}
    </Modal>
  );
}

export type SaveTarget = { collectionId: string; folderId: string | null } | { newCollection: string };

export function SaveModal(p: { collections: Collection[]; defaultName: string; onClose: () => void; onSave: (name: string, target: SaveTarget) => void }) {
  const options = p.collections.flatMap((c) => [
    { value: `${c.id}|`, label: c.name },
    ...listFolders(c.items).map((f) => ({ value: `${c.id}|${f.id}`, label: `${c.name} / ${f.label}` })),
  ]);
  const [name, setName] = useState(p.defaultName);
  const [target, setTarget] = useState(options[0]?.value ?? 'new');
  const [newName, setNewName] = useState('My Collection');

  const valid = name.trim() && (target !== 'new' || newName.trim());
  const save = () => {
    if (!valid) return;
    if (target === 'new') return p.onSave(name.trim(), { newCollection: newName.trim() });
    const [collectionId, folderId] = target.split('|');
    p.onSave(name.trim(), { collectionId, folderId: folderId || null });
  };
  const onEnter = (e: React.KeyboardEvent) => e.key === 'Enter' && save();

  return (
    <Modal
      title="Save request"
      onClose={p.onClose}
      width="max-w-[440px]"
      footer={
        <>
          <button className={btnCls} onClick={p.onClose}>
            Cancel
          </button>
          <button className={primaryBtnCls} disabled={!valid} onClick={save}>
            Save
          </button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <label className="flex flex-col gap-1.5">
          <span className={labelCls}>Request name</span>
          <input autoFocus className={inputCls} placeholder="e.g. List users" value={name} onChange={(e) => setName(e.target.value)} onKeyDown={onEnter} />
        </label>
        <label className="flex flex-col gap-1.5">
          <span className={labelCls}>Save to</span>
          <Select value={target} onChange={(e) => setTarget(e.target.value)}>
            {options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
            <option value="new">+ New collection…</option>
          </Select>
        </label>
        {target === 'new' && (
          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>Collection name</span>
            <input className={inputCls} placeholder="Collection name" value={newName} onChange={(e) => setNewName(e.target.value)} onKeyDown={onEnter} />
          </label>
        )}
      </div>
    </Modal>
  );
}

// Edits a local copy; nothing is saved until Save.
export function ProtoModal(p: {
  collection: Collection;
  // Absolute path of the workspace folder; picked paths inside it are saved relative to it.
  workspace: string;
  environmentId: string;
  environmentName: string;
  onClose: () => void;
  onSave: (proto: ProtoConfig | undefined) => void;
}) {
  const [roots, setRoots] = useState(p.collection.proto?.roots ?? []);
  const [files, setFiles] = useState(p.collection.proto?.files ?? []);
  const [busy, setBusy] = useState(false);
  const [browsing, setBrowsing] = useState<'roots' | 'files'>();
  const [checked, setChecked] = useState<{ key: string; result?: ProtoCheck; error?: string }>();
  // Appends picked paths, skipping ones already listed.
  const add = (rows: string[], picked: string[]) => [...rows, ...picked.filter((path) => !rows.includes(path))];
  // Where the file browser opens: the first root it can resolve, else the workspace.
  const firstRoot = roots.find((root) => !root.includes('{{') && !root.startsWith('~'));
  const filesStart = !firstRoot ? '' : /^([\\/]|[A-Za-z]:)/.test(firstRoot) ? firstRoot : `${p.workspace}/${firstRoot}`;
  const browseBtn = (target: 'roots' | 'files', label: string) => (
    <button className={`${ghostBtnCls} -ml-2 self-start px-2`} onClick={() => setBrowsing(browsing === target ? undefined : target)}>
      <Icon name="folder" size={13} /> {label}
    </button>
  );
  const proto = normalizeProto({ roots, files });
  // A result only describes the values it was run with.
  const key = JSON.stringify(proto) ?? '';
  const shown = checked?.key === key ? checked : undefined;
  const code = (text: string) => <code className="font-mono text-fg">{text}</code>;
  const resultRef = useRef<HTMLDivElement>(null);
  // Block body on purpose: newer browsers return a promise from
  // scrollIntoView, and React would take a returned value for a cleanup function.
  useEffect(() => {
    resultRef.current?.scrollIntoView({ block: 'nearest' });
  }, [checked]);

  const check = async () => {
    if (!proto || busy) return;
    setBusy(true);
    try {
      setChecked({ key, result: await api.checkProto(proto, p.environmentId) });
    } catch (e) {
      setChecked({ key, error: e instanceof Error ? e.message : String(e) });
    }
    setBusy(false);
  };

  return (
    <Modal
      title={`Proto files: ${p.collection.name}`}
      onClose={p.onClose}
      width="max-w-[620px]"
      footer={
        <>
          <span className="mr-auto flex min-w-0 items-center gap-2.5">
            <button className={`${btnCls} w-[68px]`} disabled={!proto || busy} onClick={check} title="Load these files now, without saving">
              {busy ? <Spinner size={13} /> : 'Check'}
            </button>
            <span className="truncate text-xs text-muted">{p.environmentName ? `Variables from ${p.environmentName}` : 'No environment selected'}</span>
          </span>
          <button className={btnCls} onClick={p.onClose}>
            Cancel
          </button>
          <button className={primaryBtnCls} onClick={() => p.onSave(proto)}>
            Save
          </button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-muted">
          Use local .proto files for the gRPC requests in this collection instead of server reflection. The paths are saved with the collection, so your
          team gets them through git.
        </p>
        <div className="flex flex-col gap-1.5">
          <ListEditor autoFocus label="Import roots" placeholder="Add a folder, e.g. protos" rows={roots} onChange={setRoots} />
          {browsing === 'roots' ? (
            <PathBrowser
              mode="folder"
              onCancel={() => setBrowsing(undefined)}
              onPick={(picked) => {
                setRoots(add(roots, picked.map((path) => portablePath(path, p.workspace))));
                setBrowsing(undefined);
              }}
            />
          ) : (
            browseBtn('roots', 'Browse for a folder…')
          )}
          <p className="text-xs leading-relaxed text-muted">
            Folders that imports are resolved from, like {code('-I')} for protoc. Relative paths are relative to the workspace folder; absolute paths and{' '}
            {code('~')} work too.
          </p>
        </div>
        <div className="flex flex-col gap-1.5">
          <ListEditor label="Files" placeholder="Add a file, e.g. api/v1/users.proto" rows={files} onChange={setFiles} />
          {browsing === 'files' ? (
            <PathBrowser
              mode="files"
              ext=".proto"
              start={filesStart}
              onCancel={() => setBrowsing(undefined)}
              onPick={(picked) => {
                setFiles(add(files, picked.map((path) => protoFilePath(path, roots, p.workspace))));
                setBrowsing(undefined);
              }}
            />
          ) : (
            browseBtn('files', 'Browse for .proto files…')
          )}
          <p className="text-xs leading-relaxed text-muted">
            Proto files to load, relative to a root. Leave this empty to load every .proto file under the roots.
          </p>
        </div>
        <p className="text-xs leading-relaxed text-muted">
          If a path differs per person, use a variable such as {code('{{protoRoot}}/third_party')} and let everyone set {code('protoRoot')} in their own
          environment. Environments stay on each machine.
        </p>
        {!proto && <p className="text-xs text-muted">Nothing is set, so gRPC requests in this collection use server reflection.</p>}
        {shown?.error !== undefined && (
          <div ref={resultRef}>
            <pre role="alert" className={`${errorCls} whitespace-pre-wrap break-words font-mono text-xs`}>
              {shown.error}
            </pre>
          </div>
        )}
        {shown?.result && (
          <div ref={resultRef} className="flex flex-col gap-1.5" role="status">
            <p className="flex items-center gap-1.5 tabular-nums">
              <span className="text-ok">
                <Icon name="check" size={13} strokeWidth={2.5} />
              </span>
              {checkSummary(shown.result)}
            </p>
            {groupMethods(shown.result).length > 0 && (
              <div className="rounded-md bg-panel px-2.5 py-1.5 font-mono text-xs leading-5">
                {groupMethods(shown.result).map((g) => (
                  <div key={g.service}>
                    <div className="break-all">{g.service}</div>
                    {g.methods.map((m) => (
                      <div key={m} className="break-all pl-4 text-muted">
                        {m}
                      </div>
                    ))}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}

const CHANGE_STYLE = { added: ['A', 'text-ok'], modified: ['M', 'text-warn'], deleted: ['D', 'text-danger'] } as const;

// switched is true when the workspace now points at a different folder.
export function WorkspaceModal(p: {
  workspace: Workspace;
  hasCollections: boolean;
  onClose: () => void;
  onChanged: (workspace: Workspace, switched: boolean) => void;
}) {
  const { git } = p.workspace;
  const [path, setPath] = useState('');
  const [carry, setCarry] = useState(false);
  const [browsing, setBrowsing] = useState(false);
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');
  const [copied, copy] = useCopied();

  const run = async (label: string, action: () => Promise<Workspace>, switched = false) => {
    setBusy(label);
    setError('');
    try {
      p.onChanged(await action(), switched);
      setMessage('');
      setPath('');
      setCarry(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    setBusy('');
  };

  const canSync = !busy && !(git.changes.length === 0 && (!git.remote || (git.upstream && git.ahead === 0)));
  const sync = () => canSync && run('sync', () => api.gitSync(message));
  const canOpen = !busy && !!path.trim();
  const open = () => canOpen && run('open', () => api.openWorkspace(path.trim(), carry), true);

  return (
    <Modal title="Workspace" onClose={p.onClose} width="max-w-[640px]">
      <div className="flex flex-col gap-5">
        <div className="flex flex-col gap-1.5">
          <div className={labelCls}>Folder</div>
          <div className="flex items-start gap-1 rounded-md bg-panel py-1 pl-2.5 pr-1">
            <span className="min-w-0 flex-1 py-1 font-mono text-xs break-all">{p.workspace.path}</span>
            <IconButton icon={copied ? 'check' : 'copy'} title={copied ? 'Copied' : 'Copy path'} onClick={() => copy(p.workspace.path)} />
          </div>
          <p className="text-xs leading-relaxed text-muted">
            Every request is saved as a readable YAML file under <code className="font-mono">collections/</code> in this folder, ready to be
            committed with the rest of your code.
          </p>
        </div>

        {git.repo ? (
          <div className="flex flex-col gap-2">
            <div className="flex min-h-7 items-center justify-between gap-3">
              <div className="flex min-w-0 items-baseline gap-2">
                <span className="truncate font-medium">{git.remote ? repoName(git.remote) : 'Git repository'}</span>
                <span className="shrink-0 font-mono text-xs text-muted">{git.branch || 'no branch'}</span>
                {git.ahead > 0 && <span className="shrink-0 text-xs tabular-nums text-muted">{git.ahead} to push</span>}
                {git.behind > 0 && <span className="shrink-0 text-xs tabular-nums text-muted">{git.behind} to pull</span>}
              </div>
              {git.remote && (
                <button className={`${btnCls} w-[72px]`} disabled={!!busy} onClick={() => run('pull', api.gitPull)}>
                  {busy === 'pull' ? <Spinner size={13} /> : 'Pull'}
                </button>
              )}
            </div>

            {git.changes.length === 0 ? (
              <p className="text-muted">No uncommitted changes.</p>
            ) : (
              <>
                <p className="text-xs tabular-nums text-muted">
                  {git.changes.length} uncommitted {git.changes.length === 1 ? 'change' : 'changes'}
                </p>
                <div className="max-h-40 overflow-auto rounded-md bg-panel px-2.5 py-1.5 font-mono text-xs leading-5">
                  {git.changes.map((c) => (
                    <div key={c.path} className="flex gap-2" title={c.status}>
                      <span className={`${CHANGE_STYLE[c.status][1]} w-3 shrink-0 font-medium`}>{CHANGE_STYLE[c.status][0]}</span>
                      <span className="break-all">{c.path}</span>
                    </div>
                  ))}
                </div>
              </>
            )}

            <div className="flex gap-2">
              <input
                className={`${inputCls} flex-1`}
                aria-label="Commit message"
                placeholder="Commit message (default: Update requests)"
                value={message}
                disabled={git.changes.length === 0}
                onChange={(e) => setMessage(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && sync()}
              />
              <button className={`${primaryBtnCls} min-w-[112px]`} disabled={!canSync} onClick={sync}>
                {busy === 'sync' ? <Spinner size={13} /> : git.remote ? (git.changes.length ? 'Commit & push' : 'Push') : 'Commit'}
              </button>
            </div>
            <p className="text-xs leading-relaxed text-muted">
              Optional: you can commit with git yourself at any time. Only files under <code className="font-mono">collections/</code> are committed
              from here.
            </p>
          </div>
        ) : (
          <p className="text-muted">
            {p.workspace.gitAvailable
              ? 'This folder is not in a git repository. To keep your requests in source control, open a folder inside a repository below.'
              : 'git was not found on this machine, so changes cannot be shown or committed from here. Your requests are still saved as files.'}
          </p>
        )}

        {p.workspace.recent.length > 0 && (
          <div className="flex flex-col gap-1.5">
            <h3 className="font-medium">Switch workspace</h3>
            <div className="-mx-2 flex max-h-40 flex-col overflow-auto">
              {p.workspace.recent.map((recent) => (
                <button
                  key={recent}
                  className="flex h-7 shrink-0 items-center gap-3 rounded-md px-2 text-left hover:bg-hover disabled:opacity-50"
                  disabled={!!busy}
                  title={recent}
                  onClick={() => run('open', () => api.openWorkspace(recent, false), true)}
                >
                  <span className="shrink-0">{folderName(recent)}</span>
                  <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted" dir="rtl">
                    {recent}
                  </span>
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="flex flex-col gap-2">
          <h3 className="font-medium">Open another folder</h3>
          <div className="flex gap-2">
            <button className={btnCls} disabled={!!busy} onClick={() => setBrowsing(!browsing)}>
              Browse…
            </button>
            <input
              className={`${inputCls} flex-1 font-mono text-xs placeholder:font-sans placeholder:text-[13px]`}
              aria-label="Folder path"
              spellCheck={false}
              placeholder={`Full path, e.g. ${isMac ? '/Users/you/code/my-service/api-requests' : 'C:\\Users\\you\\code\\my-service\\api-requests'}`}
              value={path}
              onChange={(e) => setPath(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && open()}
            />
            <button className={`${btnCls} w-[84px]`} disabled={!canOpen} onClick={open}>
              {busy === 'open' ? <Spinner size={13} /> : 'Open'}
            </button>
          </div>
          {browsing && (
            <PathBrowser
              mode="folder"
              start={path.trim()}
              onCancel={() => setBrowsing(false)}
              onPick={([picked]) => {
                setPath(picked);
                setBrowsing(false);
              }}
            />
          )}
          {p.hasCollections && (
            <label className="flex items-center gap-2 text-muted">
              <input type="checkbox" checked={carry} onChange={(e) => setCarry(e.target.checked)} />
              Copy my current collections into it, if it has none yet
            </label>
          )}
          <p className="text-xs leading-relaxed text-muted">
            Use a folder inside a git repository, for example next to the code of the service the requests belong to. The folder is created if it
            does not exist.
          </p>
        </div>

        {error && <pre className={`${errorCls} max-h-40 overflow-auto whitespace-pre-wrap break-words font-mono text-xs`}>{error}</pre>}
      </div>
    </Modal>
  );
}
