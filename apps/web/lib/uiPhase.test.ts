import { describe, expect, it } from 'vitest';
import { deriveUIPhase } from './uiPhase';
import type { Snapshot } from '../features/game/model/types';

const baseSnapshot: Snapshot = {
  matchId: 'm1',
  state: 'active',
  phase: 'live',
  roundPhase: 'round_live',
  phaseStartedAt: 1000,
  phaseEndsAt: 2000,
  roundMsLeft: 30000,
  currentRound: { roundId: 'r1', roundNumber: 1, location: { panoId: 'pano-123' } },
  players: {},
  eventSequence: 1
};

describe('deriveUIPhase', () => {
  it.each([
    ['queueing fallback', null, 'queueing', 'queueing'],
    ['queued fallback', null, 'queued', 'queueing'],
    ['matched fallback', null, 'matched', 'queueing'],
    ['round intro', { ...baseSnapshot, roundPhase: 'round_intro' }, 'matched', 'prematch_countdown'],
    ['round result phase', { ...baseSnapshot, phase: 'round_result' }, 'matched', 'round_result'],
    ['round transition', { ...baseSnapshot, roundPhase: 'round_transition' }, 'matched', 'round_result'],
    ['ended match', { ...baseSnapshot, state: 'ended' }, 'matched', 'match_end']
  ] as const)('derives %s', (_name, snapshot, status, expected) => {
    expect(deriveUIPhase({ snapshot, status })).toBe(expected);
  });
});
