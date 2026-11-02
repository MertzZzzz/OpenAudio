import { Link } from 'react-router-dom';
import { ArrowUpCircle, Pin, PinOff, Trash2 } from 'lucide-react';
import type { LibraryItem } from '@/shared/types/domain';
import { Badge } from '@/shared/ui/Badge';
import { Button } from '@/shared/ui/Button';
import { Select } from '@/shared/ui/Select';
import { Skeleton } from '@/shared/ui/Skeleton';
import { TD, TR } from '@/shared/ui/Table';
import { PackageCover } from '@/shared/ui/PackageCover';
import { KIND_META } from '@/shared/lib/kinds';
import { compare } from '@/shared/lib/semver';
import { packageHref } from '@/features/catalog/PackageCard';
import { usePackageById, useVersions } from '@/features/package-details/api';
import { usePinVersion, useRemoveFromLibrary, useUpdateItem } from './api';

export function LibraryRow({ item }: { item: LibraryItem }) {
  const pkgQ = usePackageById(item.packageId);
  const pkg = pkgQ.data;
  const versionsQ = useVersions(pkg?.publisher.slug ?? '', pkg?.slug ?? '');
  const pin = usePinVersion();
  const remove = useRemoveFromLibrary();
  const update = useUpdateItem();

  if (!pkg) {
    return (
      <TR>
        <TD colSpan={5}><Skeleton className="h-10" /></TD>
      </TR>
    );
  }

  const releases = versionsQ.data ?? [];
  const published = releases.filter((r) => r.status === 'published').map((r) => r.version);
  const pinnedYanked = item.pinnedVersion !== null && releases.some((r) => r.version === item.pinnedVersion && r.status === 'yanked');
  const pinOptions = [
    ...published.map((v) => ({ value: v, label: v })),
    ...(pinnedYanked && item.pinnedVersion ? [{ value: item.pinnedVersion, label: `${item.pinnedVersion} (yanked)` }] : []),
  ];
  const outdated = compare(pkg.latestVersion, item.installedVersion) > 0;

  return (
    <TR>
      <TD>
        <div className="flex items-center gap-3">
          <PackageCover hue={pkg.coverHue} kind={pkg.kind} className="h-10 w-10 shrink-0" />
          <div className="min-w-0">
            <Link to={packageHref(pkg)} className="block truncate font-medium hover:text-primary">{pkg.name}</Link>
            <span className="text-caption text-fg-subtle">{pkg.publisher.name} · {KIND_META[pkg.kind].label}</span>
          </div>
        </div>
      </TD>
      <TD className="font-mono">{item.installedVersion}</TD>
      <TD className="font-mono">
        {pkg.latestVersion}
        {outdated && <Badge tone="accent" className="ml-2">update</Badge>}
        {pinnedYanked && <Badge tone="danger" className="ml-2">pinned yanked</Badge>}
      </TD>
      <TD>
        <div className="flex w-40 items-center gap-1">
          <Select
            aria-label={`Pin version for ${pkg.name}`}
            value={item.pinnedVersion ?? ''}
            placeholder="Track latest"
            options={pinOptions}
            onChange={(e) => pin.mutate({ packageId: pkg.id, version: e.target.value || null })}
          />
          {item.pinnedVersion ? <Pin className="h-4 w-4 shrink-0 text-warning" aria-label="Pinned" /> : <PinOff className="h-4 w-4 shrink-0 text-fg-subtle" aria-label="Not pinned" />}
        </div>
      </TD>
      <TD className="text-right">
        <div className="flex justify-end gap-1">
          {outdated && !item.pinnedVersion && (
            <Button size="sm" variant="secondary" leftIcon={<ArrowUpCircle className="h-4 w-4" />} loading={update.isPending} onClick={() => update.mutate({ packageId: pkg.id, name: pkg.name })}>
              Update
            </Button>
          )}
          <Button size="icon" variant="ghost" aria-label={`Remove ${pkg.name} from library`} onClick={() => remove.mutate({ packageId: pkg.id, name: pkg.name })}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </TD>
    </TR>
  );
}
