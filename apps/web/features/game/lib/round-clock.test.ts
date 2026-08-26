import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Snapshot } from '../model/types';
import { RoundClock } from './round-clock';

function createLiveSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    matchId: 'match-1',
    state: 'active',
    phase: 'live',
    roundPhase: 'round_live',
    phaseStartedAt: 100_000,
    phaseEndsAt: 110_000,
    roundMsLeft: 10_000,
    currentRound: {
      roundId: 'round-1',
      roundNumber: 1,
      timerStarted: true,
      location: { panoId: 'pano-123' }
    },
    players: {},
    eventSequence: 1,
    ...overrides
  };
}

describe('RoundClock', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      return window.setTimeout(() => callback(performance.now()), 16);
    });
    vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
      window.clearTimeout(id);
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('uses the latest server offset immediately', () => {
    vi.setSystemTime(102_000);
    const clock = new RoundClock();
    const ticks: Array<{ roundMSLeft: number; displayRoundSeconds: number }> = [];

    clock.setServerTime(100_000);
    clock.start(createLiveSnapshot(), (state) => ticks.push(state));
    vi.advanceTimersByTime(16);

    expect(ticks[ticks.length - 1]?.roundMSLeft).toBeGreaterThan(9_900);
    expect(ticks[ticks.length - 1]?.displayRoundSeconds).toBe(10);
    clock.reset();
  });

  it('applies later offset corrections without smoothing', () => {
    vi.setSystemTime(100_000);
    const clock = new RoundClock();
    const ticks: Array<{ roundMSLeft: number; displayRoundSeconds: number }> = [];

    clock.setServerTime(100_000);
    clock.start(createLiveSnapshot(), (state) => ticks.push(state));
    vi.advanceTimersByTime(16);
    expect(ticks[ticks.length - 1]?.roundMSLeft).toBeGreaterThan(9_900);

    vi.setSystemTime(102_000);
    clock.setServerTime(100_000);
    vi.advanceTimersByTime(16);

    expect(ticks[ticks.length - 1]?.roundMSLeft).toBeGreaterThan(9_900);
    clock.reset();
  });
});
