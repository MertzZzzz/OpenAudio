/** A file plus its path inside the release (folder structure preserved). */
export interface PickedFile {
  file: File;
  path: string;
}

/** OS clutter that must never become a release artifact (code signatures in _CodeSignature are kept). */
export function isJunk(path: string): boolean {
  const name = path.split('/').pop() ?? '';
  return name === '.DS_Store' || name === 'Thumbs.db' || name === 'desktop.ini' || name.startsWith('._') || path.startsWith('__MACOSX/') || path.includes('/__MACOSX/');
}

async function walk(entry: FileSystemEntry, out: PickedFile[]): Promise<void> {
  const path = entry.fullPath.replace(/^\/+/, '');
  if (entry.isFile) {
    const file = await new Promise<File>((ok, fail) => (entry as FileSystemFileEntry).file(ok, fail));
    out.push({ file, path });
    return;
  }
  if (!entry.isDirectory) return;
  const reader = (entry as FileSystemDirectoryEntry).createReader();
  // readEntries returns batches (≤ 100 in Chrome) until it yields an empty one.
  for (;;) {
    const batch = await new Promise<FileSystemEntry[]>((ok, fail) => reader.readEntries(ok, fail));
    if (!batch.length) break;
    for (const child of batch) await walk(child, out);
  }
}

/** Files from a drop, walking dropped folders recursively. */
export async function collectDropped(dt: DataTransfer): Promise<PickedFile[]> {
  const entries = [...dt.items]
    .map((item) => (item.kind === 'file' ? item.webkitGetAsEntry() : null))
    .filter((e): e is FileSystemEntry => e !== null);
  if (!entries.length) return [...dt.files].map((file) => ({ file, path: file.name }));
  const out: PickedFile[] = [];
  for (const e of entries) await walk(e, out);
  return out;
}
