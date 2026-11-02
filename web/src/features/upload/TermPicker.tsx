import { Check } from 'lucide-react';
import { cn } from '@/shared/lib/cn';

export interface TermOption {
  value: string;
  label: string;
  depth?: number;
}

export interface TermPickerProps {
  label: string;
  options: TermOption[];
  value: string[];
  onChange: (value: string[]) => void;
  max?: number;
  error?: string;
  hint?: string;
}

/** Multi-select chip group for controlled vocabularies. */
export function TermPicker({ label, options, value, onChange, max, error, hint }: TermPickerProps) {
  const toggle = (v: string) => {
    if (value.includes(v)) onChange(value.filter((x) => x !== v));
    else if (!max || value.length < max) onChange([...value, v]);
  };
  const id = label.toLowerCase().replace(/[^a-z]+/g, '-');
  return (
    <fieldset className="space-y-2" aria-describedby={`${id}-desc`}>
      <legend className="flex w-full items-center justify-between text-caption font-medium text-fg-muted">
        {label}
        {max && <span className="text-fg-subtle">{value.length}/{max}</span>}
      </legend>
      <div className="flex flex-wrap gap-1.5">
        {options.map((o) => {
          const on = value.includes(o.value);
          const disabled = !on && max !== undefined && value.length >= max;
          return (
            <button
              key={o.value}
              type="button"
              aria-pressed={on}
              disabled={disabled}
              onClick={() => toggle(o.value)}
              className={cn(
                'inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-caption transition-colors disabled:opacity-40',
                on ? 'border-primary bg-primary/15 text-fg' : 'border-border text-fg-muted hover:text-fg',
                o.depth ? 'border-dashed' : '',
              )}
            >
              {on && <Check className="h-3 w-3" aria-hidden />}
              {o.label}
            </button>
          );
        })}
      </div>
      <p id={`${id}-desc`} className={cn('text-caption', error ? 'text-danger' : 'text-fg-subtle')} role={error ? 'alert' : undefined}>
        {error ?? hint}
      </p>
    </fieldset>
  );
}
