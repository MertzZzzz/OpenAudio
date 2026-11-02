import { useRef } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import type { Package } from '@/shared/types/domain';
import { Button } from '@/shared/ui/Button';
import { Skeleton } from '@/shared/ui/Skeleton';
import { PackageCard } from './PackageCard';

export function PackageCarousel({ title, packages, loading }: { title: string; packages?: Package[]; loading?: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const scroll = (dir: 1 | -1) => ref.current?.scrollBy({ left: dir * ref.current.clientWidth * 0.8, behavior: 'smooth' });
  const headingId = `carousel-${title.toLowerCase().replace(/[^a-z]+/g, '-')}`;
  return (
    <section aria-labelledby={headingId} className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 id={headingId} className="text-heading">{title}</h2>
        <div className="flex gap-1">
          <Button variant="ghost" size="icon" aria-label={`Scroll ${title} left`} onClick={() => scroll(-1)}>
            <ChevronLeft className="h-4 w-4" />
          </Button>
          <Button variant="ghost" size="icon" aria-label={`Scroll ${title} right`} onClick={() => scroll(1)}>
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <div ref={ref} className="-mx-1 flex snap-x gap-4 overflow-x-auto px-1 pb-2 scrollbar-thin">
        {loading || !packages
          ? Array.from({ length: 5 }, (_, i) => <Skeleton key={i} className="h-72 w-52 shrink-0" />)
          : packages.map((p) => <PackageCard key={p.id} pkg={p} className="w-52 shrink-0 snap-start" />)}
      </div>
    </section>
  );
}
