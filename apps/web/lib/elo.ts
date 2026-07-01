export const INITIAL_MMR = 500;
export const INITIAL_RATING_RD = 220;
export const MAX_DUEL_MMR_DELTA = 80;

const MIN_RANKED_MMR = 500;
const LOW_MMR_FORGIVENESS_END_MMR = 1000;
const PROVISIONAL_LOSS_MIN_FACTOR = 0.2;
const PROVISIONAL_PROTECTION_MMR_GAP = 250;
const MIN_RATING_RD = 110;
const MAX_RATING_RD = 220;
const GLICKO_C = Math.sqrt((MAX_RATING_RD ** 2 - MIN_RATING_RD ** 2) / 365);
const Q = Math.log(10) / 400;

export type EloWinner = 'self' | 'opp' | 'draw';

export function calculateDuelEloDeltas(
  selfElo: number,
  opponentElo: number,
  winner: EloWinner,
  selfRatingRd = INITIAL_RATING_RD,
  opponentRatingRd = INITIAL_RATING_RD,
  selfUpdatedAt?: string | number | Date,
  opponentUpdatedAt?: string | number | Date,
  now: Date = new Date()
): {
  selfDelta: number;
  opponentDelta: number;
  selfRatingRd: number;
  opponentRatingRd: number;
} {
  const selfRd = inflateRatingRd(selfRatingRd, selfUpdatedAt, now);
  const opponentRd = inflateRatingRd(opponentRatingRd, opponentUpdatedAt, now);
  const selfScore = winner === 'self' ? 1 : winner === 'opp' ? 0 : 0.5;
  const opponentScore = winner === 'opp' ? 1 : winner === 'self' ? 0 : 0.5;

  const selfNext = calculateGlickoRating(selfElo, selfRd, opponentElo, opponentRd, selfScore);
  const opponentNext = calculateGlickoRating(opponentElo, opponentRd, selfElo, selfRd, opponentScore);
  const selfCappedRating = capRatingDelta(selfElo, selfNext.rating);
  const opponentCappedRating = capRatingDelta(opponentElo, opponentNext.rating);
  const selfProtectedRating = applyProvisionalOpponentLossProtection(
    selfElo,
    opponentElo,
    opponentRd,
    selfCappedRating
  );
  const opponentProtectedRating = applyProvisionalOpponentLossProtection(
    opponentElo,
    selfElo,
    selfRd,
    opponentCappedRating
  );
  const selfNextRating = clampRankedMmr(applyLowMmrLossForgiveness(selfElo, selfProtectedRating));
  const opponentNextRating = clampRankedMmr(
    applyLowMmrLossForgiveness(opponentElo, opponentProtectedRating)
  );

  return {
    selfDelta: selfNextRating - selfElo,
    opponentDelta: opponentNextRating - opponentElo,
    selfRatingRd: selfNext.rd,
    opponentRatingRd: opponentNext.rd
  };
}

function calculateGlickoRating(
  rating: number,
  rd: number,
  opponentRating: number,
  opponentRd: number,
  score: number
): { rating: number; rd: number } {
  const safeRd = clampRatingRd(rd);
  const safeOpponentRd = clampRatingRd(opponentRd);
  const g = glickoG(safeOpponentRd);
  const expected = glickoExpected(rating, opponentRating, safeOpponentRd);
  const dSquared = 1 / (Q * Q * g * g * expected * (1 - expected));
  const nextVariance = 1 / (1 / (safeRd * safeRd) + 1 / dSquared);
  const nextRating = rating + Q * nextVariance * g * (score - expected);

  return {
    rating: Math.round(nextRating),
    rd: clampRatingRd(Math.sqrt(nextVariance))
  };
}

function glickoG(rd: number): number {
  return 1 / Math.sqrt(1 + (3 * Q * Q * rd * rd) / (Math.PI * Math.PI));
}

function glickoExpected(self: number, opponent: number, opponentRd: number): number {
  return 1 / (1 + 10 ** (-glickoG(opponentRd) * (self - opponent) / 400));
}

function inflateRatingRd(rd: number, updatedAt: string | number | Date | undefined, now: Date): number {
  const safeRd = clampRatingRd(rd);
  const updated = parseDate(updatedAt);
  if (!updated || Number.isNaN(updated.getTime()) || now <= updated) {
    return safeRd;
  }
  const inactiveDays = (now.getTime() - updated.getTime()) / 86_400_000;
  if (inactiveDays <= 0) {
    return safeRd;
  }
  return clampRatingRd(Math.sqrt(safeRd * safeRd + GLICKO_C * GLICKO_C * inactiveDays));
}

function parseDate(value: string | number | Date | undefined): Date | null {
  if (value === undefined || value === null || value === '') return null;
  return value instanceof Date ? value : new Date(value);
}

function clampRatingRd(rd: number): number {
  if (!Number.isFinite(rd) || rd <= 0) return INITIAL_RATING_RD;
  if (rd < MIN_RATING_RD) return MIN_RATING_RD;
  if (rd > MAX_RATING_RD) return MAX_RATING_RD;
  return rd;
}

function capRatingDelta(current: number, next: number): number {
  const delta = next - current;
  if (delta > MAX_DUEL_MMR_DELTA) return current + MAX_DUEL_MMR_DELTA;
  if (delta < -MAX_DUEL_MMR_DELTA) return current - MAX_DUEL_MMR_DELTA;
  return next;
}

function applyLowMmrLossForgiveness(current: number, next: number): number {
  const delta = next - current;
  if (delta >= 0 || current >= LOW_MMR_FORGIVENESS_END_MMR) return next;
  if (current <= MIN_RANKED_MMR) return current;

  const rangeSize = LOW_MMR_FORGIVENESS_END_MMR - MIN_RANKED_MMR;
  if (rangeSize <= 0) return next;

  const factor = (current - MIN_RANKED_MMR) / rangeSize;
  return current + Math.round(delta * factor);
}

function applyProvisionalOpponentLossProtection(
  selfMmr: number,
  opponentMmr: number,
  opponentRd: number,
  next: number
): number {
  const delta = next - selfMmr;
  if (delta >= 0 || opponentMmr > selfMmr - PROVISIONAL_PROTECTION_MMR_GAP) return next;

  const factor = provisionalRdFactor(opponentRd);
  if (factor >= 1) return next;

  return selfMmr + Math.round(delta * factor);
}

function provisionalRdFactor(rd: number): number {
  const safeRd = clampRatingRd(rd);
  const rangeSize = MAX_RATING_RD - MIN_RATING_RD;
  if (rangeSize <= 0) return 1;

  const certainty = 1 - (safeRd - MIN_RATING_RD) / rangeSize;
  return Math.min(1, Math.max(PROVISIONAL_LOSS_MIN_FACTOR, certainty));
}

function clampRankedMmr(mmr: number): number {
  return mmr < MIN_RANKED_MMR ? MIN_RANKED_MMR : mmr;
}
