import { describe, expect, it } from 'vitest';
import { calculateDuelEloDeltas, INITIAL_MMR, MAX_DUEL_MMR_DELTA } from './elo';

describe('calculateDuelEloDeltas', () => {
  it('moves high-RD players quickly and lowers rating uncertainty', () => {
    const result = calculateDuelEloDeltas(1500, 1500, 'self', 350, 350);

    expect(result.selfDelta).toBe(MAX_DUEL_MMR_DELTA);
    expect(result.opponentDelta).toBe(-MAX_DUEL_MMR_DELTA);
    expect(result.selfRatingRd).toBeLessThan(350);
    expect(result.opponentRatingRd).toBeLessThan(350);
  });

  it('keeps established equal players near a thirty-point move', () => {
    const result = calculateDuelEloDeltas(1500, 1500, 'self', 110, 110);

    expect(result.selfDelta).toBeGreaterThanOrEqual(28);
    expect(result.selfDelta).toBeLessThanOrEqual(32);
    expect(result.opponentDelta).toBeLessThanOrEqual(-28);
    expect(result.opponentDelta).toBeGreaterThanOrEqual(-32);
  });

  it('starts ranked players at 500 MMR', () => {
    expect(INITIAL_MMR).toBe(500);
  });

  it('forgives losses while players are below 1000 MMR', () => {
    const low = calculateDuelEloDeltas(600, 600, 'self', 110, 110);
    const nearExit = calculateDuelEloDeltas(900, 900, 'self', 110, 110);
    const regular = calculateDuelEloDeltas(1000, 1000, 'self', 110, 110);

    expect(low.opponentDelta).toBeGreaterThanOrEqual(-8);
    expect(low.opponentDelta).toBeLessThanOrEqual(-4);
    expect(nearExit.opponentDelta).toBeGreaterThanOrEqual(-26);
    expect(nearExit.opponentDelta).toBeLessThanOrEqual(-22);
    expect(regular.opponentDelta).toBeGreaterThanOrEqual(-32);
    expect(regular.opponentDelta).toBeLessThanOrEqual(-28);
  });

  it('protects established favorites from large losses against new low-rated opponents', () => {
    const result = calculateDuelEloDeltas(
      1000,
      500,
      'opp',
      110,
      220
    );

    expect(result.selfDelta).toBeGreaterThanOrEqual(-15);
    expect(result.selfDelta).toBeLessThanOrEqual(-8);
    expect(result.opponentDelta).toBe(MAX_DUEL_MMR_DELTA);
  });

  it('keeps normal upset penalties against established low-rated opponents', () => {
    const result = calculateDuelEloDeltas(
      1000,
      500,
      'opp',
      110,
      110
    );

    expect(result.selfDelta).toBeGreaterThanOrEqual(-65);
    expect(result.selfDelta).toBeLessThanOrEqual(-55);
  });
});
