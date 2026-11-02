import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode;
  trailing?: ReactNode;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(function Checkbox({ label, trailing, className, ...rest }, ref) {
  return (
    <label className={cn('flex cursor-pointer items-center gap-2 rounded px-1 py-1 text-body text-fg-muted hover:text-fg', className)}>
      <input ref={ref} type="checkbox" className="h-4 w-4 cursor-pointer rounded border-border-strong bg-surface-sunken accent-[rgb(var(--oa-primary))]" {...rest} />
      <span className="flex-1 truncate">{label}</span>
      {trailing && <span className="text-caption text-fg-subtle">{trailing}</span>}
    </label>
  );
});
