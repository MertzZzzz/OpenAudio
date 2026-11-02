import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react';
import { CheckCircle2, AlertTriangle, Info, X, XCircle } from 'lucide-react';
import { cn } from '@/shared/lib/cn';

export type ToastTone = 'success' | 'error' | 'info' | 'warning';

interface ToastItem {
  id: number;
  tone: ToastTone;
  title: string;
  description?: string;
}

interface ToastApi {
  toast: (t: Omit<ToastItem, 'id'>) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

const icons: Record<ToastTone, ReactNode> = {
  success: <CheckCircle2 className="h-5 w-5 text-success" aria-hidden />,
  error: <XCircle className="h-5 w-5 text-danger" aria-hidden />,
  warning: <AlertTriangle className="h-5 w-5 text-warning" aria-hidden />,
  info: <Info className="h-5 w-5 text-accent" aria-hidden />,
};

export function ToastProvider({ children, durationMs = 4000 }: { children: ReactNode; durationMs?: number }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const next = useRef(1);

  const dismiss = useCallback((id: number) => setItems((xs) => xs.filter((x) => x.id !== id)), []);

  const toast = useCallback(
    (t: Omit<ToastItem, 'id'>) => {
      const id = next.current++;
      setItems((xs) => [...xs.slice(-3), { ...t, id }]);
      window.setTimeout(() => dismiss(id), durationMs);
    },
    [dismiss, durationMs],
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div aria-live="polite" aria-atomic="false" className="pointer-events-none fixed bottom-24 right-4 z-[60] flex w-80 flex-col gap-2">
        {items.map((t) => (
          <div
            key={t.id}
            role={t.tone === 'error' ? 'alert' : 'status'}
            className={cn('pointer-events-auto flex animate-toast-in items-start gap-3 rounded-lg border border-border bg-surface-raised p-4 shadow-elevated')}
          >
            {icons[t.tone]}
            <div className="flex-1">
              <p className="text-body font-medium text-fg">{t.title}</p>
              {t.description && <p className="mt-0.5 text-caption text-fg-muted">{t.description}</p>}
            </div>
            <button type="button" onClick={() => dismiss(t.id)} aria-label="Dismiss notification" className="text-fg-subtle hover:text-fg">
              <X className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within <ToastProvider>');
  return ctx;
}
