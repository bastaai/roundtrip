import { useEffect, useState } from 'react';
import { api } from '../api';
import type { FsListing } from '../types';
import { btnCls, ghostBtnCls, Icon, inputCls, primaryBtnCls, Spinner } from './ui';

type Props = {
  // "folder" picks the folder being viewed; "files" picks one or more files in it.
  mode: 'folder' | 'files';
  // File extension to show in "files" mode, e.g. ".proto".
  ext?: string;
  // Absolute path to start in; the workspace folder when empty.
  start?: string;
  onPick: (paths: string[]) => void;
  onCancel: () => void;
};

const join = (dir: string, name: string) => (/[\\/]$/.test(dir) ? dir + name : dir + (dir.includes('\\') ? '\\' : '/') + name);

// An inline browser over the machine's folders. The interface runs in a web
// view and cannot open the system file dialog, so the server lists the
// directories and this component navigates them.
export function PathBrowser({ mode, ext = '', start = '', onPick, onCancel }: Props) {
  const [listing, setListing] = useState<FsListing>();
  const [address, setAddress] = useState(start);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const go = async (path: string) => {
    setLoading(true);
    setError('');
    try {
      const next = await api.listDir(path, mode === 'files' ? ext : '');
      setListing(next);
      setAddress(next.path);
      setSelected([]);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
    setLoading(false);
  };

  useEffect(() => {
    go(start);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const toggle = (name: string) => setSelected((names) => (names.includes(name) ? names.filter((n) => n !== name) : [...names, name]));
  const folders = listing?.entries.filter((e) => e.dir) ?? [];
  const files = listing?.entries.filter((e) => !e.dir) ?? [];
  const rowCls = 'flex h-7 w-full shrink-0 items-center gap-2 rounded-md px-2 text-left hover:bg-hover';

  return (
    <div className="flex flex-col gap-2 rounded-md bg-panel p-2" role="group" aria-label={mode === 'folder' ? 'Choose a folder' : 'Choose files'}>
      <div className="flex items-center gap-1">
        <button className={`${ghostBtnCls} px-2`} disabled={!listing?.parent} onClick={() => listing && go(listing.parent)} title="Up one folder">
          <span className="-rotate-90">
            <Icon name="chevron" size={13} />
          </span>
          Up
        </button>
        <button className={`${ghostBtnCls} px-2`} onClick={() => listing && go(listing.workspace)} title={listing?.workspace}>
          Workspace
        </button>
        <button className={`${ghostBtnCls} px-2`} onClick={() => listing && go(listing.home)} title={listing?.home}>
          Home
        </button>
        <input
          className={`${inputCls} ml-1 flex-1 font-mono text-xs`}
          aria-label="Folder"
          spellCheck={false}
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              // Enter navigates; it must not submit the dialog around this.
              e.stopPropagation();
              go(address);
            }
          }}
        />
        {loading && <Spinner size={13} />}
      </div>

      <div className="flex h-48 flex-col overflow-auto">
        {folders.map((entry) => (
          <button key={entry.name} className={rowCls} onClick={() => listing && go(join(listing.path, entry.name))}>
            <span className="text-muted">
              <Icon name="folder" size={13} />
            </span>
            <span className="truncate">{entry.name}</span>
          </button>
        ))}
        {files.map((entry) => (
          <label key={entry.name} className={`${rowCls} cursor-pointer`}>
            <input type="checkbox" checked={selected.includes(entry.name)} onChange={() => toggle(entry.name)} />
            <span className="truncate font-mono text-xs">{entry.name}</span>
          </label>
        ))}
        {listing && !loading && folders.length + files.length === 0 && (
          <p className="px-2 py-1.5 text-muted">{mode === 'files' ? `No folders or ${ext} files here.` : 'No folders here.'}</p>
        )}
      </div>

      {error && <p className="px-2 text-xs text-danger">{error}</p>}

      <div className="flex items-center justify-end gap-2">
        {mode === 'files' && files.length > 0 && (
          <button
            className={`${ghostBtnCls} mr-auto px-2`}
            onClick={() => setSelected(selected.length === files.length ? [] : files.map((f) => f.name))}
          >
            {selected.length === files.length ? 'Select none' : 'Select all'}
          </button>
        )}
        <button className={btnCls} onClick={onCancel}>
          Cancel
        </button>
        {mode === 'folder' ? (
          <button className={primaryBtnCls} disabled={!listing} onClick={() => listing && onPick([listing.path])}>
            Use this folder
          </button>
        ) : (
          <button className={primaryBtnCls} disabled={!listing || selected.length === 0} onClick={() => listing && onPick(selected.map((name) => join(listing.path, name)))}>
            {selected.length > 1 ? `Add ${selected.length} files` : 'Add file'}
          </button>
        )}
      </div>
    </div>
  );
}
