import { useCallback, useEffect, useRef, useState, type DragEvent } from 'react';
import { useFormContext } from 'react-hook-form';
import { CheckCircle2, FileAudio, FolderUp, Loader2, Trash2, UploadCloud } from 'lucide-react';
import { Button } from '@/shared/ui/Button';
import { formatBytes, shortDigest } from '@/shared/lib/format';
import { cn } from '@/shared/lib/cn';
import { api } from '@/shared/api';
import { sha256File } from './hash';
import type { WizardValues } from './schema';
import { detectPlugins, isExecutableCandidate, mergePlugins, sniffFile } from './detectPlugins';
import { collectDropped, isJunk, type PickedFile } from './pickFiles';
import { PluginBuildsEditor } from './PluginBuildsEditor';

const ACCEPT = '.wav,.flac,.aif,.aiff,.mp3,.json,.mid,.clap,.vst3,.onnx,.zip';

function guessType(name: string, type: string): string {
  if (type) return type;
  const ext = name.split('.').pop()?.toLowerCase();
  if (ext === 'wav') return 'audio/wav';
  if (ext === 'flac') return 'audio/flac';
  if (ext === 'json') return 'application/json';
  if (ext === 'mid') return 'audio/midi';
  return 'application/octet-stream';
}

/** At most this many files hash/upload at once (a plug-in bundle can hold hundreds of files). */
const CONCURRENCY = 4;

function limiter(max: number) {
  let active = 0;
  const queue: (() => void)[] = [];
  const next = () => {
    if (active >= max) return;
    const run = queue.shift();
    if (run) {
      active += 1;
      run();
    }
  };
  return <T,>(task: () => Promise<T>): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      queue.push(() => {
        task()
          .then(resolve, reject)
          .finally(() => {
            active -= 1;
            next();
          });
      });
      next();
    });
}

/**
 * Files are hashed client-side (SHA-256) and uploaded in resumable parts; the server re-hashes
 * the assembled blob and stores it at `blobs/{sha256}`. Already-stored content is skipped.
 */
