import { useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { BadgeCheck, Check, Copy, Download, FileJson, Library, PackageX, Plus, Star } from 'lucide-react';
import { Button } from '@/shared/ui/Button';
import { Badge } from '@/shared/ui/Badge';
import { Select } from '@/shared/ui/Select';
import { Modal } from '@/shared/ui/Overlay';
import { Tabs, TabPanel } from '@/shared/ui/Tabs';
import { Skeleton } from '@/shared/ui/Skeleton';
import { EmptyState } from '@/shared/ui/EmptyState';
import { PackageCover } from '@/shared/ui/PackageCover';
import { PlayButton } from '@/shared/player/PlayButton';
import { useToast } from '@/shared/ui/Toast';
import { useCopy } from '@/shared/hooks/useCopy';
import { downloadJson } from '@/shared/lib/download';
import { formatBytes, formatCount, formatDate, formatKey, humanize } from '@/shared/lib/format';
import { termLabel } from '@/shared/lib/taxonomy';
import { KIND_META, MODULE_CLASS_META } from '@/shared/lib/kinds';
import { api } from '@/shared/api';
import { LicenseBadge } from '@/features/catalog/PackageCard';
import { packageTrack } from '@/features/catalog/toTrack';
import { useAddToLibrary, useLibrary } from '@/features/library/api';
import { usePackage, useVersions } from './api';
import { ChangelogTab, DependenciesTab, FilesTab, LicenseTab, OverviewTab, SamplesTab, VersionsTab } from './tabs';

type Tab = 'overview' | 'versions' | 'files' | 'samples' | 'dependencies' | 'changelog' | 'license';

function MetaRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <dt className="text-caption text-fg-subtle">{label}</dt>
      <dd className="text-right text-body">{children}</dd>
    </div>
  );
}

