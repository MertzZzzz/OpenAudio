import type { PackageKind } from '@/shared/types/domain';
import { KIND_META } from '@/shared/lib/kinds';
import { cn } from '@/shared/lib/cn';

export interface PackageCoverProps {
  hue: number;
  kind: PackageKind;
  className?: string;
  iconClassName?: string;
}

/** Generated artwork until publishers can upload covers. */
export function PackageCover({ hue, kind, className, iconClassName }: PackageCoverProps) {
  const Icon = KIND_META[kind].icon;
  return (
    <div
      aria-hidden
      className={cn('relative flex items-center justify-center overflow-hidden rounded', className)}
      style={{
        background: `radial-gradient(120% 120% at 0% 0%, hsl(${hue} 85% 62% / .95), transparent 60%), radial-gradient(120% 120% at 100% 100%, hsl(${(hue + 60) % 360} 80% 45% / .9), transparent 55%), hsl(${hue} 35% 14%)`,
      }}
    >
      <Icon className={cn('h-1/3 w-1/3 text-white/85 drop-shadow', iconClassName)} strokeWidth={1.5} />
    </div>
  );
}
