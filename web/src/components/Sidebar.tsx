import { useRef, useState } from 'react';
import { emptyRequest, uid } from '../lib/request';
import { addItem, filterCollections, isFolder, mapItem, moveItem, removeItem } from '../lib/tree';
import type { Collection, HistoryEntry, Item } from '../types';
import { btnCls, Confirm, ghostBtnCls, Icon, IconButton, Method, statusColor, Tabs } from './ui';

type Props = {
  width: number;
  hidden: boolean;
  collections: Collection[];
  history: HistoryEntry[];
  activeItemId?: string;
  onOpen: (collection: Collection, item: Item) => void;
  onOpenHistory: (entry: HistoryEntry) => void;
  onChange: (collection: Collection) => void;
  // A request or folder was dragged elsewhere; changed is what moveItem returned.
  onMove: (changed: Collection[], itemId: string) => void;
  onDelete: (id: string) => void;
  onProto: (collection: Collection) => void;
  onClearHistory: () => void;
  onReload: () => void;
};

export function Sidebar(p: Props) {
  const [view, setView] = useState<'collections' | 'history'>('collections');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [renaming, setRenaming] = useState<string | null>(null);
  const [filter, setFilter] = useState('');
  const [asking, setAsking] = useState<{ title: string; message: string; onConfirm: () => void } | null>(null);
  const treeRef = useRef<HTMLDivElement>(null);
  // What is being dragged, and the folder or collection it would land in ("collection/folder").
  const [dragging, setDragging] = useState<{ from: string; id: string } | null>(null);
  const [over, setOver] = useState<string | null>(null);
  const filtering = filter.trim() !== '';
  // Only what is drawn is narrowed; every change is made to the full collection.
  const shown = filterCollections(p.collections, filter);
  const isOpen = (id: string) => filtering || !collapsed.has(id);

  const setOpen = (id: string, open: boolean) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (open) next.delete(id);
      else next.add(id);
      return next;
    });

  const newCollection = () => {
    const c: Collection = { id: uid(), name: 'New Collection', items: [] };
    p.onChange(c);
    setFilter('');
    setRenaming(c.id);
  };

  const add = (c: Collection, parentId: string | null, folder: boolean) => {
    const item: Item = folder
      ? { id: uid(), name: 'New Folder', items: [] }
      : { id: uid(), name: 'New Request', request: emptyRequest() };
    const next = { ...c, items: addItem(c.items, parentId, item) };
    p.onChange(next);
    setOpen(parentId ?? c.id, true);
    setFilter('');
    setRenaming(item.id);
    if (!folder) p.onOpen(next, item);
  };

  const endDrag = () => {
    setDragging(null);
    setOver(null);
  };

  // A folder or collection, with everything drawn inside it, takes drops for itself:
  // dropping on a request moves the dragged item next to it.
  const dropZone = (c: Collection, into: string | null) => {
    const key = `${c.id}/${into ?? ''}`;
    const move = () => {
      const from = dragging && p.collections.find((x) => x.id === dragging.from);
      return from ? moveItem(from, c, dragging.id, into) : null;
    };
    return {
      className: `rounded-md ${over === key ? 'bg-hover' : ''}`,
      onDragOver: (e: React.DragEvent) => {
        if (!dragging) return;
        e.stopPropagation();
        if (!move()) return setOver(null);
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        setOver(key);
      },
      onDrop: (e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        const changed = move();
        if (changed && dragging) {
          p.onMove(changed, dragging.id);
          setOpen(into ?? c.id, true);
        }
        endDrag();
      },
    };
  };

  const dragProps = (c: Collection, item: Item) => ({
    onDragStart: (e: React.DragEvent) => {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', item.name);
      setDragging({ from: c.id, id: item.id });
    },
    onDragEnd: endDrag,
  });

  const renderItems = (c: Collection, items: Item[], depth: number) =>
    items.map((item) => {
      const rename = (name: string) => p.onChange({ ...c, items: mapItem(c.items, item.id, (it) => ({ ...it, name })) });
      const remove = () =>
        setAsking({
          title: isFolder(item) ? 'Delete folder' : 'Delete request',
          message: isFolder(item) ? `Delete "${item.name}" and everything in it?` : `Delete "${item.name}"?`,
          onConfirm: () => p.onChange({ ...c, items: removeItem(c.items, item.id) }),
        });
      if (!isFolder(item)) {
        return (
          <Row
            key={item.id}
            depth={depth}
            name={item.name}
            lead={<Method request={item.request!} />}
            active={item.id === p.activeItemId}
            renaming={renaming === item.id}
            onRename={rename}
            onRenameStart={() => setRenaming(item.id)}
            onRenameDone={() => setRenaming(null)}
            onClick={() => p.onOpen(c, item)}
            drag={dragProps(c, item)}
            actions={
              <>
                <IconButton icon="pencil" title="Rename (F2)" onClick={() => setRenaming(item.id)} />
                <IconButton icon="trash" title="Delete" danger onClick={remove} />
              </>
            }
          />
        );
      }
      const open = isOpen(item.id);
      const children = item.items ?? [];
      return (
        <div key={item.id} {...dropZone(c, item.id)}>
          <Row
            depth={depth}
            drag={dragProps(c, item)}
            name={item.name}
            lead={<Chevron open={open} />}
            expanded={open}
            renaming={renaming === item.id}
            onRename={rename}
            onRenameStart={() => setRenaming(item.id)}
            onRenameDone={() => setRenaming(null)}
            onClick={() => setOpen(item.id, !open)}
            actions={
              <>
                <IconButton icon="filePlus" title="Add request" onClick={() => add(c, item.id, false)} />
                <IconButton icon="folderPlus" title="Add folder" onClick={() => add(c, item.id, true)} />
                <IconButton icon="pencil" title="Rename (F2)" onClick={() => setRenaming(item.id)} />
                <IconButton icon="trash" title="Delete" danger onClick={remove} />
              </>
            }
          />
          {open && (children.length ? renderItems(c, children, depth + 1) : <EmptyRow depth={depth + 1}>Empty folder</EmptyRow>)}
        </div>
      );
    });

  return (
    <aside className={`shrink-0 bg-panel flex-col min-h-0 ${p.hidden ? 'hidden' : 'flex'}`} style={{ width: p.width }}>
      <Tabs
        active={view}
        onChange={setView}
        tabs={[
          { id: 'collections', label: 'Collections' },
          { id: 'history', label: 'History' },
        ]}
        right={
          view === 'collections' ? (
            <div className="flex">
              <IconButton icon="plus" title="New collection" onClick={newCollection} />
              <IconButton icon="refresh" title="Reload from disk (e.g. after git pull)" onClick={p.onReload} />
            </div>
          ) : (
            p.history.length > 0 && (
              <button className={`${ghostBtnCls} h-6 px-1.5 text-xs`} onClick={p.onClearHistory}>
                Clear
              </button>
            )
          )
        }
      />
      {view === 'collections' ? (
        <>
        {p.collections.length > 0 && (
          <label className="mx-1.5 mt-1.5 flex h-7 shrink-0 items-center gap-1.5 rounded-md pl-2 pr-0.5 text-faint transition-colors duration-100 hover:bg-hover focus-within:bg-hover focus-within:shadow-focus">
            <Icon name="search" size={12} />
            <input
              className="h-full min-w-0 flex-1 bg-transparent text-fg outline-none placeholder:text-faint"
              aria-label="Filter collections"
              spellCheck={false}
              placeholder="Filter"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape' && filter) {
                  e.preventDefault();
                  setFilter('');
                } else if (e.key === 'ArrowDown') {
                  e.preventDefault();
                  treeRef.current?.querySelector<HTMLElement>('[data-row]')?.focus();
                }
              }}
            />
            {filter && <IconButton icon="x" title="Clear filter (Esc)" onClick={() => setFilter('')} />}
          </label>
        )}
        <div ref={treeRef} className="flex-1 select-none overflow-auto p-1.5" data-tree onDragOver={() => setOver(null)}>
          {filtering && shown.length === 0 && (
            <p className="break-words px-1.5 py-2.5 text-muted">Nothing matches “{filter.trim()}”.</p>
          )}
          {p.collections.length === 0 && (
            <div className="flex flex-col items-start gap-3 px-1.5 py-2.5">
              <p className="text-muted">No collections yet. A collection is a folder of saved requests in your workspace.</p>
              <button className={btnCls} onClick={newCollection}>
                New collection
              </button>
            </div>
          )}
          {shown.map((visible) => {
            const c = p.collections.find((x) => x.id === visible.id) ?? visible;
            const open = isOpen(c.id);
            return (
              <div key={c.id} {...dropZone(c, null)}>
                <Row
                  depth={0}
                  name={c.name}
                  bold
                  lead={<Chevron open={open} />}
                  expanded={open}
                  renaming={renaming === c.id}
                  onRename={(name) => p.onChange({ ...c, name })}
                  onRenameStart={() => setRenaming(c.id)}
                  onRenameDone={() => setRenaming(null)}
                  onClick={() => setOpen(c.id, !open)}
                  actions={
                    <>
                      <IconButton icon="filePlus" title="Add request" onClick={() => add(c, null, false)} />
                      <IconButton icon="folderPlus" title="Add folder" onClick={() => add(c, null, true)} />
                      <IconButton icon="sliders" title="Proto files for gRPC" onClick={() => p.onProto(c)} />
                      <IconButton icon="pencil" title="Rename (F2)" onClick={() => setRenaming(c.id)} />
                      <IconButton
                        icon="trash"
                        title="Delete collection"
                        danger
                        onClick={() =>
                          setAsking({
                            title: 'Delete collection',
                            message: `Delete "${c.name}" and everything in it?`,
                            onConfirm: () => p.onDelete(c.id),
                          })
                        }
                      />
                    </>
                  }
                />
                {open && (visible.items.length ? renderItems(c, visible.items, 1) : <EmptyRow depth={1}>No requests yet</EmptyRow>)}
              </div>
            );
          })}
        </div>
        </>
      ) : (
        <div className="flex-1 select-none overflow-auto p-1.5">
          {p.history.length === 0 ? (
            <p className="px-1.5 py-2.5 text-muted">Requests you send show up here. History is kept on this machine only.</p>
          ) : (
            <p className="px-1.5 pb-1.5 pt-1 text-xs text-muted">Kept on this machine only</p>
          )}
          {p.history.map((h) => (
            <button
              key={h.id}
              className="w-full flex items-center gap-2 rounded-md px-1.5 h-7 hover:bg-hover text-left focus-visible:-outline-offset-2"
              title={`${h.request.url}\n${new Date(h.at).toLocaleString()}${h.error ? `\n${h.error}` : ''}`}
              onClick={() => p.onOpenHistory(h)}
            >
              <Method request={h.request} />
              <span className="flex-1 truncate font-mono text-xs">{h.request.url}</span>
              {!!h.failed && (
                <span className="flex items-center gap-0.5 text-xs tabular-nums text-danger" title={`${h.failed} failed checks`}>
                  <Icon name="x" size={11} strokeWidth={2.5} />
                  {h.failed}
                </span>
              )}
              <span className={`text-xs tabular-nums ${h.error ? 'text-danger' : statusColor(h.status, h.request.protocol === 'grpc')}`}>
                {h.error ? 'ERR' : h.request.protocol === 'grpc' && h.status === 0 ? 'OK' : h.status}
              </span>
            </button>
          ))}
        </div>
      )}
      {asking && <Confirm {...asking} action="Delete" onClose={() => setAsking(null)} />}
    </aside>
  );
}

