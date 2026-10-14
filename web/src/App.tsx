import { useEffect, useRef, useState } from 'react';
import { api } from './api';
import { CurlModal, EnvModal, ImportModal, ProtoModal, RepoModal, SaveModal, type SaveTarget } from './components/Modals';
import { Logo } from './components/Logo';
import { RequestEditor } from './components/RequestEditor';
import { ResponseView } from './components/ResponseView';
import { Sidebar } from './components/Sidebar';
import { ghostBtnCls, Icon, IconButton, Method, ResizeHandle, Select } from './components/ui';
import { grpcSource, protoOf, sameProto, withProto } from './lib/proto';
import { emptyRequest, normalizeReq, repoName, uid } from './lib/request';
import { addItem, findItem, mapItem, sortItems } from './lib/tree';
import type { Collection, Environment, HistoryEntry, Item, Req, Res, Workspace } from './types';

type Tab = {
  id: string;
  name: string;
  request: Req;
  source?: { collectionId: string; itemId: string };
  dirty: boolean;
  sending: boolean;
  response?: Res;
};

const blankTab = (): Tab => ({ id: uid(), name: 'Untitled', request: emptyRequest(), dirty: false, sending: false });
const untouched = (t: Tab) => !t.source && !t.dirty && !t.response && !t.sending;
const errorResponse = (error: string): Res => ({ status: 0, statusText: '', headers: [], body: '', base64: false, size: 0, timeMs: 0, error });

const SIDEBAR_DEFAULT = 280;
const SPLIT_DEFAULT = 0.45;
const clamp = (n: number, min: number, max: number) => Math.max(min, Math.min(max, n));

