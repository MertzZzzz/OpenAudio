import { forwardRef, useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react';
import { cn } from '@/shared/lib/cn';

const fieldBase =
  'w-full rounded border border-border bg-surface-sunken px-3 text-body text-fg placeholder:text-fg-subtle transition-colors hover:border-border-strong focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:opacity-50';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  icon?: ReactNode;
  invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input({ icon, invalid, className, ...rest }, ref) {
  return (
    <div className="relative w-full">
      {icon && <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-fg-subtle">{icon}</span>}
      <input
        ref={ref}
        aria-invalid={invalid || undefined}
        className={cn(fieldBase, 'h-9', icon && 'pl-9', invalid && 'border-danger focus:border-danger focus:ring-danger/30', className)}
        {...rest}
      />
    </div>
  );
});

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea({ invalid, className, ...rest }, ref) {
  return (
    <textarea
      ref={ref}
      aria-invalid={invalid || undefined}
      className={cn(fieldBase, 'min-h-[96px] py-2', invalid && 'border-danger', className)}
      {...rest}
    />
  );
});

export interface FieldProps {
  label: string;
  hint?: string;
  error?: string;
  children: (props: { id: string; 'aria-describedby'?: string; invalid: boolean }) => ReactNode;
  className?: string;
}

/** Accessible label + hint + error wrapper. */
export function Field({ label, hint, error, children, className }: FieldProps) {
  const id = useId();
  const descId = error || hint ? `${id}-desc` : undefined;
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <label htmlFor={id} className="text-caption font-medium text-fg-muted">
        {label}
      </label>
      {children({ id, 'aria-describedby': descId, invalid: Boolean(error) })}
      {(error || hint) && (
        <p id={descId} className={cn('text-caption', error ? 'text-danger' : 'text-fg-subtle')} role={error ? 'alert' : undefined}>
          {error ?? hint}
        </p>
      )}
    </div>
  );
}

export { fieldBase };
