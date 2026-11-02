import { Link } from 'react-router-dom';
import { FileJson, FileAudio, Binary, Ban } from 'lucide-react';
import type { Package, Release } from '@/shared/types/domain';
import { Badge } from '@/shared/ui/Badge';
import { Table, TBody, TD, TH, THead, TR } from '@/shared/ui/Table';
import { Skeleton } from '@/shared/ui/Skeleton';
import { EmptyState } from '@/shared/ui/EmptyState';
import { formatBytes, formatDate, shortDigest } from '@/shared/lib/format';
import { useToast } from '@/shared/ui/Toast';
import { SampleTable } from '@/features/sample-browser/SampleTable';
import { useSamples } from '@/features/sample-browser/api';
import { packageHref } from '@/features/catalog/PackageCard';
import { useArtifacts, usePackageById } from './api';
import { ModuleDiagram } from './ModuleDiagram';

export function OverviewTab({ pkg }: { pkg: Package }) {
  return (
    <div className="space-y-6">
      <p className="max-w-3xl leading-relaxed text-fg-muted">{pkg.description}</p>
      {pkg.module && (
        <section aria-labelledby="module-heading" className="space-y-3">
          <h3 id="module-heading" className="font-semibold">Module definition</h3>
          <ModuleDiagram module={pkg.module} />
          <p className="text-caption text-fg-subtle">OpenAudio distributes and describes modules; hosting (CLAP / VST3 / AU) is a future phase.</p>
        </section>
      )}
      {pkg.tags.length > 0 && (
        <section aria-labelledby="tags-heading">
          <h3 id="tags-heading" className="mb-2 font-semibold">Tags</h3>
          <div className="flex flex-wrap gap-1">
            {pkg.tags.map((t) => (
              <Link key={t} to={`/search?q=${encodeURIComponent(t)}`}>
                <Badge className="hover:border-border-strong hover:text-fg">#{t}</Badge>
              </Link>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

const statusTone = { published: 'success', draft: 'warning', yanked: 'danger' } as const;

export function VersionsTab({ releases, current, onSelect }: { releases: Release[]; current: string; onSelect: (v: string) => void }) {
  return (
    <Table>
      <THead>
        <tr>
          <TH>Version</TH>
          <TH>Status</TH>
          <TH>Published</TH>
          <TH>Manifest digest</TH>
          <TH><span className="sr-only">Actions</span></TH>
        </tr>
      </THead>
      <TBody>
        {releases.filter((r) => r.status !== 'draft').map((r) => (
          <TR key={r.version}>
            <TD className="font-mono font-medium">
              {r.version}
              {r.version === current && <Badge tone="primary" className="ml-2">viewing</Badge>}
            </TD>
            <TD><Badge tone={statusTone[r.status]}>{r.status}</Badge></TD>
            <TD className="text-fg-muted">{r.publishedAt ? formatDate(r.publishedAt) : '—'}</TD>
            <TD className="font-mono text-caption text-fg-subtle" title={r.manifestDigest}>{shortDigest(r.manifestDigest)}</TD>
            <TD className="text-right">
              {r.version !== current && r.status === 'published' && (
                <button type="button" className="text-caption text-primary hover:underline" onClick={() => onSelect(r.version)}>
                  View
                </button>
              )}
            </TD>
          </TR>
        ))}
      </TBody>
    </Table>
  );
}

export function FilesTab({ pkg, version }: { pkg: Package; version: string }) {
  const { data, isLoading } = useArtifacts(pkg.publisher.slug, pkg.slug, version);
  const { toast } = useToast();
  if (isLoading) return <Skeleton className="h-64" />;
  const icon = (k: string) => (k === 'manifest' ? <FileJson className="h-4 w-4 text-warning" /> : k === 'binary' ? <Binary className="h-4 w-4 text-primary" /> : <FileAudio className="h-4 w-4 text-accent" />);
  const total = (data ?? []).reduce((n, a) => n + a.size, 0);
  return (
    <div className="space-y-3">
      <p className="text-caption text-fg-muted">
        {data?.length} artifacts · {formatBytes(total)} · every blob is stored at <code className="font-mono">blobs/&lt;sha256&gt;</code> and served through short-lived presigned URLs.
      </p>
      <Table>
        <THead>
          <tr>
            <TH>Path</TH>
            <TH>Kind</TH>
            <TH className="text-right">Size</TH>
            <TH>Digest</TH>
          </tr>
        </THead>
        <TBody>
          {data?.map((a) => (
            <TR key={a.path}>
              <TD>
                <span className="flex items-center gap-2 font-mono text-caption">{icon(a.kind)}{a.path}</span>
              </TD>
              <TD><Badge>{a.kind}</Badge></TD>
              <TD className="text-right font-mono text-caption">{formatBytes(a.size)}</TD>
              <TD>
                <button
                  type="button"
                  className="font-mono text-caption text-fg-subtle hover:text-fg"
                  title="Copy digest"
                  onClick={() => {
                    void navigator.clipboard?.writeText(a.digest);
                    toast({ tone: 'success', title: 'Digest copied' });
                  }}
                >
                  {shortDigest(a.digest)}
                </button>
              </TD>
            </TR>
          ))}
        </TBody>
      </Table>
    </div>
  );
}

export function SamplesTab({ pkg }: { pkg: Package }) {
  const { data, isLoading } = useSamples({ packageId: pkg.id });
  const { toast } = useToast();
  if (isLoading) return <Skeleton className="h-64" />;
  if (!data?.length) return <EmptyState icon={<FileAudio className="h-6 w-6" />} title="No samples in this package" description="Module packages ship binaries and presets instead of audio samples." />;
  return (
    <div className="space-y-3">
      <p className="text-caption text-fg-muted">
        Showing {data.length} of {pkg.sampleCount} samples. <Link to={`/samples?package=${encodeURIComponent(pkg.id)}`} className="text-primary hover:underline">Open in sample browser →</Link>
      </p>
      <SampleTable samples={data} onDownload={(s) => toast({ tone: 'info', title: `Download queued: ${s.name}`, description: 'Presigned URLs arrive with the backend.' })} />
    </div>
  );
}

function DependencyRow({ id, range }: { id: string; range: string }) {
  const { data } = usePackageById(id);
  return (
    <TR>
      <TD>
        {data ? (
          <Link to={packageHref(data)} className="font-medium hover:text-primary">{data.name}</Link>
        ) : (
          <Skeleton className="h-4 w-32" />
        )}
        <p className="font-mono text-caption text-fg-subtle">{id}</p>
      </TD>
      <TD className="font-mono">{range}</TD>
      <TD className="font-mono text-fg-muted">{data?.latestVersion ?? '…'}</TD>
    </TR>
  );
}

export function DependenciesTab({ pkg }: { pkg: Package }) {
  if (!pkg.dependencies.length) return <EmptyState icon={<Ban className="h-6 w-6" />} title="No dependencies" description="This package is self-contained." />;
  return (
    <Table>
      <THead>
        <tr>
          <TH>Package</TH>
          <TH>Range</TH>
          <TH>Latest</TH>
        </tr>
      </THead>
      <TBody>
        {pkg.dependencies.map((d) => (
          <DependencyRow key={d.packageId} id={d.packageId} range={d.range} />
        ))}
      </TBody>
    </Table>
  );
}

export function ChangelogTab({ releases }: { releases: Release[] }) {
  return (
    <ol className="relative space-y-6 border-l border-border pl-6">
      {releases.filter((r) => r.status !== 'draft').map((r) => (
        <li key={r.version}>
          <span className="absolute -left-[5px] mt-1.5 h-2.5 w-2.5 rounded-full bg-primary" aria-hidden />
          <h4 className="flex items-center gap-2 font-mono font-semibold">
            {r.version}
            {r.status === 'yanked' && <Badge tone="danger">yanked</Badge>}
          </h4>
          <p className="text-caption text-fg-subtle">{r.publishedAt ? formatDate(r.publishedAt) : ''}</p>
          <p className="mt-1 text-fg-muted">{r.changelog}</p>
        </li>
      ))}
    </ol>
  );
}

export function LicenseTab({ pkg }: { pkg: Package }) {
  const l = pkg.license;
  return (
    <div className="max-w-2xl space-y-4">
      <div className="flex items-center gap-3">
        <span className="font-mono text-lg font-semibold">{l.content}</span>
        <a href={`https://spdx.org/licenses/${l.content}.html`} target="_blank" rel="noreferrer" className="text-caption text-primary hover:underline">
          SPDX ↗
        </a>
      </div>
      <dl className="grid grid-cols-2 gap-4">
        <div className="rounded-lg border border-border p-4">
          <dt className="text-caption text-fg-subtle">Commercial use</dt>
          <dd className={l.commercialUse ? 'mt-1 font-semibold text-success' : 'mt-1 font-semibold text-warning'}>{l.commercialUse ? 'Allowed' : 'Not allowed'}</dd>
        </div>
        <div className="rounded-lg border border-border p-4">
          <dt className="text-caption text-fg-subtle">Attribution</dt>
          <dd className="mt-1 font-semibold">{l.attributionRequired ? 'Required' : 'Not required'}</dd>
        </div>
      </dl>
      {l.attributionRequired && (
        <div className="rounded-lg border border-border bg-surface-sunken p-4 font-mono text-caption text-fg-muted">
          “{pkg.name}” by {pkg.publisher.name}, licensed under {l.content}.
        </div>
      )}
    </div>
  );
}
