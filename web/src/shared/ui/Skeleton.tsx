import type { HTMLAttributes } from 'react';
import { cn } from '@/shared/lib/cn';

export function Skeleton({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div aria-hidden className={cn('animate-pulse rounded bg-surface-raised', className)} {...rest} />;
}
