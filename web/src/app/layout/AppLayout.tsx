import { Suspense } from 'react';
import { Outlet } from 'react-router-dom';
import { PlayerBar } from '@/shared/player/PlayerBar';
import { Skeleton } from '@/shared/ui/Skeleton';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

function PageFallback() {
  return (
    <div className="space-y-4" aria-busy="true" aria-label="Loading page">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}

export function AppLayout() {
  return (
    <div className="min-h-screen">
      <a href="#main" className="sr-only z-50 rounded bg-primary px-3 py-2 text-primary-fg focus:not-sr-only focus:fixed focus:left-4 focus:top-4">
        Skip to content
      </a>
      <TopBar />
      <Sidebar />
      <main id="main" className="px-4 pb-28 pt-20 sm:px-6 lg:pl-[17rem] lg:pr-8">
        <div className="mx-auto max-w-7xl">
          <Suspense fallback={<PageFallback />}>
            <Outlet />
          </Suspense>
        </div>
      </main>
      <PlayerBar />
    </div>
  );
}
