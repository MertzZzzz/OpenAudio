import { Pause, Play } from 'lucide-react';
import { cn } from '@/shared/lib/cn';
import { usePlayer, type PlayerTrack } from './PlayerContext';

export interface PlayButtonProps {
  track: PlayerTrack;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const sizes = { sm: 'h-7 w-7', md: 'h-9 w-9', lg: 'h-12 w-12' };
const icons = { sm: 'h-3.5 w-3.5', md: 'h-4 w-4', lg: 'h-5 w-5' };

export function PlayButton({ track, size = 'md', className }: PlayButtonProps) {
  const player = usePlayer();
  const active = player.isCurrent(track.id) && player.playing;
  return (
    <button
      type="button"
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        player.toggle(track);
      }}
      aria-label={active ? `Pause ${track.title}` : `Play preview of ${track.title}`}
      aria-pressed={active}
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-full transition-all',
        active ? 'bg-primary text-primary-fg' : 'bg-fg text-background hover:scale-105',
        sizes[size],
        className,
      )}
    >
      {active ? <Pause className={icons[size]} fill="currentColor" /> : <Play className={cn(icons[size], 'translate-x-[1px]')} fill="currentColor" />}
    </button>
  );
}
