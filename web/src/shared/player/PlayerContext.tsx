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

/**
 * Placeholder audio player: simulates playback progress with a timer.
 * When previews exist in S3 this is replaced with HTMLAudioElement / Web Audio,
 * keeping the same context API.
 */
export function PlayerProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<PlayerState>({ track: null, playing: false, position: 0, volume: 0.8 });
  const timer = useRef<number | null>(null);

  useEffect(() => {
    if (!state.playing || !state.track) return;
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
  }, [state.playing, state.track]);

  const play = useCallback((track: PlayerTrack) => {
    setState((s) => ({ ...s, track, playing: true, position: s.track?.id === track.id ? s.position : 0 }));
  }, []);

  const pause = useCallback(() => setState((s) => ({ ...s, playing: false })), []);

  const toggle = useCallback((track?: PlayerTrack) => {
    setState((s) => {
      if (track && s.track?.id !== track.id) return { ...s, track, playing: true, position: 0 };
      if (!s.track) return s;
      return { ...s, playing: !s.playing };
    });
  }, []);

  const seek = useCallback((sec: number) => setState((s) => ({ ...s, position: Math.max(0, Math.min(sec, s.track?.durationSec ?? 0)) })), []);
  const setVolume = useCallback((volume: number) => setState((s) => ({ ...s, volume })), []);
  const stop = useCallback(() => setState((s) => ({ ...s, track: null, playing: false, position: 0 })), []);

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