export function StepFiles() {
  const { formState, getValues, setValue, trigger, watch } = useFormContext<WizardValues>();
  const fields = watch('files');
  const [dragOver, setDragOver] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const folderRef = useRef<HTMLInputElement>(null);
  const limit = useRef(limiter(CONCURRENCY)).current;

  // Not in React's typings; lets the picker select whole folders (.vst3/.clap bundles are folders).
  useEffect(() => {
    folderRef.current?.setAttribute('webkitdirectory', '');
    folderRef.current?.setAttribute('directory', '');
  }, []);

  // Keep plugins[] in step with the uploaded bundles (user edits are preserved).
  const pathsKey = fields.map((f) => `${f.path}|${f.binary?.os ?? ''}|${f.binary?.arch.join('+') ?? ''}`).join('\n');
  useEffect(() => {
    const files = getValues('files');
    const next = mergePlugins(getValues('plugins'), detectPlugins(files), files.map((f) => f.path));
    if (JSON.stringify(next) !== JSON.stringify(getValues('plugins'))) setValue('plugins', next, { shouldDirty: true });
  }, [pathsKey, getValues, setValue]);

  const patch = useCallback(
    (id: string, p: Partial<WizardValues['files'][number]>) => {
      const files = getValues('files');
      if (!files.some((f) => f.id === id)) return;
      setValue('files', files.map((f) => (f.id === id ? { ...f, ...p } : f)), { shouldDirty: true });
    },
    [getValues, setValue],
  );

  const addFiles = useCallback(
    (list: PickedFile[]) => {
      const existing = new Set(getValues('files').map((f) => f.path));
      for (const { file, path } of list) {
        if (isJunk(path) || existing.has(path)) continue;
        const id = `${path}:${file.size}:${Math.random().toString(36).slice(2, 8)}`;
        existing.add(path);
        setValue('files', [...getValues('files'), { id, path, size: file.size, contentType: guessType(file.name, file.type), digest: null, status: 'hashing', progress: 0 }], { shouldDirty: true });
        const contentType = guessType(file.name, file.type);
        if (isExecutableCandidate(path)) {
          void sniffFile(file).then((binary) => binary && patch(id, { binary }), () => undefined);
        }
        void limit(() => sha256File(file)
          .then(async (digest) => {
            patch(id, { digest, status: 'uploading', progress: 0 });
            await api.uploadBlob(file, {
              digest,
              contentType,
              onProgress: ({ fraction, phase }) =>
                patch(id, { progress: phase === 'done' ? 1 : Math.min(0.99, fraction), status: phase === 'done' ? 'done' : 'uploading' }),
            });
            patch(id, { status: 'done', progress: 1 });
            void trigger('files');
          }))
          .catch((e: unknown) => {
            patch(id, { status: 'error' });
            setUploadError(e instanceof Error ? e.message : 'Upload failed');
          });
      }
    },
    [getValues, setValue, patch, trigger, limit],
  );

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    // Folders (plug-in bundles) are walked recursively; plain files keep their name as path.
    void collectDropped(e.dataTransfer).then(addFiles, (err: unknown) => setUploadError(err instanceof Error ? err.message : 'Could not read the dropped folder'));
  };

  const fromInput = (list: FileList | null): PickedFile[] =>
    [...(list ?? [])].map((file) => ({ file, path: (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name }));

  const total = fields.reduce((n, f) => n + f.size, 0);
  const error = formState.errors.files?.message ?? formState.errors.files?.root?.message;

  return (
    <div className="space-y-4">
      <div
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={onDrop}
        className={cn('flex flex-col items-center justify-center rounded-lg border-2 border-dashed px-6 py-10 text-center transition-colors', dragOver ? 'border-primary bg-primary/10' : 'border-border')}
      >
        <UploadCloud className="h-10 w-10 text-fg-subtle" aria-hidden />
        <p className="mt-3 font-medium">Drag & drop audio, presets or plug-in bundles</p>
        <p className="mt-1 text-caption text-fg-muted">WAV, FLAC, AIFF, MIDI, JSON presets, ONNX models, VST3 / CLAP plug-ins — drop the whole <code className="font-mono">.vst3</code> / <code className="font-mono">.clap</code> folder</p>
        <div className="mt-4 flex flex-wrap justify-center gap-2">
          <Button variant="secondary" onClick={() => inputRef.current?.click()}>
            Browse files
          </Button>
          <Button variant="secondary" leftIcon={<FolderUp className="h-4 w-4" />} onClick={() => folderRef.current?.click()}>
            Add folder
          </Button>
        </div>
        <input
          ref={inputRef}
          type="file"
          multiple
          accept={ACCEPT}
          className="sr-only"
          aria-label="Choose files to upload"
          data-testid="file-input"
          onChange={(e) => {
            addFiles(fromInput(e.target.files));
            e.target.value = '';
          }}
        />
        <input
          ref={folderRef}
          type="file"
          multiple
          className="sr-only"
          aria-label="Choose a folder to upload"
          data-testid="folder-input"
          onChange={(e) => {
            addFiles(fromInput(e.target.files));
            e.target.value = '';
          }}
        />
      </div>

      {error && <p className="text-caption text-danger" role="alert">{error}</p>}
      {uploadError && <p className="text-caption text-danger" role="alert">Upload failed: {uploadError}</p>}

      <PluginBuildsEditor />

      {fields.length > 0 && (
        <div className="rounded-lg border border-border">
          <div className="flex items-center justify-between border-b border-border px-4 py-2 text-caption text-fg-muted">
            <span>{fields.length} files · {formatBytes(total)}</span>
            <span>Blob key = sha256 of content</span>
          </div>
          <ul className="divide-y divide-border" aria-label="Files">
            {fields.map((f) => (
              <li key={f.id} className="flex items-center gap-3 px-4 py-2.5">
                <FileAudio className="h-4 w-4 shrink-0 text-accent" aria-hidden />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-mono text-caption">{f.path}</p>
                  <p className="font-mono text-caption text-fg-subtle">{f.digest ? shortDigest(f.digest) : 'computing sha256…'} · {formatBytes(f.size)}</p>
                  {f.status !== 'done' && f.status !== 'error' && (
                    <div className="mt-1 h-1 overflow-hidden rounded-full bg-surface-raised" role="progressbar" aria-label={`Upload progress for ${f.path}`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(f.progress * 100)}>
                      <div className="h-full bg-primary transition-all" style={{ width: `${f.progress * 100}%` }} />
                    </div>
                  )}
                </div>
                <span className="w-20 text-right text-caption">
                  {f.status === 'done' && <span className="inline-flex items-center gap-1 text-success"><CheckCircle2 className="h-4 w-4" aria-hidden />done</span>}
                  {(f.status === 'hashing' || f.status === 'uploading') && <span className="inline-flex items-center gap-1 text-fg-muted"><Loader2 className="h-4 w-4 animate-spin" aria-hidden />{f.status}</span>}
                  {f.status === 'error' && <span className="text-danger">error</span>}
                </span>
                <button type="button" aria-label={`Remove ${f.path}`} onClick={() => setValue('files', getValues('files').filter((x) => x.id !== f.id), { shouldDirty: true })} className="rounded p-1 text-fg-subtle hover:text-danger">
                  <Trash2 className="h-4 w-4" />
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
