import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PlayerProvider, isPlayable, usePlayer, type PlayerTrack } from './PlayerContext';

const mockTrack: PlayerTrack = { id: 'm', title: 'Mock', subtitle: '', durationSec: 2, previewUrl: 'mock://x.mp3' };
const realTrack: PlayerTrack = { id: 'r', title: 'Real', subtitle: '', durationSec: 0, previewUrl: '/api/v1/packages/acme/kit/preview' };

function Probe() {
  const p = usePlayer();
  return (
    <>
      <button onClick={() => p.toggle(mockTrack)}>mock</button>
      <button onClick={() => p.toggle(realTrack)}>real</button>
      <p>{`${p.track?.id ?? '-'} playing=${p.playing} real=${p.real} pos=${p.position.toFixed(1)}`}</p>
    </>
  );
}

describe('player', () => {
  it('knows which URLs are real audio', () => {
    expect(isPlayable('/api/v1/packages/a/b/preview')).toBe(true);
    expect(isPlayable('https://cdn/x.wav')).toBe(true);
    expect(isPlayable('mock://previews/x.mp3')).toBe(false);
    expect(isPlayable('')).toBe(false);
  });

  it('simulates mock previews and uses an audio element for real ones', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const play = vi.spyOn(window.HTMLMediaElement.prototype, 'play').mockImplementation(() => Promise.resolve());
    vi.spyOn(window.HTMLMediaElement.prototype, 'pause').mockImplementation(() => undefined);
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<PlayerProvider><Probe /></PlayerProvider>);

    await user.click(screen.getByRole('button', { name: 'mock' }));
    act(() => void vi.advanceTimersByTime(500));
    expect(screen.getByText(/^m playing=true real=false pos=0\.[4-6]/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'real' }));
    expect(screen.getByText('r playing=true real=true pos=0.0')).toBeInTheDocument();
    expect(play).toHaveBeenCalled();
    vi.useRealTimers();
  });
});
