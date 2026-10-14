// Path helpers for values picked in the folder and file browser. Paths come
// from the server as absolute paths in the machine's own format, so both
// separators are handled.

const split = (path: string) => path.split(/[\\/]+/).filter(Boolean);
const isAbsolute = (path: string) => /^([\\/]|[A-Za-z]:[\\/])/.test(path);

// The path of child below parent with forward slashes, "." when they are the
// same folder, or null when child is not inside parent.
export function relativeTo(parent: string, child: string): string | null {
  if (!isAbsolute(parent) || !isAbsolute(child)) return null;
  const a = split(parent);
  const b = split(child);
  if (b.length < a.length || a.some((part, i) => part !== b[i])) return null;
  return b.slice(a.length).join('/') || '.';
}

// A picked folder as it should be saved with a collection: relative to the
// workspace when it is inside it, so the setting works on a teammate's
// machine, and absolute otherwise.
export function portablePath(picked: string, workspace: string): string {
  return relativeTo(workspace, picked) ?? picked;
}

function resolveRoot(root: string, workspace: string): string | null {
  if (root.includes('{{') || root.startsWith('~')) return null;
  if (isAbsolute(root)) return root;
  return root === '.' ? workspace : `${workspace}/${root}`;
}

// A picked .proto file as it should be saved: relative to the first import
// root that contains it (the form imports use), otherwise like a folder.
export function protoFilePath(picked: string, roots: string[], workspace: string): string {
  for (const root of roots) {
    const resolved = resolveRoot(root, workspace);
    const relative = resolved && relativeTo(resolved, picked);
    if (relative && relative !== '.') return relative;
  }
  return portablePath(picked, workspace);
}
