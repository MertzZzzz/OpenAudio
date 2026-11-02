import { useCallback, useRef, useState, type DragEvent } from 'react';
import { useFormContext } from 'react-hook-form';
import { CheckCircle2, FileAudio, Loader2, Trash2, UploadCloud } from 'lucide-react';
import { Button } from '@/shared/ui/Button';
import { formatBytes, shortDigest } from '@/shared/lib/format';
import { cn } from '@/shared/lib/cn';
import { api } from '@/shared/api';
import { sha256File } from './hash';
import type { WizardValues } from './schema';

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

  const patch = useCallback(
    (id: string, p: Partial<WizardValues['files'][number]>) => {
      const files = getValues('files');
      if (!files.some((f) => f.id === id)) return;
      setValue('files', files.map((f) => (f.id === id ? { ...f, ...p } : f)), { shouldDirty: true });
    },
    [getValues, setValue],
  );

  const addFiles = useCallback(
    (list: FileList | File[]) => {
      const files = [...list];
      const existing = new Set(getValues('files').map((f) => f.path));
      for (const file of files) {
        const path = (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name;
        if (existing.has(path)) continue;
        const id = `${path}:${file.size}:${Math.random().toString(36).slice(2, 8)}`;
        existing.add(path);
        setValue('files', [...getValues('files'), { id, path, size: file.size, contentType: guessType(file.name, file.type), digest: null, status: 'hashing', progress: 0 }], { shouldDirty: true });
        const contentType = guessType(file.name, file.type);
        void sha256File(file)
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
          })
          .catch((e: unknown) => {
            patch(id, { status: 'error' });
            setUploadError(e instanceof Error ? e.message : 'Upload failed');
          });
      }
    },
    [getValues, setValue, patch, trigger],
  );

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    if (e.dataTransfer.files.length) addFiles(e.dataTransfer.files);
  };

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
        <p className="mt-3 font-medium">Drag & drop audio, presets or module binaries</p>
        <p className="mt-1 text-caption text-fg-muted">WAV, FLAC, AIFF, MIDI, JSON presets, CLAP/VST3 bundles, ONNX models</p>
        <Button variant="secondary" className="mt-4" onClick={() => inputRef.current?.click()}>
          Browse files
        </Button>
        <input
          ref={inputRef}
          type="file"
          multiple
          accept={ACCEPT}
          className="sr-only"
          aria-label="Choose files to upload"
          data-testid="file-input"
          onChange={(e) => {
            if (e.target.files) addFiles(e.target.files);
            e.target.value = '';
          }}
        />
      </div>

      {error && <p className="text-caption text-danger" role="alert">{error}</p>}
      {uploadError && <p className="text-caption text-danger" role="alert">Upload failed: {uploadError}</p>}

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
