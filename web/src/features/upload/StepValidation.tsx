import { AlertTriangle, CheckCircle2, Info, XCircle } from 'lucide-react';
import type { ValidationIssue } from '@/shared/types/domain';
import { Skeleton } from '@/shared/ui/Skeleton';
import { cn } from '@/shared/lib/cn';

const icon = {
  error: <XCircle className="h-4 w-4 text-danger" aria-hidden />,
  warning: <AlertTriangle className="h-4 w-4 text-warning" aria-hidden />,
  info: <Info className="h-4 w-4 text-accent" aria-hidden />,
};

export function StepValidation({ issues, loading }: { issues: ValidationIssue[] | undefined; loading: boolean }) {
  if (loading || !issues) {
    return (
      <div className="space-y-2" aria-busy="true">
        <Skeleton className="h-12" />
        <Skeleton className="h-12" />
      </div>
    );
  }
  const errors = issues.filter((i) => i.severity === 'error').length;
  const warnings = issues.filter((i) => i.severity === 'warning').length;
  return (
    <div className="space-y-4">
      <div className={cn('flex items-center gap-3 rounded-lg border p-4', errors ? 'border-danger/40 bg-danger/10' : 'border-success/40 bg-success/10')} role="status">
        {errors ? <XCircle className="h-6 w-6 text-danger" aria-hidden /> : <CheckCircle2 className="h-6 w-6 text-success" aria-hidden />}
        <div>
          <p className="font-semibold">{errors ? `${errors} error${errors > 1 ? 's' : ''} must be fixed` : 'Manifest is valid'}</p>
          <p className="text-caption text-fg-muted">
            Checked against <span className="font-mono">openaudio.package/v1</span>, module rules (§5.4) and metadata recommendations. {warnings} warning{warnings === 1 ? '' : 's'}.
          </p>
        </div>
      </div>
      {issues.length > 0 && (
        <ul className="divide-y divide-border rounded-lg border border-border" aria-label="Validation issues">
          {issues.map((i, n) => (
            <li key={n} className="flex items-start gap-3 px-4 py-3">
              {icon[i.severity]}
              <div>
                <p className="text-body">{i.message}</p>
                <p className="font-mono text-caption text-fg-subtle">{i.path}</p>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
