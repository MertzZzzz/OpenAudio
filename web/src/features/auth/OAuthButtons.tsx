import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { Button } from '@/shared/ui/Button';
import { Skeleton } from '@/shared/ui/Skeleton';
import { useAuth } from './AuthContext';
import type { OAuthProviderId } from './types';

function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-4 w-4 shrink-0" aria-hidden>
      <path fill="#EA4335" d="M12 10.2v3.9h5.5c-.2 1.3-1.6 3.9-5.5 3.9-3.3 0-6-2.7-6-6.1s2.7-6.1 6-6.1c1.9 0 3.1.8 3.8 1.5l2.6-2.5C16.8 3.3 14.6 2.4 12 2.4 6.7 2.4 2.4 6.7 2.4 12s4.3 9.6 9.6 9.6c5.5 0 9.2-3.9 9.2-9.4 0-.6-.1-1.1-.2-1.6H12z" />
      <path fill="#4285F4" d="M21.2 12.2c0-.6-.1-1.1-.2-1.6H12v3.9h5.5c-.3 1.4-1.1 2.5-2.3 3.3l3.1 2.4c1.8-1.7 2.9-4.2 2.9-8z" />
      <path fill="#FBBC05" d="M6.4 14.3a5.8 5.8 0 010-4.6L3.2 7.3a9.6 9.6 0 000 9.4l3.2-2.4z" />
      <path fill="#34A853" d="M12 21.6c2.6 0 4.8-.9 6.3-2.4l-3.1-2.4c-.8.6-1.9 1-3.2 1-2.5 0-4.6-1.7-5.4-3.9l-3.2 2.5c1.6 3.1 4.9 5.2 8.6 5.2z" />
    </svg>
  );
}

function DiscordIcon() {
  return (
    <svg viewBox="0 0 24 24" className="h-4 w-4 shrink-0" aria-hidden fill="#5865F2">
      <path d="M20.3 4.4A19.8 19.8 0 0015.4 3l-.6 1.3a18.4 18.4 0 00-5.6 0L8.6 3a19.7 19.7 0 00-4.9 1.5C.6 9 -.3 13.5.1 17.9a19.9 19.9 0 006 3l1.3-2.1a12.9 12.9 0 01-2-1l.5-.4a14.2 14.2 0 0012.2 0l.5.4c-.6.4-1.3.7-2 1l1.3 2.1a19.8 19.8 0 006-3c.5-5.1-.8-9.6-3.6-13.5zM8 15.2c-1.2 0-2.2-1.1-2.2-2.4S6.8 10.4 8 10.4s2.2 1.1 2.2 2.4-1 2.4-2.2 2.4zm8 0c-1.2 0-2.2-1.1-2.2-2.4s1-2.4 2.2-2.4 2.2 1.1 2.2 2.4-1 2.4-2.2 2.4z" />
    </svg>
  );
}

const META: Record<OAuthProviderId, { label: string; icon: JSX.Element }> = {
  google: { label: 'Google', icon: <GoogleIcon /> },
  discord: { label: 'Discord', icon: <DiscordIcon /> },
};

export function OAuthButtons({ returnTo, onError }: { returnTo: string; onError: (e: unknown) => void }) {
  const { api, startOAuth } = useAuth();
  const providers = useQuery({ queryKey: ['auth-providers', api.mode], queryFn: () => api.providers(), staleTime: Infinity });
  const [pending, setPending] = useState<OAuthProviderId | null>(null);

  if (providers.isLoading) return <Skeleton className="h-10" />;
  const list = providers.data?.oauth ?? [];
  if (list.length === 0) return null;

  return (
    <div className="grid gap-2">
      {list.map(({ id }) => (
        <Button
          key={id}
          variant="secondary"
          size="lg"
          leftIcon={META[id].icon}
          loading={pending === id}
          disabled={pending !== null && pending !== id}
          onClick={async () => {
            setPending(id);
            try {
              await startOAuth(id, returnTo);
            } catch (e) {
              onError(e);
            } finally {
              setPending(null);
            }
          }}
        >
          Continue with {META[id].label}
        </Button>
      ))}
    </div>
  );
}
