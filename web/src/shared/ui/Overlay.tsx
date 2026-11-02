import { useEffect, useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from '@/shared/lib/cn';

const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

/** Escape to close, focus trap, focus restore and body scroll lock. */
function useDialogBehaviour(open: boolean, onClose: () => void) {
  const panelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    const first = panel?.querySelector<HTMLElement>(FOCUSABLE);
    (first ?? panel)?.focus();
    const { overflow } = document.body.style;
    document.body.style.overflow = 'hidden';

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
      if (e.key === 'Tab' && panel) {
        const nodes = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)];
        if (!nodes.length) return;
        const firstEl = nodes[0]!;
        const lastEl = nodes[nodes.length - 1]!;
        if (e.shiftKey && document.activeElement === firstEl) {
          e.preventDefault();
          lastEl.focus();
        } else if (!e.shiftKey && document.activeElement === lastEl) {
          e.preventDefault();
          firstEl.focus();
        }
      }
    };
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.body.style.overflow = overflow;
      previous?.focus();
    };
  }, [open, onClose]);
  return panelRef;
}

interface OverlayProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
  className?: string;
}

export function Modal({ open, onClose, title, description, children, footer, className }: OverlayProps) {
  const panelRef = useDialogBehaviour(open, onClose);
  const id = useId();
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 animate-fade-in bg-black/60 backdrop-blur-sm" onClick={onClose} aria-hidden />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={`${id}-title`}
        aria-describedby={description ? `${id}-desc` : undefined}
        tabIndex={-1}
        className={cn('relative flex max-h-[90vh] w-full max-w-lg animate-scale-in flex-col rounded-xl border border-border bg-surface shadow-elevated', className)}
      >
        <div className="flex items-start justify-between gap-4 border-b border-border px-6 py-4">
          <div>
            <h2 id={`${id}-title`} className="text-heading">{title}</h2>
            {description && <p id={`${id}-desc`} className="mt-1 text-body text-fg-muted">{description}</p>}
          </div>
          <button type="button" onClick={onClose} aria-label="Close dialog" className="rounded p-1 text-fg-muted hover:bg-surface-raised hover:text-fg">
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="overflow-y-auto px-6 py-5 scrollbar-thin">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-border px-6 py-4">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

export function Drawer({ open, onClose, title, description, children, footer, className }: OverlayProps) {
  const panelRef = useDialogBehaviour(open, onClose);
  const id = useId();
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 animate-fade-in bg-black/60" onClick={onClose} aria-hidden />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={`${id}-title`}
        tabIndex={-1}
        className={cn('absolute right-0 top-0 flex h-full w-full max-w-md animate-slide-in-right flex-col border-l border-border bg-surface shadow-elevated', className)}
      >
        <div className="flex items-start justify-between gap-4 border-b border-border px-6 py-4">
          <div>
            <h2 id={`${id}-title`} className="text-heading">{title}</h2>
            {description && <p className="mt-1 text-body text-fg-muted">{description}</p>}
          </div>
          <button type="button" onClick={onClose} aria-label="Close panel" className="rounded p-1 text-fg-muted hover:bg-surface-raised hover:text-fg">
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-6 py-5 scrollbar-thin">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-border px-6 py-4">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}
