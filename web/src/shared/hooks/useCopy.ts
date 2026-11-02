import { useCallback } from 'react';
import { useToast } from '@/shared/ui/Toast';

export function useCopy() {
  const { toast } = useToast();
  return useCallback(
    async (text: string, label = 'Copied to clipboard') => {
      try {
        await navigator.clipboard.writeText(text);
        toast({ tone: 'success', title: label });
      } catch {
        toast({ tone: 'error', title: 'Clipboard unavailable', description: text });
      }
    },
    [toast],
  );
}
