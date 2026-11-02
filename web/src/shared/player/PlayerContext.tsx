import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';

export interface PlayerTrack {
  id: string;
  title: string;
  subtitle: string;
  durationSec: number;
  previewUrl: string;
  hue?: number;
}

interface PlayerState {
  track: PlayerTrack | null;
  playing: boolean;
  position: number;
  volume: number;
  /** Real audio (HTML5) vs simulated playback for mock data. */
  real: boolean;
  error: string | null;
}

interface PlayerApi extends PlayerState {
  play: (track: PlayerTrack) => void;
  toggle: (track?: PlayerTrack) => void;
  pause: () => void;
  seek: (sec: number) => void;
  setVolume: (v: number) => void;
  stop: () => void;
  isCurrent: (id: string) => boolean;
}

const PlayerContext = createContext<PlayerApi | null>(null);

/** URLs served by the API (or absolute URLs) are played for real; mock:// URLs are simulated. */
export function isPlayable(url: string): boolean {
  return url.startsWith('/') || url.startsWith('http://') || url.startsWith('https://');
}

/**
 * Audio preview player. Real previews use one shared HTMLAudioElement (the API redirects to a
 * short-lived presigned URL); the offline demo simulates playback with a timer.
 */
export function PlayerProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<PlayerState>({ track: null, playing: false, position: 0, volume: 0.8, real: false, error: null });
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const timer = useRef<number | null>(null);

  const audio = useCallback(() => {
    if (!audioRef.current) {
      const a = new Audio();
      a.preload = 'auto';
      a.addEventListener('timeupdate', () => setState((s) => (s.real ? { ...s, position: a.currentTime } : s)));
      a.addEventListener('loadedmetadata', () =>
        setState((s) => (s.real && s.track && Number.isFinite(a.duration) ? { ...s, track: { ...s.track, durationSec: a.duration } } : s)),
      );
      a.addEventListener('ended', () => setState((s) => (s.real ? { ...s, playing: false, position: 0 } : s)));
      a.addEventListener('error', () => setState((s) => (s.real ? { ...s, playing: false, error: 'Preview could not be loaded' } : s)));
      audioRef.current = a;
    }
    return audioRef.current;
  }, []);

  // Drive the real element from state.
  useEffect(() => {
    if (!state.real || !state.track) {
      audioRef.current?.pause();
      return;
    }
    const a = audio();
    a.volume = state.volume;
    if (state.playing) {
      const started = a.play() as Promise<void> | undefined; // undefined in very old browsers / jsdom
      started?.catch(() => setState((s) => ({ ...s, playing: false, error: 'Playback was blocked or failed' })));
    } else {
      a.pause();
    }
  }, [state.real, state.track, state.playing, state.volume, audio]);

  // Simulated progress for mock tracks.
  useEffect(() => {
    if (state.real || !state.playing || !state.track) return;
    const tick = 100;
    timer.current = window.setInterval(() => {
      setState((s) => {
        if (!s.track) return s;
        const pos = s.position + tick / 1000;
        if (pos >= s.track.durationSec) return { ...s, position: 0, playing: false };
        return { ...s, position: pos };
      });
    }, tick);
    return () => {
      if (timer.current) window.clearInterval(timer.current);
    };
  }, [state.real, state.playing, state.track]);

  const load = useCallback(
    (track: PlayerTrack): Partial<PlayerState> => {
      const real = isPlayable(track.previewUrl);
      if (real) {
        const a = audio();
        a.src = track.previewUrl;
        a.currentTime = 0;
      }
      return { track, real, position: 0, playing: true, error: null };
    },
    [audio],
  );

  const play = useCallback(
    (track: PlayerTrack) => setState((s) => (s.track?.id === track.id ? { ...s, playing: true } : { ...s, ...load(track) })),
    [load],
  );

  const pause = useCallback(() => setState((s) => ({ ...s, playing: false })), []);

  const toggle = useCallback(
    (track?: PlayerTrack) => {
      setState((s) => {
        if (track && s.track?.id !== track.id) return { ...s, ...load(track) };
        if (!s.track) return s;
        return { ...s, playing: !s.playing };
      });
    },
    [load],
  );

  const seek = useCallback((sec: number) => {
    setState((s) => {
      const pos = Math.max(0, Math.min(sec, s.track?.durationSec ?? 0));
      if (s.real && audioRef.current) audioRef.current.currentTime = pos;
      return { ...s, position: pos };
    });
  }, []);

  const setVolume = useCallback((volume: number) => setState((s) => ({ ...s, volume })), []);
  const stop = useCallback(() => {
    audioRef.current?.pause();
    setState((s) => ({ ...s, track: null, playing: false, position: 0, real: false, error: null }));
  }, []);

  const value = useMemo<PlayerApi>(
    () => ({ ...state, play, pause, toggle, seek, setVolume, stop, isCurrent: (id) => state.track?.id === id }),
    [state, play, pause, toggle, seek, setVolume, stop],
  );

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>;
}

export function usePlayer(): PlayerApi {
  const ctx = useContext(PlayerContext);
  if (!ctx) throw new Error('usePlayer must be used within <PlayerProvider>');
  return ctx;
}
