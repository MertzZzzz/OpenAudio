import { useId, useRef, type KeyboardEvent, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export interface TabItem<V extends string = string> {
  value: V;
  label: ReactNode;
  count?: number;
}

export interface TabsProps<V extends string> {
  items: readonly TabItem<V>[];
  value: V;
  onChange: (value: V) => void;
  label: string;
  className?: string;
}

/** WAI-ARIA tablist with roving tabindex and arrow-key navigation. Pair with <TabPanel>. */
export function Tabs<V extends string>({ items, value, onChange, label, className }: TabsProps<V>) {
  const baseId = useId();
  const refs = useRef<(HTMLButtonElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent, index: number) => {
    const last = items.length - 1;
    let next: number | null = null;
    if (e.key === 'ArrowRight') next = index === last ? 0 : index + 1;
    if (e.key === 'ArrowLeft') next = index === 0 ? last : index - 1;
    if (e.key === 'Home') next = 0;
    if (e.key === 'End') next = last;
    if (next === null) return;
    e.preventDefault();
    refs.current[next]?.focus();
    onChange(items[next]!.value);
  };

  return (
    <div role="tablist" aria-label={label} className={cn('flex gap-1 overflow-x-auto border-b border-border scrollbar-thin', className)}>
      {items.map((item, i) => {
        const selected = item.value === value;
        return (
          <button
            key={item.value}
            ref={(el) => {
              refs.current[i] = el;
            }}
            role="tab"
            type="button"
            id={`${baseId}-tab-${item.value}`}
            aria-selected={selected}
            aria-controls={`tabpanel-${item.value}`}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(item.value)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={cn(
              '-mb-px inline-flex items-center gap-2 whitespace-nowrap border-b-2 px-3 py-2.5 text-body font-medium transition-colors',
              selected ? 'border-primary text-fg' : 'border-transparent text-fg-muted hover:text-fg',
            )}
          >
            {item.label}
            {item.count !== undefined && <span className="rounded-full bg-surface-raised px-1.5 text-caption text-fg-subtle">{item.count}</span>}
          </button>
        );
      })}
    </div>
  );
}

export function TabPanel({ value, children, className }: { value: string; children: ReactNode; className?: string }) {
  return (
    <div role="tabpanel" id={`tabpanel-${value}`} tabIndex={0} className={cn('focus-visible:shadow-none', className)}>
      {children}
    </div>
  );
}