function Chevron({ open }: { open: boolean }) {
  return (
    <span className={`text-faint transition-transform duration-100 ${open ? 'rotate-90' : ''}`}>
      <Icon name="chevron" size={12} />
    </span>
  );
}

// Nesting is shown by indentation alone.
function Indent({ depth }: { depth: number }) {
  return depth > 0 ? <span className="shrink-0" style={{ width: depth * 14 }} /> : null;
}

function EmptyRow({ depth, children }: { depth: number; children: React.ReactNode }) {
  return (
    <div className="flex h-7 items-center gap-2 px-1.5 text-xs text-faint">
      <Indent depth={depth} />
      <span>{children}</span>
    </div>
  );
}

type RowProps = {
  depth: number;
  name: string;
  lead: React.ReactNode;
  actions: React.ReactNode;
  bold?: boolean;
  active?: boolean;
  // set for folders and collections
  expanded?: boolean;
  renaming: boolean;
  onRename: (name: string) => void;
  onRenameStart: () => void;
  onRenameDone: () => void;
  onClick: () => void;
  // set for requests and folders, which can be dragged into another folder or collection
  drag?: { onDragStart: (e: React.DragEvent) => void; onDragEnd: () => void };
};

function Row(p: RowProps) {
  const commit = (value: string) => {
    const name = value.trim();
    if (name && name !== p.name) p.onRename(name);
    p.onRenameDone();
  };
  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.target !== e.currentTarget) return;
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      p.onClick();
    } else if (e.key === 'F2') {
      e.preventDefault();
      p.onRenameStart();
    } else if ((e.key === 'ArrowRight' && p.expanded === false) || (e.key === 'ArrowLeft' && p.expanded === true)) {
      e.preventDefault();
      p.onClick();
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const rows = [...(e.currentTarget.closest('[data-tree]')?.querySelectorAll<HTMLElement>('[data-row]') ?? [])];
      rows[rows.indexOf(e.currentTarget) + (e.key === 'ArrowDown' ? 1 : -1)]?.focus();
    }
  };
  return (
    <div
      data-row
      role="button"
      tabIndex={0}
      aria-expanded={p.expanded}
      aria-current={p.active || undefined}
      title={p.renaming ? undefined : p.name}
      className={`group flex items-center gap-2 rounded-md pl-1.5 pr-0.5 h-7 outline-none focus-visible:shadow-focus ${
        p.active ? 'bg-active' : 'hover:bg-hover focus-visible:bg-hover'
      }`}
      onClick={p.onClick}
      onKeyDown={onKeyDown}
      draggable={!!p.drag && !p.renaming}
      {...(p.renaming ? {} : p.drag)}
    >
      <Indent depth={p.depth} />
      {p.lead}
      {p.renaming ? (
        <input
          autoFocus
          className="mr-1 h-[22px] min-w-0 flex-1 select-text rounded border border-focus bg-raised px-1 outline-none ring-2 ring-focus/25"
          aria-label="Name"
          defaultValue={p.name}
          onFocus={(e) => e.target.select()}
          onClick={(e) => e.stopPropagation()}
          onBlur={(e) => commit(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') commit(e.currentTarget.value);
            if (e.key === 'Escape') p.onRenameDone();
          }}
        />
      ) : (
        <>
          <span className={`flex-1 truncate ${p.bold ? 'font-medium' : ''}`}>{p.name}</span>
          <span className="hidden group-hover:flex group-focus-within:flex">{p.actions}</span>
        </>
      )}
    </div>
  );
}
