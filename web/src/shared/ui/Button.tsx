import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { Loader2 } from 'lucide-react';
import { cn } from '@/shared/lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline';
export type ButtonSize = 'sm' | 'md' | 'lg' | 'icon';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  leftIcon?: ReactNode;
}

const variants: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-primary-fg hover:bg-primary/90 shadow-sm shadow-primary/20',
  secondary: 'bg-surface-raised text-fg hover:bg-border border border-border',
  outline: 'border border-border-strong text-fg hover:bg-surface-raised',
  ghost: 'text-fg-muted hover:text-fg hover:bg-surface-raised',
  danger: 'bg-danger/15 text-danger hover:bg-danger/25 border border-danger/30',
};

const sizes: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-caption gap-1.5',
  md: 'h-9 px-4 text-body gap-2',
  lg: 'h-11 px-6 text-base gap-2',
  icon: 'h-9 w-9 justify-center',
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = 'primary', size = 'md', loading = false, leftIcon, className, children, disabled, type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        'inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded font-medium transition-colors',
        'disabled:pointer-events-none disabled:opacity-50',
        variants[variant],
        sizes[size],
        className,
      )}
      {...rest}
    >
      {loading ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : leftIcon}
      {children}
    </button>
  );
});
