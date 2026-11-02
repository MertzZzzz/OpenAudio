import { forwardRef, type SelectHTMLAttributes } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '@/shared/lib/cn';
import { fieldBase } from './Input';

export interface SelectOption<V extends string = string> {
  value: V;
  label: string;
}

export interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'children'> {
  options: readonly SelectOption[];
  placeholder?: string;
  invalid?: boolean;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  { options, placeholder, invalid, className, ...rest },
  ref,
) {
  return (
    <div className="relative w-full">
      <select
        ref={ref}
        aria-invalid={invalid || undefined}
        className={cn(fieldBase, 'h-9 cursor-pointer appearance-none pr-9', invalid && 'border-danger', className)}
        {...rest}
      >
        {placeholder !== undefined && <option value="">{placeholder}</option>}
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-fg-subtle" aria-hidden />
    </div>
  );
});
