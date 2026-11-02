import type { ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn('flex flex-col items-center justify-center rounded-lg border border-dashed border-border px-6 py-12 text-center', className)}>
      {icon && <div className="mb-3 rounded-full bg-surface-raised p-3 text-fg-muted">{icon}</div>}
      <p className="font-semibold text-fg">{title}</p>
      {description && <p className="mt-1 max-w-sm text-body text-fg-muted">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
