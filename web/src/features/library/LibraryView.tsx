import { useRef } from 'react';
import { AlertTriangle, CheckCircle2, Download, FileUp, Library as LibraryIcon, RefreshCw } from 'lucide-react';
import { Button, ButtonLink } from '@/shared/ui/Button';
import { Card, CardBody, CardHeader, CardTitle } from '@/shared/ui/Card';
import { Table, TBody, TH, THead } from '@/shared/ui/Table';
import { Skeleton } from '@/shared/ui/Skeleton';
import { EmptyState } from '@/shared/ui/EmptyState';
import { Badge } from '@/shared/ui/Badge';
import { useToast } from '@/shared/ui/Toast';
import { downloadJson } from '@/shared/lib/download';
import { shortDigest } from '@/shared/lib/format';
import { maxSatisfying } from '@/shared/lib/semver';
import { useImportLockfile, useLibrary, usePinVersion, useResolve } from './api';
import { LibraryRow } from './LibraryRow';
import { lockfileSchema } from './lockfileSchema';

export function LibraryView() {
  const library = useLibrary();
  const resolve = useResolve();
  const pin = usePinVersion();
  const importLf = useImportLockfile();
  const { toast } = useToast();
  const fileRef = useRef<HTMLInputElement>(null);

  const items = library.data?.items ?? [];
  const conflicts = resolve.data?.conflicts ?? [];
  const lockfile = resolve.data?.lockfile;
  const libraryIds = new Set(items.map((i) => i.packageId));

  const onImport = async (file: File) => {
    try {
      const parsed = lockfileSchema.safeParse(JSON.parse(await file.text()));
      if (!parsed.success) {
        toast({ tone: 'error', title: 'Invalid lockfile', description: parsed.error.issues[0]?.message });
        return;
      }
      importLf.mutate(parsed.data);
    } catch {
      toast({ tone: 'error', title: 'Could not read file', description: 'Expected openaudio.lock.json' });
    }
  };

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-display">Library</h1>
          <p className="text-fg-muted">Installed packages, pinned versions and your project lockfile.</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" leftIcon={<FileUp className="h-4 w-4" />} onClick={() => fileRef.current?.click()} loading={importLf.isPending}>
            Import lockfile
          </Button>
          <input
            ref={fileRef}
            type="file"
            accept="application/json,.json"
            className="sr-only"
            aria-label="Import lockfile"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void onImport(f);
              e.target.value = '';
            }}
          />
          <Button
            leftIcon={<Download className="h-4 w-4" />}
            disabled={!lockfile || conflicts.length > 0}
            title={conflicts.length ? 'Resolve conflicts before exporting' : undefined}
            onClick={() => {
              if (!lockfile) return;
              downloadJson('openaudio.lock.json', lockfile);
              toast({ tone: 'success', title: 'Lockfile exported', description: `${Object.keys(lockfile.packages).length} packages with exact versions and digests` });
            }}
          >
            Export lockfile
          </Button>
        </div>
      </div>

      <section aria-labelledby="installed-h" className="space-y-3">
        <h2 id="installed-h" className="text-heading">Installed <span className="text-fg-subtle">({items.length})</span></h2>
        {library.isLoading ? (
          <Skeleton className="h-64" />
        ) : items.length === 0 ? (
          <EmptyState icon={<LibraryIcon className="h-6 w-6" />} title="Your library is empty" description="Add packages from the catalog to pin versions and generate a lockfile." action={<ButtonLink to="/">Discover packages</ButtonLink>} />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Package</TH>
                <TH>Installed</TH>
                <TH>Latest</TH>
                <TH>Pinned</TH>
                <TH><span className="sr-only">Actions</span></TH>
              </tr>
            </THead>
            <TBody>
              {items.map((i) => (
                <LibraryRow key={i.packageId} item={i} />
              ))}
            </TBody>
          </Table>
        )}
      </section>

      {conflicts.length > 0 && (
        <Card className="border-warning/40">
          <CardHeader>
            <CardTitle className="flex items-center gap-2"><AlertTriangle className="h-5 w-5 text-warning" aria-hidden /> Resolve conflicts</CardTitle>
            <Badge tone="warning">{conflicts.length}</Badge>
          </CardHeader>
          <ul className="divide-y divide-border">
            {conflicts.map((c) => {
              const depRanges = c.requestedBy.filter((r) => r.packageId !== 'library');
              const fix = depRanges.reduce<string[]>((pool, r) => pool.filter((v) => maxSatisfying([v], r.range)), c.candidates);
              const best = maxSatisfying(fix, '*');
              return (
                <li key={c.packageId} className="space-y-2 px-6 py-4">
                  <p className="font-mono text-body font-medium">{c.packageId}</p>
                  <ul className="space-y-1 text-caption text-fg-muted">
                    {c.requestedBy.map((r, n) => (
                      <li key={n}>
                        <span className="font-mono">{r.packageId === 'library' ? 'your pin' : r.packageId}</span> requires <span className="font-mono text-fg">{r.range}</span>
                      </li>
                    ))}
                  </ul>
                  <div className="flex flex-wrap gap-2 pt-1">
                    {best && libraryIds.has(c.packageId) && (
                      <Button size="sm" onClick={() => pin.mutate({ packageId: c.packageId, version: best })}>
                        Pin to {best}
                      </Button>
                    )}
                    {libraryIds.has(c.packageId) && (
                      <Button size="sm" variant="secondary" onClick={() => pin.mutate({ packageId: c.packageId, version: null })}>
                        Unpin
                      </Button>
                    )}
                    {!best && <span className="text-caption text-danger">No published version satisfies every range.</span>}
                  </div>
                </li>
              );
            })}
          </ul>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {conflicts.length === 0 && lockfile ? <CheckCircle2 className="h-5 w-5 text-success" aria-hidden /> : null}
            openaudio.lock.json
          </CardTitle>
          <Button size="sm" variant="ghost" leftIcon={<RefreshCw className="h-4 w-4" />} onClick={() => void resolve.refetch()} loading={resolve.isFetching}>
            Re-resolve
          </Button>
        </CardHeader>
        <CardBody className="space-y-4">
          {resolve.isLoading || !lockfile ? (
            <Skeleton className="h-40" />
          ) : (
            <>
              <p className="text-caption text-fg-muted">
                {Object.keys(lockfile.packages).length} packages resolved (including transitive dependencies). Each entry pins an exact version and its manifest digest.
              </p>
              <ul className="grid gap-2 md:grid-cols-2">
                {Object.values(lockfile.packages).map((e) => (
                  <li key={e.id} className="rounded border border-border bg-surface-sunken px-3 py-2">
                    <p className="flex items-center justify-between gap-2 font-mono text-caption">
                      <span className="truncate">{e.id}</span>
                      <span className="text-fg">{e.version}</span>
                    </p>
                    <p className="flex items-center gap-2 font-mono text-caption text-fg-subtle">
                      {shortDigest(e.manifestDigest)}
                      {!libraryIds.has(e.id) && <Badge className="py-0">transitive</Badge>}
                    </p>
                  </li>
                ))}
              </ul>
              <details>
                <summary className="cursor-pointer text-caption text-fg-muted hover:text-fg">Raw JSON</summary>
                <pre className="mt-2 max-h-80 overflow-auto rounded border border-border bg-surface-sunken p-4 font-mono text-caption text-fg-muted scrollbar-thin">{JSON.stringify(lockfile, null, 2)}</pre>
              </details>
            </>
          )}
        </CardBody>
      </Card>
    </div>
  );
}
