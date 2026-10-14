// True inside the native desktop window, which injects its own runtime.
export const isDesktop = typeof window !== 'undefined' && ('runtime' in window || 'wails' in window || 'WailsInvoke' in window);

// Last segment of a path, for either separator: "/a/b/requests/" -> "requests".
export function folderName(path: string): string {
  const parts = path.split(/[\\/]+/).filter(Boolean);
  return parts[parts.length - 1] ?? path;
}
