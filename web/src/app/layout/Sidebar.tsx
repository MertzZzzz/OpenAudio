import { NavLink, Link } from 'react-router-dom';
import { cn } from '@/shared/lib/cn';
import { KIND_META } from '@/shared/lib/kinds';
import type { PackageKind } from '@/shared/types/domain';
import { useAuth } from '@/features/auth/AuthContext';
import { hasRole } from '@/features/auth/types';
import { primaryNav, publisherNav, type NavItem } from './nav';

const browseKinds: PackageKind[] = ['instrument', 'effect', 'sample-pack', 'preset-pack', 'midi-tool', 'utility'];

function NavGroup({ title, items, onNavigate }: { title?: string; items: NavItem[]; onNavigate?: () => void }) {
  return (
    <div>
      {title && <p className="mb-1 px-3 text-caption font-semibold uppercase tracking-wider text-fg-subtle">{title}</p>}
      <ul className="space-y-0.5">
        {items.map(({ to, label, icon: Icon, end }) => (
          <li key={to}>
            <NavLink
              to={to}
              end={end}
              onClick={onNavigate}
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded px-3 py-2 text-body font-medium transition-colors',
                  isActive ? 'bg-primary/15 text-fg' : 'text-fg-muted hover:bg-surface-raised hover:text-fg',
                )
              }
            >
              <Icon className="h-4 w-4" aria-hidden />
              {label}
            </NavLink>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function SidebarContent({ onNavigate }: { onNavigate?: () => void }) {
  const { user } = useAuth();
  return (
    <nav aria-label="Main" className="flex flex-col gap-6 p-4">
      <NavGroup items={primaryNav} onNavigate={onNavigate} />
      <div>
        <p className="mb-1 px-3 text-caption font-semibold uppercase tracking-wider text-fg-subtle">Browse</p>
        <ul className="space-y-0.5">
          {browseKinds.map((k) => {
            const { icon: Icon, plural } = KIND_META[k];
            return (
              <li key={k}>
                <Link
                  to={`/search?kind=${k}`}
                  onClick={onNavigate}
                  className="flex items-center gap-3 rounded px-3 py-1.5 text-body text-fg-muted hover:bg-surface-raised hover:text-fg"
                >
                  <Icon className="h-4 w-4" aria-hidden />
                  {plural}
                </Link>
              </li>
            );
          })}
        </ul>
      </div>
      {hasRole(user, 'publisher') && <NavGroup title="Publisher" items={publisherNav} onNavigate={onNavigate} />}
    </nav>
  );
}

export function Sidebar() {
  return (
    <aside className="fixed inset-y-0 left-0 top-14 hidden w-60 overflow-y-auto border-r border-border bg-surface/40 scrollbar-thin lg:block">
      <SidebarContent />
      <div className="mx-4 mt-2 rounded-lg border border-border bg-surface p-4 text-caption text-fg-muted">
        <p className="font-semibold text-fg">Prototype</p>
        <p className="mt-1">Catalog data is mocked. Auth: {import.meta.env.VITE_AUTH_MODE === 'http' ? 'live API' : 'in-memory mock'}.</p>
      </div>
    </aside>
  );
}