export default function App() {
  const [collections, setCollections] = useState<Collection[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [envId, setEnvId] = useState(() => localStorage.getItem('envId') ?? '');
  const [tabs, setTabs] = useState<Tab[]>(() => [blankTab()]);
  const [activeId, setActiveId] = useState(() => tabs[0].id);
  const [modal, setModal] = useState<null | 'env' | 'import' | 'save' | 'curl' | 'repo'>(null);
  // id of the collection whose proto settings are open
  const [protoFor, setProtoFor] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<Workspace>();
  const [runtime, setRuntime] = useState<Record<string, string>>({});
  const [error, setError] = useState('');
  const [sidebarWidth, setSidebarWidth] = useState(() => Number(localStorage.getItem('sidebarWidth')) || SIDEBAR_DEFAULT);
  const [split, setSplit] = useState(() => Number(localStorage.getItem('split')) || SPLIT_DEFAULT);
  const mainRef = useRef<HTMLDivElement>(null);

  const changeCount = workspace ? workspace.git.changes.length + workspace.git.ahead : 0;
  const active = tabs.find((t) => t.id === activeId) ?? tabs[0];
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
  useEffect(() => {
    document.getElementById(`tab-${activeId}`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, [activeId, tabs.length]);

  const resizeSidebar = (width: number) => setSidebarWidth(clamp(Math.round(width), 200, Math.min(520, window.innerWidth - 480)));
  const resizeSplit = (ratio: number) => setSplit(clamp(ratio, 0.2, 0.8));
  const newTab = () => {
    const tab = blankTab();
    setTabs((ts) => [...ts, tab]);
    setActiveId(tab.id);
  };

  const patchTab = (id: string, patch: Partial<Tab>) => setTabs((ts) => ts.map((t) => (t.id === id ? { ...t, ...patch } : t)));

  // Reuses the current tab when it is still a pristine "Untitled".
  const openTab = (tab: Tab) => {
    setTabs((ts) => (untouched(active) ? ts.map((t) => (t.id === active.id ? tab : t)) : [...ts, tab]));
    setActiveId(tab.id);
  };

  const closeTab = (id: string) => {
    const index = tabs.findIndex((t) => t.id === id);
    const rest = tabs.filter((t) => t.id !== id);
    const next = rest.length ? rest : [blankTab()];
    setTabs(next);
    if (id === activeId) setActiveId(next[Math.min(index, next.length - 1)].id);
  };

  const openItem = (c: Collection, item: Item) => {
    const existing = tabs.find((t) => t.source?.itemId === item.id);
    if (existing) return setActiveId(existing.id);
    openTab({ ...blankTab(), name: item.name, request: normalizeReq(item.request), source: { collectionId: c.id, itemId: item.id } });
  };

  const changeCollection = (changed: Collection) => {
    const c = { ...changed, items: sortItems(changed.items) };
    setCollections((cs) =>
      (cs.some((x) => x.id === c.id) ? cs.map((x) => (x.id === c.id ? c : x)) : [...cs, c]).sort((a, b) =>
        a.name.toLowerCase().localeCompare(b.name.toLowerCase()),
      ),
    );
    // Keep open tabs in step with renames and deletes.
    setTabs((ts) =>
      ts.map((t) => {
        if (t.source?.collectionId !== c.id) return t;
        const item = findItem(c.items, t.source.itemId);
        return item ? { ...t, name: item.name } : { ...t, source: undefined, dirty: true };
      }),
    );
    api.putCollection(c).then(refreshWorkspace, fail);
  };

  const deleteCollection = (id: string) => {
    setCollections((cs) => cs.filter((c) => c.id !== id));
    setTabs((ts) => ts.map((t) => (t.source?.collectionId === id ? { ...t, source: undefined, dirty: true } : t)));
    api.deleteCollection(id).then(refreshWorkspace, fail);
  };

  const send = async (tab: Tab) => {
    if (tab.sending) return;
    patchTab(tab.id, { sending: true });
    try {
      const response = await api.send(tab.request, envId, protoOf(grpcSource(collections, tab.source)));
      patchTab(tab.id, { response, sending: false });
      setHistory(await api.history());
      setRuntime(await api.runtimeVariables());
    } catch (e) {
      patchTab(tab.id, { sending: false, response: errorResponse(e instanceof Error ? e.message : String(e)) });
    }
  };

  const save = (tab: Tab) => {
    const c = collections.find((c) => c.id === tab.source?.collectionId);
    if (!tab.source || !c) return setModal('save');
    changeCollection({ ...c, items: mapItem(c.items, tab.source.itemId, (item) => ({ ...item, request: tab.request })) });
    patchTab(tab.id, { dirty: false });
  };

  const saveAs = (name: string, target: SaveTarget) => {
    const item: Item = { id: uid(), name, request: active.request };
    let c: Collection;
    if ('newCollection' in target) {
      c = { id: uid(), name: target.newCollection, items: [item] };
    } else {
      const base = collections.find((x) => x.id === target.collectionId)!;
      c = { ...base, items: addItem(base.items, target.folderId, item) };
    }
    changeCollection(c);
    patchTab(active.id, { name, source: { collectionId: c.id, itemId: item.id }, dirty: false });
    setModal(null);
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
      if (!(e.metaKey || e.ctrlKey) || modal || protoFor) return;
      if (e.key === 'Enter') {
        e.preventDefault();
        send(active);
      } else if (e.key === 's') {
        e.preventDefault();
        save(active);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  return (
    <div className="h-full flex flex-col">
      <header className="h-11 shrink-0 flex items-center gap-3 pl-3 pr-2 border-b border-line bg-panel">
        <div className="flex items-center gap-2 select-none">
          <Logo />
          <span className="font-semibold tracking-tight">Roundtrip</span>
        </div>
        {workspace && (
          <button
            className={`${ghostBtnCls} min-w-0 shrink text-fg`}
            title={`${workspace.path}\nOpen repository and sync`}
            onClick={() => {
              refreshWorkspace();
              setModal('repo');
            }}
          >
            <span className="text-muted">
              <Icon name="branch" />
            </span>
            {workspace.git.repo ? (
              <>
                <span className="truncate">{workspace.git.remote ? repoName(workspace.git.remote) : 'Local repository'}</span>
                <span className="shrink-0 text-xs tabular-nums text-muted">{changeCount > 0 ? `${changeCount} to sync` : 'Synced'}</span>
              </>
            ) : (
              'Connect GitHub'
            )}
          </button>
        )}
        <div className="ml-auto flex shrink-0 items-center gap-1">
          <button className={ghostBtnCls} onClick={() => setModal('import')} title="Import a cURL command as a new request">
            Import cURL
          </button>
          <Select
            className="ml-1 w-44"
            selectClassName={envId ? '' : 'text-muted'}
            value={envId}
            onChange={(e) => setEnvId(e.target.value)}
            title="Active environment"
            aria-label="Active environment"
          >
            <option value="">No environment</option>
            {environments.map((e) => (
              <option key={e.id} value={e.id}>
                {e.name}
              </option>
            ))}
          </Select>
          <button className={ghostBtnCls} onClick={() => setModal('env')} title="Edit environments and their variables">
            Environments
          </button>
        </div>
      </header>

      <div className="flex-1 flex min-h-0">
        <Sidebar
          width={sidebarWidth}
          collections={collections}
          history={history}
          activeItemId={active.source?.itemId}
          onOpen={openItem}
          onOpenHistory={(h) => openTab({ ...blankTab(), name: 'From history', request: normalizeReq(h.request), dirty: true })}
          onChange={changeCollection}
          onDelete={deleteCollection}
          onProto={(c) => setProtoFor(c.id)}
          onClearHistory={() => {
            setHistory([]);
            api.clearHistory().catch(fail);
          }}
          onReload={reload}
        />
        <ResizeHandle
          axis="x"
          label="Sidebar width"
          onMove={resizeSidebar}
          onNudge={(d) => resizeSidebar(sidebarWidth + d)}
          onReset={() => setSidebarWidth(SIDEBAR_DEFAULT)}
        />

        <main className="flex-1 flex flex-col min-w-0">
          <div role="tablist" aria-label="Open requests" className="flex h-9 shrink-0 items-stretch bg-panel overflow-x-auto [scrollbar-width:none]"
            onWheel={(e) => {
              // A plain mouse wheel scrolls the strip sideways.
              if (!e.deltaX) e.currentTarget.scrollLeft += e.deltaY;
            }}
          >
            {tabs.map((t) => {
              const current = t.id === active.id;
              return (
                <div
                  key={t.id}
                  id={`tab-${t.id}`}
                  role="tab"
                  tabIndex={0}
                  aria-selected={current}
                  title={t.name}
                  onClick={() => setActiveId(t.id)}
                  onAuxClick={(e) => e.button === 1 && closeTab(t.id)}
                  onKeyDown={(e) => e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ') && setActiveId(t.id)}
                  className={`group relative flex min-w-28 max-w-56 shrink-0 scroll-mx-10 items-center gap-1.5 border-r border-line pl-3 pr-1.5 transition-colors duration-100 focus-visible:-outline-offset-2 ${
                    current ? 'bg-bg text-fg' : 'border-b text-muted hover:bg-hover hover:text-fg'
                  }`}
                >
                  <Method request={t.request} />
                  <span className="flex-1 truncate">{t.name}</span>
                  <span className="relative grid size-6 shrink-0 place-items-center">
                    {t.dirty && <span className="size-1.5 rounded-full bg-muted group-hover:opacity-0 group-focus-within:opacity-0" title="Unsaved changes" />}
                    <span
                      className={`absolute inset-0 group-hover:opacity-100 group-focus-within:opacity-100 ${current && !t.dirty ? '' : 'opacity-0'}`}
                    >
                      <IconButton icon="x" title="Close tab" onClick={() => closeTab(t.id)} />
                    </span>
                  </span>
                </div>
              );
            })}
            <div className="sticky right-0 flex items-center border-b border-line bg-panel px-1.5">
              <IconButton icon="plus" title="New request" onClick={newTab} />
            </div>
            <div className="flex-1 border-b border-line" />
          </div>

          <div ref={mainRef} className="flex-1 flex flex-col min-h-0">
            <div className="shrink-0 min-h-0" style={{ height: `${split * 100}%` }}>
              <RequestEditor
                key={active.id}
                request={active.request}
                sending={active.sending}
                dirty={active.dirty}
                onChange={(request) => patchTab(active.id, { request, dirty: true })}
                onSend={() => send(active)}
                onSave={() => save(active)}
                onCurl={() => setModal('curl')}
                grpcSource={activeSource.kind}
                onProtoSettings={() => setProtoFor(active.source?.collectionId ?? null)}
                onLoadMethods={() => api.grpcMethods(active.request, envId, protoOf(activeSource))}
              />
            </div>
            <ResizeHandle
              axis="y"
              label="Request and response split"
              onMove={(y) => {
                const rect = mainRef.current?.getBoundingClientRect();
                if (rect) resizeSplit((y - rect.top) / rect.height);
              }}
              onNudge={(d) => resizeSplit(split + d / (mainRef.current?.clientHeight || 600))}
              onReset={() => setSplit(SPLIT_DEFAULT)}
            />
            <div className="flex-1 min-h-0">
              <ResponseView key={active.id} res={active.response} sending={active.sending} />
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

      {modal === 'repo' && workspace && (
        <RepoModal
          workspace={workspace}
          onClose={() => setModal(null)}
          onChanged={(w, switched) => {
            setWorkspace(w);
            if (switched) {
              // Open tabs belong to the previous workspace.
              const tab = blankTab();
              setTabs([tab]);
              setActiveId(tab.id);
            }
            reload();
          }}
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
            openTab({ ...blankTab(), name: 'Imported cURL', request: normalizeReq(request), dirty: true });
          }}
        />
      )}
      {protoCollection && (
        <ProtoModal
          collection={protoCollection}
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
      {modal === 'save' && (
        <SaveModal
          collections={collections}
          defaultName={active.name === 'Untitled' ? '' : active.name}
          onClose={() => setModal(null)}
          onSave={saveAs}
        />
      )}
    </div>
  );
}
