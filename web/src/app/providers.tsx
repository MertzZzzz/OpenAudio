import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { ToastProvider } from '@/shared/ui/Toast';
import { PlayerProvider } from '@/shared/player/PlayerContext';
import { AuthProvider } from '@/features/auth/AuthContext';
import type { AuthApi } from '@/features/auth/authApi';

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { staleTime: 30_000, refetchOnWindowFocus: false, retry: 1 },
    },
  });
}

export function Providers({ children, client, authApi }: { children: ReactNode; client?: QueryClient; authApi?: AuthApi }) {
  const [queryClient] = useState(() => client ?? createQueryClient());
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <AuthProvider api={authApi}>
          <PlayerProvider>{children}</PlayerProvider>
        </AuthProvider>
      </ToastProvider>
    </QueryClientProvider>
  );
}
