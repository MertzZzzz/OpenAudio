import { Fragment, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight, Download, FileClock, Package as PackageIcon, Plus, Rocket, Star, Upload } from 'lucide-react';
import type { Release } from '@/shared/types/domain';
import { Button, ButtonLink } from '@/shared/ui/Button';
import { Badge } from '@/shared/ui/Badge';
import { Card, CardHeader, CardTitle } from '@/shared/ui/Card';
import { Table, TBody, TD, TH, THead, TR } from '@/shared/ui/Table';
import { Skeleton } from '@/shared/ui/Skeleton';
import { EmptyState } from '@/shared/ui/EmptyState';
import { Modal } from '@/shared/ui/Overlay';
import { PackageCover } from '@/shared/ui/PackageCover';
import { KIND_META } from '@/shared/lib/kinds';
import { formatCount, formatDate, shortDigest } from '@/shared/lib/format';
import { cn } from '@/shared/lib/cn';
import { can, PublisherSwitcher, type PublisherContext } from '@/features/publisher/currentPublisher';
import { packageHref } from '@/features/catalog/PackageCard';
import { usePublisherPackages, useReleaseAction } from './api';
import { CreatePackageModal } from './CreatePackageModal';
import { MembersCard } from './MembersCard';

const statusTone = { published: 'success', draft: 'warning', yanked: 'danger' } as const;

function Stat({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <Card className="flex items-center gap-4 p-5">
      <div className="rounded-lg bg-primary/15 p-2.5 text-primary">{icon}</div>
      <div>
        <p className="text-caption text-fg-subtle">{label}</p>
        <p className="text-heading">{value}</p>
      </div>
    </Card>
  );
}

