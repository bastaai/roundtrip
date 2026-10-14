import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { api } from './api';
import { CurlModal, EnvModal, ImportModal, ProtoModal, WorkspaceModal } from './components/Modals';
import { Logo } from './components/Logo';
import { QuickOpen, type QuickAction } from './components/QuickOpen';
import { AddressBar, RequestEditor } from './components/RequestEditor';
import { ResponseView } from './components/ResponseView';
import { Sidebar } from './components/Sidebar';
import { ghostBtnCls, Icon, IconButton, isMod, keyLabel, KEYS, ResizeHandle, Select } from './components/ui';
import { grpcSource, protoOf, sameProto, withProto } from './lib/proto';
import { emptyRequest, kindOf, normalizeReq, uid } from './lib/request';
import { folderName } from './lib/shell';
import { findItem, folderPath, freeName, mapItem, sortItems, subtreeIds } from './lib/tree';
import type { Collection, Environment, HistoryEntry, Item, Req, Res, Workspace } from './types';

// A request opened in this session. One is shown at a time; the others keep
// their last response so going back to them shows it again.
type Page = {
  id: string;
  name: string;
  request: Req;
  // Where the request is saved. A page without one becomes a request in the
  // current collection on its first edit.
  source?: { collectionId: string; itemId: string };
  sending: boolean;
  response?: Res;
  // The history entry the response came from, when it is known.
  run?: string;
};

const blankPage = (): Page => ({ id: uid(), name: '', request: emptyRequest(), sending: false });
const untouched = (p: Page) => !p.source && !p.response && !p.sending;
// Edits are written once typing pauses for this long.
const TYPING_PAUSE = 400;
const MANAGE_ENVIRONMENTS = '\u0000manage';
const errorResponse = (error: string): Res => ({ status: 0, statusText: '', headers: [], body: '', base64: false, size: 0, timeMs: 0, error });

const SIDEBAR_DEFAULT = 280;
// Share of the pair given to the request: stacked (height) and side by side (width) are remembered separately.
const SPLIT_DEFAULT = 0.45;
const COLUMNS_DEFAULT = 0.44;
// Below this the pair cannot hold two usable columns (a key/value grid next to a readable body), so it stacks.
const COLUMNS_MIN_WIDTH = 880;
const COLUMN_MIN = 340;
const clamp = (n: number, min: number, max: number) => Math.max(min, Math.min(max, n));

