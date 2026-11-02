import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { ShieldAlert } from 'lucide-react';
import { ButtonLink } from '@/shared/ui/Button';
import { EmptyState } from '@/shared/ui/EmptyState';
import { Skeleton } from '@/shared/ui/Skeleton';
import { useAuth } from './AuthContext';
import { hasRole, type Role } from './types';

/** Route guard: redirects anonymous visitors to /login and checks the platform role. */
export function RequireAuth({ children, role }: { children: ReactNode; role?: Role }) {
  const { status, user, signedOut } = useAuth();
  const location = useLocation();

  if (status === 'loading') {
    return (
      <div className="space-y-4" aria-busy="true" aria-label="Checking session">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-64" />
      </div>
    );
  }
  if (status === 'anonymous') {
    if (signedOut) return <Navigate to="/" replace />;
    const returnTo = location.pathname + location.search;
    return <Navigate to={`/login?returnTo=${encodeURIComponent(returnTo)}`} replace />;
  }
  if (role && !hasRole(user, role)) {
    return (
      <EmptyState
        icon={<ShieldAlert className="h-6 w-6" />}
        title={`${role[0]!.toUpperCase()}${role.slice(1)} access required`}
        description="Your account does not have permission to open this page. Ask an admin to grant you the publisher role."
        action={
          <ButtonLink to="/" variant="secondary">Back to Discover</ButtonLink>
        }
      />
    );
  }
  return <>{children}</>;
}