export function PackageDetailsView({ publisher, slug }: { publisher: string; slug: string }) {
  const pkgQ = usePackage(publisher, slug);
  const versionsQ = useVersions(publisher, slug);
  const library = useLibrary();
  const add = useAddToLibrary();
  const copy = useCopy();
  const { toast } = useToast();
  const [sp, setSp] = useSearchParams();
  const [tab, setTab] = useState<Tab>('overview');
  const [installOpen, setInstallOpen] = useState(false);

  const pkg = pkgQ.data;
  const published = useMemo(() => (versionsQ.data ?? []).filter((r) => r.status === 'published'), [versionsQ.data]);
  const version = sp.get('v') ?? pkg?.latestVersion ?? '';
  const inLibrary = library.data?.items.some((i) => i.packageId === pkg?.id) ?? false;

  if (pkgQ.isLoading) {
    return (
      <div className="space-y-6">
        <div className="flex gap-6">
          <Skeleton className="h-40 w-40" />
          <div className="flex-1 space-y-3">
            <Skeleton className="h-8 w-80" />
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-10 w-64" />
          </div>
        </div>
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (pkgQ.isError || !pkg) {
    return <EmptyState icon={<PackageX className="h-6 w-6" />} title="Package not found" description={`${publisher}/${slug} does not exist or was removed.`} action={<Link to="/search"><Button variant="secondary">Back to search</Button></Link>} />;
  }

  const installCmd = `openaudio add ${pkg.publisher.slug}/${pkg.slug}@${version}`;
  const tabs = [
    { value: 'overview' as const, label: 'Overview' },
    { value: 'versions' as const, label: 'Versions', count: published.length },
    { value: 'files' as const, label: 'Files' },
    { value: 'samples' as const, label: 'Samples', count: pkg.sampleCount || undefined },
    { value: 'dependencies' as const, label: 'Dependencies', count: pkg.dependencies.length || undefined },
    { value: 'changelog' as const, label: 'Changelog' },
    { value: 'license' as const, label: 'License' },
  ];

  const downloadManifest = async () => {
    const manifest = await api.getManifest(pkg.publisher.slug, pkg.slug, version);
    downloadJson(`${pkg.slug}-${version}.manifest.json`, manifest);
    toast({ tone: 'success', title: 'Manifest downloaded' });
  };

  return (
    <div className="space-y-8">
      <nav aria-label="Breadcrumb" className="text-caption text-fg-subtle">
        <Link to="/search" className="hover:text-fg">Packages</Link> / <Link to={`/search?q=${encodeURIComponent(pkg.publisher.name)}`} className="hover:text-fg">{pkg.publisher.slug}</Link> / <span className="text-fg-muted">{pkg.slug}</span>
      </nav>

      <header className="flex flex-col gap-6 md:flex-row">
        <div className="relative shrink-0">
          <PackageCover hue={pkg.coverHue} kind={pkg.kind} className="h-40 w-40 rounded-lg" />
          <div className="absolute bottom-2 right-2">
            <PlayButton track={packageTrack(pkg)} size="lg" />
          </div>
        </div>
        <div className="min-w-0 flex-1 space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone="primary">{pkg.module ? MODULE_CLASS_META[pkg.module.moduleClass].label : KIND_META[pkg.kind].label}</Badge>
            <LicenseBadge license={pkg.license} />
            {pkg.editorsPick && <Badge tone="accent"><Star className="h-3 w-3" aria-hidden /> Editors’ pick</Badge>}
          </div>
          <h1 className="text-display">{pkg.name}</h1>
          <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-fg-muted">
            <span className="flex items-center gap-1">
              by <span className="font-medium text-fg">{pkg.publisher.name}</span>
              {pkg.publisher.verified && <BadgeCheck className="h-4 w-4 text-accent" aria-label="Verified publisher" />}
            </span>
            <span aria-hidden>·</span>
            <span className="flex items-center gap-1"><Download className="h-4 w-4" aria-hidden />{formatCount(pkg.downloads)} downloads</span>
            <span aria-hidden>·</span>
            <span className="flex items-center gap-1"><Star className="h-4 w-4" aria-hidden />{pkg.rating.toFixed(1)}</span>
            <span aria-hidden>·</span>
            <span>Updated {formatDate(pkg.updatedAt)}</span>
          </p>
          <div className="flex flex-wrap items-center gap-2 pt-2">
            <div className="w-36">
              <Select
                aria-label="Version"
                value={version}
                onChange={(e) => {
                  const v = e.target.value;
                  setSp(v === pkg.latestVersion ? {} : { v }, { replace: true });
                }}
                options={published.map((r) => ({ value: r.version, label: `v${r.version}${r.version === pkg.latestVersion ? ' (latest)' : ''}` }))}
              />
            </div>
            <Button leftIcon={<Download className="h-4 w-4" />} onClick={() => setInstallOpen(true)}>
              Install
            </Button>
            <Button
              variant="secondary"
              loading={add.isPending}
              disabled={inLibrary}
              leftIcon={inLibrary ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
              onClick={() => add.mutate({ packageId: pkg.id, name: pkg.name, version: sp.get('v') })}
            >
              {inLibrary ? 'In library' : 'Add to library'}
            </Button>
            <Button variant="ghost" leftIcon={<FileJson className="h-4 w-4" />} onClick={() => void downloadManifest()}>
              Manifest
            </Button>
            <Button variant="ghost" leftIcon={<Copy className="h-4 w-4" />} onClick={() => void copy(installCmd, 'Install command copied')}>
              Copy install command
            </Button>
          </div>
        </div>
      </header>

      <div className="grid gap-8 lg:grid-cols-[1fr_280px]">
        <div className="min-w-0 space-y-6">
          <Tabs label="Package sections" items={tabs} value={tab} onChange={setTab} />
          <TabPanel value={tab}>
            {tab === 'overview' && <OverviewTab pkg={pkg} />}
            {tab === 'versions' && <VersionsTab releases={versionsQ.data ?? []} current={version} onSelect={(v) => setSp(v === pkg.latestVersion ? {} : { v })} />}
            {tab === 'files' && <FilesTab pkg={pkg} version={version} />}
            {tab === 'samples' && <SamplesTab pkg={pkg} />}
            {tab === 'dependencies' && <DependenciesTab pkg={pkg} />}
            {tab === 'changelog' && <ChangelogTab releases={versionsQ.data ?? []} />}
            {tab === 'license' && <LicenseTab pkg={pkg} />}
          </TabPanel>
        </div>

        <aside aria-label="Package metadata" className="h-fit rounded-lg border border-border bg-surface p-5">
          <h2 className="mb-2 text-caption font-semibold uppercase tracking-wide text-fg-subtle">Metadata</h2>
          <dl className="divide-y divide-border">
            <MetaRow label="Genre">
              <Link to={`/search?genre=${pkg.taxonomy.genres.primary}`} className="hover:text-primary">{termLabel('genre', pkg.taxonomy.genres.primary)}</Link>
              {pkg.taxonomy.genres.secondary.length > 0 && (
                <span className="block text-caption text-fg-subtle">{pkg.taxonomy.genres.secondary.map((g) => termLabel('genre', g)).join(', ')}</span>
              )}
            </MetaRow>
            {pkg.musical?.bpm && (
              <MetaRow label="BPM">
                <span className="font-mono">{pkg.musical.bpm.value}</span>
                {pkg.musical.bpm.range && <span className="block font-mono text-caption text-fg-subtle">{pkg.musical.bpm.range.join('–')}</span>}
              </MetaRow>
            )}
            {pkg.musical?.key && (
              <MetaRow label="Key">
                <span className="font-mono">{formatKey(pkg.musical.key)}</span>
                {pkg.musical.key.camelot && <span className="ml-1 font-mono text-caption text-fg-subtle">({pkg.musical.key.camelot})</span>}
              </MetaRow>
            )}
            {pkg.taxonomy.instruments.length > 0 && <MetaRow label="Instruments">{pkg.taxonomy.instruments.map((i) => termLabel('instrument', i)).join(', ')}</MetaRow>}
            {pkg.taxonomy.moods.length > 0 && (
              <MetaRow label="Mood">
                <span className="flex flex-wrap justify-end gap-1">{pkg.taxonomy.moods.map((m) => <Badge key={m}>{termLabel('mood', m)}</Badge>)}</span>
              </MetaRow>
            )}
            {pkg.taxonomy.energy !== undefined && (
              <MetaRow label="Energy">
                <span className="flex items-center gap-2">
                  <span className="h-1.5 w-20 overflow-hidden rounded-full bg-surface-raised" role="meter" aria-valuemin={0} aria-valuemax={1} aria-valuenow={pkg.taxonomy.energy} aria-label="Energy">
                    <span className="block h-full bg-gradient-to-r from-accent to-primary" style={{ width: `${pkg.taxonomy.energy * 100}%` }} />
                  </span>
                  <span className="font-mono text-caption">{pkg.taxonomy.energy.toFixed(2)}</span>
                </span>
              </MetaRow>
            )}
            <MetaRow label="Formats">{pkg.technical.formats.map((f) => f.toUpperCase()).join(', ') || '—'}</MetaRow>
            {pkg.technical.sampleRate && <MetaRow label="Quality"><span className="font-mono">{pkg.technical.sampleRate / 1000} kHz / {pkg.technical.bitDepth}-bit</span></MetaRow>}
            {pkg.taxonomy.useCases.length > 0 && <MetaRow label="Use cases">{pkg.taxonomy.useCases.map(humanize).join(', ')}</MetaRow>}
            <MetaRow label="Size">{formatBytes(pkg.sizeBytes)}</MetaRow>
            <MetaRow label="Package ID"><span className="break-all font-mono text-caption">{pkg.id}</span></MetaRow>
          </dl>
        </aside>
      </div>

      <Modal
        open={installOpen}
        onClose={() => setInstallOpen(false)}
        title={`Install ${pkg.name}`}
        description="Pin an exact version in your library or install via the CLI."
        footer={
          <>
            <Button variant="secondary" onClick={() => setInstallOpen(false)}>Close</Button>
            <Button
              leftIcon={<Library className="h-4 w-4" />}
              disabled={inLibrary}
              onClick={() => {
                add.mutate({ packageId: pkg.id, name: pkg.name, version });
                setInstallOpen(false);
              }}
            >
              {inLibrary ? 'Already in library' : `Add v${version} to library`}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <p className="mb-1.5 text-caption text-fg-muted">CLI</p>
            <div className="flex items-center gap-2 rounded border border-border bg-surface-sunken p-3 font-mono text-caption">
              <span className="text-fg-subtle">$</span>
              <span className="flex-1 break-all">{installCmd}</span>
              <button type="button" aria-label="Copy install command" onClick={() => void copy(installCmd, 'Install command copied')} className="text-fg-muted hover:text-fg">
                <Copy className="h-4 w-4" />
              </button>
            </div>
          </div>
          <p className="text-caption text-fg-subtle">
            Releases are immutable. The lockfile records version <span className="font-mono">{version}</span> together with its manifest digest so every machine resolves the same bytes.
          </p>
        </div>
      </Modal>
    </div>
  );
}
