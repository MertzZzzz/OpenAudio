import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { ToastProvider } from '@/shared/ui/Toast';
import { PlayerProvider } from '@/shared/player/PlayerContext';

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: 30_000, refetchOnWindowFocus: false, retry: 1 },
    },
  });
}

export function Providers({ children, client }: { children: ReactNode; client?: QueryClient }) {
  const [queryClient] = useState(() => client ?? createQueryClient());
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <PlayerProvider>{children}</PlayerProvider>
      </ToastProvider>
    </QueryClientProvider>
  );
}
