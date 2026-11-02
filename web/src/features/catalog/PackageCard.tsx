import { Link } from 'react-router-dom';
import { BadgeCheck, Download } from 'lucide-react';
import type { Package } from '@/shared/types/domain';
import { Badge } from '@/shared/ui/Badge';
import { PackageCover } from '@/shared/ui/PackageCover';
import { PlayButton } from '@/shared/player/PlayButton';
import { KIND_META, MODULE_CLASS_META } from '@/shared/lib/kinds';
import { formatCount, formatKey } from '@/shared/lib/format';
import { termLabel } from '@/shared/lib/taxonomy';
import { cn } from '@/shared/lib/cn';
import { packageTrack } from './toTrack';

export function packageHref(p: Pick<Package, 'publisher' | 'slug'>): string {
  return `/packages/${p.publisher.slug}/${p.slug}`;
}

export function LicenseBadge({ license }: { license: Package['license'] }) {
  const free = license.content.startsWith('CC0') || license.content === 'MIT';
  return (
    <Badge tone={license.commercialUse ? 'success' : 'warning'} title={license.content}>
      {free ? 'Free' : license.commercialUse ? 'Free · attribution' : 'Non-commercial'}
    </Badge>
  );
}

export function PackageCard({ pkg, className }: { pkg: Package; className?: string }) {
  const kind = KIND_META[pkg.kind];
  return (
    <article className={cn('group relative flex flex-col rounded-lg border border-border bg-surface p-3 transition-colors hover:border-border-strong hover:bg-surface-raised/50', className)}>
      <div className="relative">
        <PackageCover hue={pkg.coverHue} kind={pkg.kind} className="aspect-square w-full" />
        <div className="absolute bottom-2 right-2 opacity-100 transition-opacity sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100">
          <PlayButton track={packageTrack(pkg)} />
        </div>
      </div>
      <div className="mt-3 flex min-w-0 flex-1 flex-col gap-1">
        <h3 className="truncate font-semibold text-fg">
          <Link to={packageHref(pkg)} className="after:absolute after:inset-0 after:content-[''] focus-visible:shadow-none">
            {pkg.name}
          </Link>
        </h3>
        <p className="flex items-center gap-1 truncate text-caption text-fg-muted">
          {pkg.publisher.name}
          {pkg.publisher.verified && <BadgeCheck className="h-3.5 w-3.5 text-accent" aria-label="Verified publisher" />}
        </p>
        <div className="mt-1 flex flex-wrap gap-1">
          <Badge tone="primary">{pkg.module ? MODULE_CLASS_META[pkg.module.moduleClass].label : kind.label}</Badge>
          <Badge>{termLabel('genre', pkg.taxonomy.genres.primary)}</Badge>
        </div>
        <div className="mt-auto space-y-2 pt-2 text-caption text-fg-subtle">
          <p className="truncate font-mono">
            {pkg.musical?.bpm ? `${pkg.musical.bpm.value} BPM` : `v${pkg.latestVersion}`}
            {pkg.musical?.key ? ` · ${formatKey(pkg.musical.key)}` : ''}
          </p>
          <div className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-1" title={`${pkg.downloads} downloads`}>
              <Download className="h-3 w-3" aria-hidden />
              {formatCount(pkg.downloads)}
            </span>
            <LicenseBadge license={pkg.license} />
          </div>
        </div>
      </div>
    </article>
  );
}

export function PackageRow({ pkg }: { pkg: Package }) {
  return (
    <article className="group relative flex items-center gap-4 rounded-lg border border-border bg-surface p-3 hover:border-border-strong">
      <PackageCover hue={pkg.coverHue} kind={pkg.kind} className="h-16 w-16 shrink-0" />
      <div className="min-w-0 flex-1">
        <h3 className="truncate font-semibold">
          <Link to={packageHref(pkg)} className="after:absolute after:inset-0 after:content-[''] focus-visible:shadow-none">
            {pkg.name}
          </Link>
        </h3>
        <p className="truncate text-caption text-fg-muted">
          {pkg.publisher.name} · v{pkg.latestVersion} · {KIND_META[pkg.kind].label}
        </p>
        <p className="mt-1 line-clamp-1 text-caption text-fg-subtle">{pkg.description}</p>
      </div>
      <div className="hidden flex-wrap justify-end gap-1 md:flex">
        {pkg.taxonomy.moods.slice(0, 2).map((m) => (
          <Badge key={m}>{termLabel('mood', m)}</Badge>
        ))}
        <LicenseBadge license={pkg.license} />
      </div>
      <span className="hidden w-24 text-right font-mono text-caption text-fg-subtle lg:block">
        {pkg.musical?.bpm ? `${pkg.musical.bpm.value} BPM` : '—'}
      </span>
      <div className="relative z-10">
        <PlayButton track={packageTrack(pkg)} size="sm" />
      </div>
    </article>
  );
}
