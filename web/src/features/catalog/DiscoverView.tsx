import { useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowRight, Search } from 'lucide-react';
import type { PackageKind } from '@/shared/types/domain';
import { Button } from '@/shared/ui/Button';
import { Input } from '@/shared/ui/Input';
import { KIND_META } from '@/shared/lib/kinds';
import { useDiscover } from './api';
import { PackageCarousel } from './PackageCarousel';

const quickFilters: PackageKind[] = ['instrument', 'effect', 'sample-pack', 'preset-pack', 'midi-tool'];
const genreChips = ['deep-house', 'techno', 'lo-fi', 'drum-and-bass', 'ambient', 'cinematic'];

export function DiscoverView() {
  const { data, isLoading, isError, refetch } = useDiscover();
  const [q, setQ] = useState('');
  const navigate = useNavigate();

  const submit = (e: FormEvent) => {
    e.preventDefault();
    navigate(`/search${q.trim() ? `?q=${encodeURIComponent(q.trim())}` : ''}`);
  };

  return (
    <div className="space-y-10">
      <section className="relative overflow-hidden rounded-xl border border-border bg-surface px-6 py-12 sm:px-10">
        <div aria-hidden className="pointer-events-none absolute inset-0 bg-[radial-gradient(60%_80%_at_10%_0%,rgb(var(--oa-primary)/.25),transparent),radial-gradient(50%_70%_at_100%_100%,rgb(var(--oa-accent)/.15),transparent)]" />
        <div className="relative max-w-2xl">
          <p className="text-caption font-semibold uppercase tracking-widest text-accent">Open audio packages</p>
          <h1 className="mt-2 text-display">Find sounds, instruments and effects you can actually reuse.</h1>
          <p className="mt-3 text-fg-muted">Versioned, content-addressed and openly licensed. Publish once, pin exact versions in every project.</p>
          <form onSubmit={submit} role="search" className="mt-6 flex gap-2">
            <Input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Try “warm tape drums”, “reverb” or “dnb”"
              aria-label="Search the catalog"
              icon={<Search className="h-4 w-4" />}
              className="h-11"
            />
            <Button type="submit" size="lg">Search</Button>
          </form>
          <div className="mt-5 flex flex-wrap gap-2" aria-label="Quick filters">
            {quickFilters.map((k) => {
              const { icon: Icon, plural } = KIND_META[k];
              return (
                <Link key={k} to={`/search?kind=${k}`} className="inline-flex items-center gap-2 rounded-full border border-border bg-surface-raised px-3 py-1.5 text-body text-fg-muted transition-colors hover:border-primary hover:text-fg">
                  <Icon className="h-4 w-4" aria-hidden />
                  {plural}
                </Link>
              );
            })}
          </div>
        </div>
      </section>

      {isError ? (
        <div className="rounded-lg border border-danger/30 bg-danger/10 p-4 text-danger">
          Failed to load the catalog. <button className="underline" onClick={() => refetch()}>Retry</button>
        </div>
      ) : (
        <>
          <PackageCarousel title="Popular" packages={data?.popular} loading={isLoading} />
          <PackageCarousel title="New releases" packages={data?.newReleases} loading={isLoading} />
          <PackageCarousel title="Editors’ picks" packages={data?.editorsPicks} loading={isLoading} />
        </>
      )}

      <section aria-labelledby="genres-heading" className="space-y-3">
        <h2 id="genres-heading" className="text-heading">Browse by genre</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          {genreChips.map((g, i) => (
            <Link
              key={g}
              to={`/search?genre=${g}`}
              className="group flex h-20 items-end justify-between rounded-lg border border-border p-3 font-semibold transition-colors hover:border-border-strong"
              style={{ background: `linear-gradient(135deg, hsl(${(i * 55 + 250) % 360} 60% 30% / .6), transparent)` }}
            >
              {g.split('-').map((w) => w[0]!.toUpperCase() + w.slice(1)).join(' ').replace('And', '&')}
              <ArrowRight className="h-4 w-4 opacity-0 transition-opacity group-hover:opacity-100" aria-hidden />
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}