export function PublisherDashboardView({ ctx }: { ctx: PublisherContext }) {
  const publisher = ctx.publisher.slug;
  const role = ctx.publisher.role;
  const pkgs = usePublisherPackages(publisher);
  const action = useReleaseAction(publisher);
  const [createOpen, setCreateOpen] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [confirmYank, setConfirmYank] = useState<{ slug: string; release: Release } | null>(null);

  const data = pkgs.data ?? [];
  const drafts = data.flatMap((p) => p.releases.filter((r) => r.status === 'draft').map((r) => ({ p, r })));
  const totalDownloads = data.reduce((n, p) => n + p.downloads, 0);
  const avgRating = data.length ? data.reduce((n, p) => n + p.rating, 0) / data.filter((p) => p.rating > 0).length || 0 : 0;

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-caption text-fg-subtle">
            {ctx.publisher.name} · your role: <span className="font-medium text-fg-muted">{role}</span>
          </p>
          <h1 className="text-display">@{publisher}</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <PublisherSwitcher ctx={ctx} />
          {can(role, 'packages') && (
            <Button variant="secondary" leftIcon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)}>
              New package
            </Button>
          )}
          <ButtonLink to="/publisher/upload" leftIcon={<Upload className="h-4 w-4" />}>Upload release</ButtonLink>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat icon={<PackageIcon className="h-5 w-5" />} label="Packages" value={String(data.length)} />
        <Stat icon={<Download className="h-5 w-5" />} label="Downloads" value={formatCount(totalDownloads)} />
        <Stat icon={<Star className="h-5 w-5" />} label="Avg. rating" value={avgRating ? avgRating.toFixed(1) : '—'} />
        <Stat icon={<FileClock className="h-5 w-5" />} label="Pending drafts" value={String(drafts.length)} />
      </div>

      {drafts.length > 0 && (
        <Card className="border-warning/30">
          <CardHeader>
            <CardTitle>Drafts awaiting publication</CardTitle>
          </CardHeader>
          <ul className="divide-y divide-border">
            {drafts.map(({ p, r }) => (
              <li key={`${p.id}@${r.version}`} className="flex flex-wrap items-center gap-3 px-6 py-3">
                <Badge tone="warning">draft</Badge>
                <span className="font-medium">{p.name}</span>
                <span className="font-mono text-caption text-fg-muted">v{r.version}</span>
                <span className="flex-1 truncate text-caption text-fg-subtle">{r.changelog}</span>
                {can(role, 'publish') && (
                  <Button size="sm" leftIcon={<Rocket className="h-4 w-4" />} loading={action.isPending && action.variables?.version === r.version} onClick={() => action.mutate({ slug: p.slug, version: r.version, action: 'publish' })}>
                    Publish
                  </Button>
                )}
              </li>
            ))}
          </ul>
        </Card>
      )}

      <div className="grid gap-8 xl:grid-cols-[1fr_360px]">
        <section aria-labelledby="pkgs-heading" className="min-w-0 space-y-3">
          <h2 id="pkgs-heading" className="text-heading">Packages</h2>
          {pkgs.isLoading ? (
            <Skeleton className="h-80" />
          ) : data.length === 0 ? (
            <EmptyState
              icon={<PackageIcon className="h-6 w-6" />}
              title="No packages yet"
              description="Create your first package, then upload a release."
              action={can(role, 'packages') ? <Button onClick={() => setCreateOpen(true)}>New package</Button> : undefined}
            />
          ) : (
            <Table>
              <THead>
                <tr>
                  <TH className="w-8"><span className="sr-only">Expand</span></TH>
                  <TH>Package</TH>
                  <TH>Latest</TH>
                  <TH>Releases</TH>
                  <TH className="text-right">Downloads</TH>
                  <TH className="w-12"><span className="sr-only">Actions</span></TH>
                </tr>
              </THead>
              <TBody>
                {data.map((p) => {
                  const open = expanded === p.id;
                  const counts = p.releases.reduce<Record<string, number>>((acc, r) => ({ ...acc, [r.status]: (acc[r.status] ?? 0) + 1 }), {});
                  return (
                    <Fragment key={p.id}>
                      <TR>
                        <TD>
                          <button type="button" aria-expanded={open} aria-label={`${open ? 'Collapse' : 'Expand'} releases of ${p.name}`} onClick={() => setExpanded(open ? null : p.id)} className="rounded p-1 text-fg-subtle hover:text-fg">
                            <ChevronRight className={cn('h-4 w-4 transition-transform', open && 'rotate-90')} />
                          </button>
                        </TD>
                        <TD>
                          <div className="flex items-center gap-3">
                            <PackageCover hue={p.coverHue} kind={p.kind} className="h-9 w-9 shrink-0" />
                            <div className="min-w-0">
                              <Link to={packageHref(p)} className="block truncate font-medium hover:text-primary">{p.name}</Link>
                              <span className="text-caption text-fg-subtle">{KIND_META[p.kind].label}</span>
                            </div>
                          </div>
                        </TD>
                        <TD className="font-mono">{p.latestVersion === '0.0.0' ? '—' : p.latestVersion}</TD>
                        <TD>
                          <div className="flex gap-1">
                            {(['published', 'draft', 'yanked'] as const).map((s) => counts[s] ? <Badge key={s} tone={statusTone[s]}>{counts[s]} {s}</Badge> : null)}
                            {p.releases.length === 0 && <span className="text-caption text-fg-subtle">none</span>}
                          </div>
                        </TD>
                        <TD className="text-right font-mono">{formatCount(p.downloads)}</TD>
                        <TD className="text-right">
                          <Link to={`/publisher/upload?package=${p.slug}`} title="Upload new version" aria-label={`Upload new version of ${p.name}`} className="inline-flex rounded p-1.5 text-fg-muted hover:bg-surface-raised hover:text-primary">
                            <Upload className="h-4 w-4" />
                          </Link>
                        </TD>
                      </TR>
                      {open && (
                        <tr>
                          <td colSpan={6} className="bg-surface-sunken px-6 py-3">
                            <ul className="divide-y divide-border">
                              {p.releases.map((r) => (
                                <li key={r.version} className="flex flex-wrap items-center gap-3 py-2 text-body">
                                  <span className="w-16 font-mono font-medium">{r.version}</span>
                                  <Badge tone={statusTone[r.status]}>{r.status}</Badge>
                                  <span className="text-caption text-fg-subtle">{r.publishedAt ? formatDate(r.publishedAt) : 'not published'}</span>
                                  <span className="font-mono text-caption text-fg-subtle" title={r.manifestDigest}>{shortDigest(r.manifestDigest)}</span>
                                  <span className="ml-auto flex gap-2">
                                    {r.status === 'draft' && can(role, 'publish') && (
                                      <Button size="sm" onClick={() => action.mutate({ slug: p.slug, version: r.version, action: 'publish' })}>Publish</Button>
                                    )}
                                    {r.status === 'published' && can(role, 'publish') && (
                                      <Button size="sm" variant="danger" onClick={() => setConfirmYank({ slug: p.slug, release: r })}>Yank</Button>
                                    )}
                                  </span>
                                </li>
                              ))}
                              {p.releases.length === 0 && <li className="py-2 text-caption text-fg-subtle">No releases yet.</li>}
                            </ul>
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  );
                })}
              </TBody>
            </Table>
          )}
        </section>

        <MembersCard publisher={publisher} canManage={can(role, 'members')} />
      </div>

      <CreatePackageModal publisher={publisher} open={createOpen} onClose={() => setCreateOpen(false)} />

      <Modal
        open={confirmYank !== null}
        onClose={() => setConfirmYank(null)}
        title={`Yank ${confirmYank?.slug}@${confirmYank?.release.version}?`}
        description="Yanked releases stay downloadable for existing lockfiles but are never chosen by the resolver for new installs."
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmYank(null)}>Cancel</Button>
            <Button
              variant="danger"
              loading={action.isPending}
              onClick={async () => {
                if (confirmYank) await action.mutateAsync({ slug: confirmYank.slug, version: confirmYank.release.version, action: 'yank' });
                setConfirmYank(null);
              }}
            >
              Yank release
            </Button>
          </>
        }
      >
        <p className="text-body text-fg-muted">Releases are immutable — to fix a problem, publish a new version.</p>
      </Modal>
    </div>
  );
}