export default function App() {
  const [collections, setCollections] = useState<Collection[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [envId, setEnvId] = useState(() => localStorage.getItem('envId') ?? '');
  const [pages, setPages] = useState<Page[]>(() => [blankPage()]);
  const [activeId, setActiveId] = useState(() => pages[0].id);
  const [modal, setModal] = useState<null | 'env' | 'import' | 'curl' | 'repo' | 'quick'>(null);
  // id of the collection whose proto settings are open
  const [protoFor, setProtoFor] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<Workspace>();
  const [runtime, setRuntime] = useState<Record<string, string>>({});
  const [error, setError] = useState('');
  const [sidebarWidth, setSidebarWidth] = useState(() => Number(localStorage.getItem('sidebarWidth')) || SIDEBAR_DEFAULT);
  const [split, setSplit] = useState(() => Number(localStorage.getItem('split')) || SPLIT_DEFAULT);
  const [columnSplit, setColumnSplit] = useState(() => Number(localStorage.getItem('columnSplit')) || COLUMNS_DEFAULT);
  const [stacked, setStacked] = useState(() => localStorage.getItem('layout') === 'stacked');
  const [sidebarHidden, setSidebarHidden] = useState(() => localStorage.getItem('sidebarHidden') === '1');
  const [pairWidth, setPairWidth] = useState(0);
  const pairRef = useRef<HTMLDivElement>(null);
  // Collection writes waiting for typing to pause, at most one per collection.
  const writes = useRef(new Map<string, { collection: Collection; timer: ReturnType<typeof setTimeout> }>());
  // The collection a new request goes into: the one last worked in.
  const lastCollection = useRef('');
  const roomForColumns = pairWidth >= COLUMNS_MIN_WIDTH;
  const columns = !stacked && roomForColumns;

  const changeCount = workspace ? workspace.git.changes.length + workspace.git.ahead : 0;
  const active = pages.find((p) => p.id === activeId) ?? pages[0];
  const activeSource = grpcSource(collections, active.source);
  const protoCollection = collections.find((c) => c.id === protoFor);
  const fail = (e: unknown) => setError(e instanceof Error ? e.message : String(e));

  const refreshWorkspace = () => api.workspace().then(setWorkspace, fail);

  const reload = () =>
    Promise.all([api.collections(), api.environments(), api.history(), api.workspace(), api.runtimeVariables()])
      .then(([c, e, h, w, r]) => {
        setRuntime(r);
        setWorkspace(w);
        setCollections(c.map((col) => ({ ...col, items: col.items ?? [] })));
        setEnvironments(e);
        // Environments are per machine, so a remembered selection may no longer exist.
        setEnvId((id) => (e.some((x) => x.id === id) ? id : ''));
        setHistory(h);
        setError('');
      })
      .catch(fail);

  useEffect(() => {
    reload();
  }, []);
  useEffect(() => localStorage.setItem('envId', envId), [envId]);
  useEffect(() => localStorage.setItem('sidebarWidth', String(sidebarWidth)), [sidebarWidth]);
  useEffect(() => localStorage.setItem('split', String(split)), [split]);
  useEffect(() => localStorage.setItem('columnSplit', String(columnSplit)), [columnSplit]);
  useEffect(() => localStorage.setItem('layout', stacked ? 'stacked' : 'columns'), [stacked]);
  useEffect(() => localStorage.setItem('sidebarHidden', sidebarHidden ? '1' : ''), [sidebarHidden]);
  // Measured before the first paint, so the pair never flashes in the wrong arrangement.
  useLayoutEffect(() => {
    const el = pairRef.current!;
    setPairWidth(el.clientWidth);
    const observer = new ResizeObserver(() => setPairWidth(el.clientWidth));
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (active.source) lastCollection.current = active.source.collectionId;
  }, [active.source]);
  useEffect(() => {
    window.addEventListener('pagehide', flushWrites);
    return () => window.removeEventListener('pagehide', flushWrites);
  }, []);

  const resizeSidebar = (width: number) => setSidebarWidth(clamp(Math.round(width), 200, Math.min(520, window.innerWidth - 480)));
  const resizeSplit = (ratio: number) => setSplit(clamp(ratio, 0.2, 0.8));
  // Neither column gets narrower than COLUMN_MIN, also after the window shrinks under a remembered split.
  const fitColumns = (ratio: number) => {
    const min = Math.min(0.5, Math.max(0.25, COLUMN_MIN / (pairWidth || 1)));
    return clamp(ratio, min, 1 - min);
  };
  const resizeColumns = (ratio: number) => setColumnSplit(fitColumns(ratio));
  const patchPage = (id: string, patch: Partial<Page>) => setPages((ps) => ps.map((p) => (p.id === id ? { ...p, ...patch } : p)));

  // Replaces the current page when nothing has happened on it yet.
  const openPage = (page: Page) => {
    setPages((ps) => (untouched(active) ? ps.map((p) => (p.id === active.id ? page : p)) : [...ps, page]));
    setActiveId(page.id);
  };

  const newRequest = () => openPage(blankPage());

  // Shows the response kept with a history entry.
  const showRun = (pageId: string, run: string) =>
    api.historyResponse(run).then((response) => patchPage(pageId, { response, run }), fail);

  const openItem = (c: Collection, item: Item) => {
    const existing = pages.find((p) => p.source?.itemId === item.id);
    if (existing) return setActiveId(existing.id);
    const page = { ...blankPage(), name: item.name, request: normalizeReq(item.request), source: { collectionId: c.id, itemId: item.id } };
    openPage(page);
    // A request opens with the last response it got, from this session or an earlier one.
    const last = history.find((h) => h.itemId === item.id);
    if (last) api.historyResponse(last.id).then((response) => patchPage(page.id, { response, run: last.id }), () => {});
  };

  // Pages whose request was deleted go away; there is always one left to show.
  const prunePages = (keep: (p: Page) => boolean) =>
    setPages((ps) => {
      const rest = ps.filter(keep);
      const next = rest.length ? rest : [blankPage()];
      setActiveId((id) => (next.some((p) => p.id === id) ? id : next[next.length - 1].id));
      return next;
    });

  const putCollection = (c: Collection) => api.putCollection(c).then(refreshWorkspace, fail);

  // Writes are queued per collection so that typing is saved once it pauses, in order.
  const write = (c: Collection, delay: number) => {
    clearTimeout(writes.current.get(c.id)?.timer);
    if (!delay) {
      writes.current.delete(c.id);
      putCollection(c);
      return;
    }
    const timer = setTimeout(() => {
      writes.current.delete(c.id);
      putCollection(c);
    }, delay);
    writes.current.set(c.id, { collection: c, timer });
  };

  // Before anything that must see every edit on disk: switching workspace, leaving the page.
  function flushWrites() {
    writes.current.forEach(({ collection, timer }) => {
      clearTimeout(timer);
      api.putCollection(collection).catch(() => {});
    });
    writes.current.clear();
  }

  const changeCollection = (changed: Collection, delay = 0) => {
    const c = { ...changed, items: sortItems(changed.items) };
    setCollections((cs) =>
      (cs.some((x) => x.id === c.id) ? cs.map((x) => (x.id === c.id ? c : x)) : [...cs, c]).sort((a, b) =>
        a.name.toLowerCase().localeCompare(b.name.toLowerCase()),
      ),
    );
    // Keep open pages in step with renames and deletes.
    prunePages((p) => p.source?.collectionId !== c.id || !!findItem(c.items, p.source.itemId));
    setPages((ps) =>
      ps.map((p) => {
        const item = p.source?.collectionId === c.id ? findItem(c.items, p.source.itemId) : undefined;
        return item && item.name !== p.name ? { ...p, name: item.name } : p;
      }),
    );
    write(c, delay);
  };

  // changed is from moveItem: the source collection first, then the target if it is another one.
  const moveItems = (changed: Collection[], itemId: string) => {
    const to = changed[changed.length - 1];
    const moved = findItem(to.items, itemId);
    const ids = new Set(moved ? subtreeIds(moved) : []);
    // Open pages follow their requests before the source collection is written without them.
    setPages((ps) => ps.map((p) => (p.source && ids.has(p.source.itemId) ? { ...p, source: { ...p.source, collectionId: to.id } } : p)));
    [...changed].reverse().forEach((c) => changeCollection(c));
  };

  const deleteCollection = (id: string) => {
    clearTimeout(writes.current.get(id)?.timer);
    writes.current.delete(id);
    setCollections((cs) => cs.filter((c) => c.id !== id));
    prunePages((p) => p.source?.collectionId !== id);
    api.deleteCollection(id).then(refreshWorkspace, fail);
  };

  // Saves request as a new item of the collection last worked in (or a new
  // "Requests" collection) and returns where it went.
  const addRequest = (name: string, request: Req): Pick<Page, 'name' | 'source'> => {
    const target = collections.find((c) => c.id === lastCollection.current) ?? collections[0];
    const items = target?.items ?? [];
    const item: Item = { id: uid(), name: name.trim() || freeName(items, 'New request'), request };
    const c = target ? { ...target, items: [...items, item] } : { id: uid(), name: 'Requests', items: [item] };
    lastCollection.current = c.id;
    changeCollection(c);
    return { name: item.name, source: { collectionId: c.id, itemId: item.id } };
  };

  const editRequest = (page: Page, request: Req) => {
    const c = collections.find((x) => x.id === page.source?.collectionId);
    if (!page.source || !c) return patchPage(page.id, { request, ...addRequest(page.name, request) });
    patchPage(page.id, { request });
    changeCollection({ ...c, items: mapItem(c.items, page.source.itemId, (item) => ({ ...item, request })) }, TYPING_PAUSE);
  };

  const renamePage = (page: Page, name: string) => {
    const c = collections.find((x) => x.id === page.source?.collectionId);
    if (!page.source || !c) return patchPage(page.id, { name });
    if (name) changeCollection({ ...c, items: mapItem(c.items, page.source.itemId, (item) => ({ ...item, name })) });
  };

  const send = async (page: Page) => {
    if (page.sending) return;
    patchPage(page.id, { sending: true });
    try {
      const response = await api.send(page.request, envId, protoOf(grpcSource(collections, page.source)), page.source?.itemId);
      const h = await api.history();
      patchPage(page.id, { response, sending: false, run: h[0]?.id });
      setHistory(h);
      setRuntime(await api.runtimeVariables());
    } catch (e) {
      patchPage(page.id, { sending: false, response: errorResponse(e instanceof Error ? e.message : String(e)) });
    }
  };

  const saveEnvironments = (next: Environment[]) => {
    const removed = environments.filter((e) => !next.some((n) => n.id === e.id));
    setEnvironments(next);
    if (!next.some((e) => e.id === envId)) setEnvId('');
    Promise.all([...next.map(api.putEnvironment), ...removed.map((e) => api.deleteEnvironment(e.id))]).then(refreshWorkspace, fail);
    setModal(null);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (isMod(e) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'k' && !protoFor && (!modal || modal === 'quick')) {
        e.preventDefault();
        setModal(modal ? null : 'quick');
        return;
      }
      if (!(e.metaKey || e.ctrlKey) || modal || protoFor) return;
      if (isMod(e) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'b') {
        e.preventDefault();
        setSidebarHidden((hidden) => !hidden);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        send(active);
      } else if (e.key === 's') {
        // Everything is saved as it is typed; this only writes it out now.
        e.preventDefault();
        flushWrites();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  const openWorkspace = () => {
    flushWrites();
    refreshWorkspace();
    setModal('repo');
  };

  const describeMethod = (method: string) => api.grpcDescribe({ ...active.request, method }, envId, protoOf(activeSource));

  // switched is true when the workspace now points at a different folder.
  const workspaceChanged = (w: Workspace, switched: boolean) => {
    setWorkspace(w);
    if (switched) {
      // Open pages belong to the previous workspace.
      const page = blankPage();
      lastCollection.current = '';
      setPages([page]);
      setActiveId(page.id);
    }
    reload();
  };

  // "Collection / Folder" for a saved request.
  const whereIs = (page: Page) => {
    const c = collections.find((x) => x.id === page.source?.collectionId);
    const folders = c && page.source ? folderPath(c.items, page.source.itemId) : undefined;
    return c && folders ? [c.name, ...folders].join(' / ') : '';
  };

  const quickActions: QuickAction[] = [
    { id: 'new', label: 'New request', run: newRequest },
    { id: 'import', label: 'Import cURL', detail: 'Paste a command as a new request', run: () => setModal('import') },
    ...(kindOf(active.request) !== 'grpc' ? [{ id: 'curl', label: 'Copy as cURL', detail: 'This request', run: () => setModal('curl') }] : []),
    { id: 'workspace', label: 'Open workspace', detail: 'Folder, git status, switch workspace', run: openWorkspace },
    ...(workspace?.recent ?? []).map((path) => ({
      id: `workspace-${path}`,
      label: `Switch to workspace ${folderName(path)}`,
      detail: path,
      run: () => {
        flushWrites();
        api.openWorkspace(path, false).then((w) => workspaceChanged(w, true), fail);
      },
    })),
    ...[{ id: '', name: 'No environment' }, ...environments]
      .filter((e) => e.id !== envId)
      .map((e) => ({ id: `env-${e.id}`, label: e.id ? `Switch to ${e.name}` : 'Switch to no environment', detail: 'Environment', run: () => setEnvId(e.id) })),
    { id: 'envs', label: 'Manage environments', detail: 'Variables', run: () => setModal('env') },
    { id: 'sidebar', label: sidebarHidden ? 'Show sidebar' : 'Hide sidebar', keys: KEYS.sidebar, run: () => setSidebarHidden(!sidebarHidden) },
    ...(roomForColumns
      ? [{ id: 'layout', label: stacked ? 'Request and response side by side' : 'Response under the request', detail: 'Layout', run: () => setStacked(!stacked) }]
      : []),
  ];

  return (
    <div className="h-full flex flex-col">
      <header className="h-11 shrink-0 flex items-center gap-2 pl-2 pr-2 border-b border-line bg-panel">
        <IconButton
          icon="sidebar"
          title={`${sidebarHidden ? 'Show' : 'Hide'} sidebar (${keyLabel(KEYS.sidebar)})`}
          active={!sidebarHidden}
          onClick={() => setSidebarHidden(!sidebarHidden)}
        />
        <span className="mx-1 select-none text-fg">
          <Logo />
        </span>
        {workspace && (
          <button
            className={`${ghostBtnCls} min-w-0 shrink text-fg`}
            title={`${workspace.path}\nWorkspace folder and git status`}
            onClick={openWorkspace}
          >
            <span className="truncate">{folderName(workspace.path)}</span>
            {workspace.git.repo && (
              <span className="shrink-0 text-xs tabular-nums text-muted">
                {changeCount > 0 ? `${changeCount} ${changeCount === 1 ? 'change' : 'changes'}` : workspace.git.branch}
              </span>
            )}
          </button>
        )}
        <div className="ml-auto flex shrink-0 items-center gap-0.5">
          <IconButton icon="filePlus" title="New request" onClick={newRequest} />
          <IconButton icon="search" title={`Go to a request or run a command (${keyLabel(KEYS.quickOpen)})`} onClick={() => setModal('quick')} />
          <Select
            bare
            className="ml-1"
            selectClassName={envId ? '' : 'text-muted'}
            value={envId}
            onChange={(e) => (e.target.value === MANAGE_ENVIRONMENTS ? setModal('env') : setEnvId(e.target.value))}
            title="Environment"
            aria-label="Environment"
          >
            <option value="">No environment</option>
            {environments.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
            <option disabled>──────────</option>
            <option value={MANAGE_ENVIRONMENTS}>Manage environments…</option>
          </Select>
        </div>
      </header>

      <div className="flex-1 flex min-h-0">
        <Sidebar
          width={sidebarWidth}
          hidden={sidebarHidden}
          collections={collections}
          history={history}
          activeItemId={active.source?.itemId}
          onOpen={openItem}
          onOpenHistory={(h) => {
            const page = { ...blankPage(), request: normalizeReq(h.request) };
            openPage(page);
            showRun(page.id, h.id);
          }}
          onChange={changeCollection}
          onMove={moveItems}
          onDelete={deleteCollection}
          onProto={(c) => setProtoFor(c.id)}
          onClearHistory={() => {
            setHistory([]);
            api.clearHistory().catch(fail);
          }}
          onReload={reload}
        />
        {!sidebarHidden && (
          <ResizeHandle
            axis="x"
            label="Sidebar width"
            onMove={resizeSidebar}
            onNudge={(d) => resizeSidebar(sidebarWidth + d)}
            onReset={() => setSidebarWidth(SIDEBAR_DEFAULT)}
          />
        )}

        <main className="flex-1 flex flex-col min-w-0">
          <PageTitle
            key={active.id}
            name={active.name}
            where={whereIs(active)}
            onRename={(name) => renamePage(active, name)}
          />
          <AddressBar
            key={`bar-${active.id}`}
            request={active.request}
            sending={active.sending}
            onChange={(request) => editRequest(active, request)}
            onSend={() => send(active)}
            grpcSource={activeSource.kind}
            onProtoSettings={() => setProtoFor(active.source?.collectionId ?? null)}
            onLoadMethods={() => api.grpcMethods(active.request, envId, protoOf(activeSource))}
            onDescribe={describeMethod}
          />

          <div ref={pairRef} className={`flex min-h-0 flex-1 ${columns ? '' : 'flex-col'}`}>
            <div className="min-h-0 min-w-0 shrink-0" style={columns ? { width: `${fitColumns(columnSplit) * 100}%` } : { height: `${split * 100}%` }}>
              <RequestEditor
                key={active.id}
                request={active.request}
                onChange={(request) => editRequest(active, request)}
                grpcSource={activeSource.kind}
                onDescribe={describeMethod}
              />
            </div>
            {columns ? (
              <ResizeHandle
                key="columns"
                axis="x"
                label="Request and response split"
                onMove={(x) => {
                  const rect = pairRef.current?.getBoundingClientRect();
                  if (rect) resizeColumns((x - rect.left) / rect.width);
                }}
                onNudge={(d) => resizeColumns(fitColumns(columnSplit) + d / (pairWidth || 1000))}
                onReset={() => setColumnSplit(COLUMNS_DEFAULT)}
              />
            ) : (
              <ResizeHandle
                key="stacked"
                axis="y"
                label="Request and response split"
                onMove={(y) => {
                  const rect = pairRef.current?.getBoundingClientRect();
                  if (rect) resizeSplit((y - rect.top) / rect.height);
                }}
                onNudge={(d) => resizeSplit(split + d / (pairRef.current?.clientHeight || 600))}
                onReset={() => setSplit(SPLIT_DEFAULT)}
              />
            )}
            <div className="min-h-0 min-w-0 flex-1">
              <ResponseView
                key={active.id}
                res={active.response}
                sending={active.sending}
                runs={active.source ? history.filter((h) => h.itemId === active.source!.itemId) : []}
                run={active.run}
                onRun={(run) => showRun(active.id, run)}
              />
            </div>
          </div>
        </main>
      </div>

      {error && (
        <div
          role="alert"
          className="fixed bottom-4 left-1/2 z-40 flex max-w-[min(80vw,640px)] -translate-x-1/2 items-start gap-2.5 rounded-lg bg-raised py-2 pl-3 pr-1.5 shadow-pop animate-toast"
        >
          <span className="mt-[3px] text-danger">
            <Icon name="alert" />
          </span>
          <span className="py-0.5 break-words min-w-0">{error}</span>
          <IconButton icon="x" title="Dismiss" onClick={() => setError('')} />
        </div>
      )}

      {modal === 'quick' && <QuickOpen collections={collections} actions={quickActions} onOpen={openItem} onClose={() => setModal(null)} />}
      {modal === 'repo' && workspace && (
        <WorkspaceModal
          workspace={workspace}
          hasCollections={collections.length > 0}
          onClose={() => setModal(null)}
          onChanged={workspaceChanged}
        />
      )}
      {modal === 'env' && (
        <EnvModal
          environments={environments}
          runtime={runtime}
          onClearRuntime={() => {
            setRuntime({});
            api.clearRuntimeVariables().catch(fail);
          }}
          onDone={saveEnvironments}
        />
      )}
      {modal === 'import' && (
        <ImportModal
          onClose={() => setModal(null)}
          onCurl={(request) => {
            setModal(null);
            const imported = normalizeReq(request);
            openPage({ ...blankPage(), request: imported, ...addRequest('', imported) });
          }}
        />
      )}
      {protoCollection && (
        <ProtoModal
          collection={protoCollection}
          workspace={workspace?.path ?? ''}
          environmentId={envId}
          environmentName={environments.find((e) => e.id === envId)?.name ?? ''}
          onClose={() => setProtoFor(null)}
          onSave={(proto) => {
            if (!sameProto(protoCollection.proto, proto)) changeCollection(withProto(protoCollection, proto));
            setProtoFor(null);
          }}
        />
      )}
      {modal === 'curl' && <CurlModal request={active.request} environmentId={envId} onClose={() => setModal(null)} />}
    </div>
  );
}

// The request's name, at the head of the page like a document title. Renamed in place.
function PageTitle({ name, where, onRename }: { name: string; where: string; onRename: (name: string) => void }) {
  const [value, setValue] = useState(name);
  useEffect(() => {
    setValue(name);
  }, [name]);
  const commit = () => {
    const next = value.trim();
    if (next && next !== name) onRename(next);
    else setValue(name);
  };
  return (
    <div className="flex shrink-0 items-baseline gap-3 px-4 pt-4">
      <input
        className="min-w-0 flex-1 bg-transparent text-lg font-semibold tracking-tight outline-none placeholder:text-faint"
        value={value}
        placeholder="Untitled"
        aria-label="Request name"
        spellCheck={false}
        onChange={(e) => setValue(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.currentTarget.blur();
          if (e.key === 'Escape') {
            setValue(name);
            requestAnimationFrame(() => (e.target as HTMLInputElement).blur());
          }
        }}
      />
      {where && <span className="max-w-[45%] shrink-0 truncate text-xs text-muted">{where}</span>}
    </div>
  );
}
