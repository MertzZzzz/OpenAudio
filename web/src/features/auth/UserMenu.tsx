import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { LayoutDashboard, LogOut, User as UserIcon } from 'lucide-react';
import { ButtonLink } from '@/shared/ui/Button';
import { cn } from '@/shared/lib/cn';
import { useAuth } from './AuthContext';
import { hasRole, type AuthUser } from './types';

export function UserAvatar({ user, size = 'sm' }: { user: Pick<AuthUser, 'displayName' | 'avatarUrl'>; size?: 'sm' | 'lg' }) {
  const cls = size === 'lg' ? 'h-16 w-16 text-xl' : 'h-8 w-8 text-caption';
  if (user.avatarUrl) return <img src={user.avatarUrl} alt="" className={cn('shrink-0 rounded-full object-cover', cls)} referrerPolicy="no-referrer" />;
  const initials = user.displayName
    .split(/\s+/)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();
  return (
    <span aria-hidden className={cn('flex shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-primary to-accent font-bold text-white', cls)}>
      {initials}
    </span>
  );
}

/** Top-bar account control: "Sign in" or an avatar menu (keyboard accessible). */
export function UserMenu() {
  const { status, user, logout } = useAuth();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const location = useLocation();

  useEffect(() => setOpen(false), [location.pathname]);
  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDoc);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  if (status === 'loading') return <span className="h-8 w-8 animate-pulse rounded-full bg-surface-raised" aria-hidden />;
  if (!user) {
    const returnTo = location.pathname.startsWith('/login') ? '/' : location.pathname + location.search;
    return (
      <ButtonLink to={`/login?returnTo=${encodeURIComponent(returnTo)}`} size="sm">Sign in</ButtonLink>
    );
  }

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Account menu for ${user.displayName}`}
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-2 rounded-full p-0.5 hover:bg-surface-raised"
      >
        <span className="hidden pl-2 text-caption text-fg-muted md:inline">{user.displayName}</span>
        <UserAvatar user={user} />
      </button>
      {open && (
        <div role="menu" className="absolute right-0 mt-2 w-56 animate-scale-in rounded-lg border border-border bg-surface-raised p-1 shadow-elevated">
          <div className="px-3 py-2">
            <p className="truncate text-body font-medium">{user.displayName}</p>
            <p className="truncate text-caption text-fg-subtle">{user.email}</p>
          </div>
          <div className="my-1 h-px bg-border" />
          <Link role="menuitem" to="/account" className="flex items-center gap-2 rounded px-3 py-2 text-body text-fg-muted hover:bg-surface hover:text-fg">
            <UserIcon className="h-4 w-4" aria-hidden /> Account
          </Link>
          {hasRole(user, 'publisher') && (
            <Link role="menuitem" to="/publisher" className="flex items-center gap-2 rounded px-3 py-2 text-body text-fg-muted hover:bg-surface hover:text-fg">
              <LayoutDashboard className="h-4 w-4" aria-hidden /> Publisher dashboard
            </Link>
          )}
          <button
            role="menuitem"
            type="button"
            onClick={() => void logout()}
            className="flex w-full items-center gap-2 rounded px-3 py-2 text-body text-fg-muted hover:bg-surface hover:text-danger"
          >
            <LogOut className="h-4 w-4" aria-hidden /> Sign out
          </button>
        </div>
      )}
    </div>
  );
}
