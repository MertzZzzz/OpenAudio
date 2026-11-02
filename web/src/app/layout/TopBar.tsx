import { useState, type FormEvent } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Menu, Search } from 'lucide-react';
import { Input } from '@/shared/ui/Input';
import { Drawer } from '@/shared/ui/Overlay';
import { UserMenu } from '@/features/auth/UserMenu';
import { SidebarContent } from './Sidebar';

export function Logo() {
  return (
    <Link to="/" className="flex items-center gap-2 font-semibold tracking-tight" aria-label="OpenAudio home">
      <svg viewBox="0 0 32 32" className="h-7 w-7" aria-hidden>
        <rect width="32" height="32" rx="8" fill="rgb(var(--oa-primary))" />
        <path d="M7 18v-4M11 22V10M15 25V7M19 20v-8M23 23V9M27 17v-2" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" />
      </svg>
      <span className="hidden sm:inline">OpenAudio</span>
    </Link>
  );
}

export function TopBar() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [q, setQ] = useState(params.get('q') ?? '');
  const [menuOpen, setMenuOpen] = useState(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    navigate(q.trim() ? `/search?q=${encodeURIComponent(q.trim())}` : '/search');
  };

  return (
    <header className="fixed inset-x-0 top-0 z-30 h-14 border-b border-border bg-background/85 backdrop-blur">
      <div className="flex h-full items-center gap-4 px-4">
        <button type="button" className="rounded p-1.5 text-fg-muted hover:bg-surface-raised lg:hidden" aria-label="Open navigation" onClick={() => setMenuOpen(true)}>
          <Menu className="h-5 w-5" />
        </button>
        <div className="lg:w-52">
          <Logo />
        </div>
        <form role="search" onSubmit={submit} className="max-w-xl flex-1">
          <Input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search packages, samples, effects…"
            aria-label="Search packages"
            icon={<Search className="h-4 w-4" />}
          />
        </form>
        <div className="ml-auto flex items-center gap-3">
          <UserMenu />
        </div>
      </div>
      <Drawer open={menuOpen} onClose={() => setMenuOpen(false)} title="Navigation" className="left-0 right-auto border-l-0 border-r">
        <SidebarContent onNavigate={() => setMenuOpen(false)} />
      </Drawer>
    </header>
  );
}
