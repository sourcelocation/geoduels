import type { RatingDeltaPreview, Snapshot } from "./types";

export type MatchRatingOutcome = "win" | "lose" | "draw";

export type MatchRatingDeltas = {
  selfRatingDelta?: number;
  opponentRatingDelta?: number;
};

function opponentOutcome(outcome: MatchRatingOutcome): MatchRatingOutcome {
  if (outcome === "win") return "lose";
  if (outcome === "lose") return "win";
  return "draw";
}

export function getRatingDeltaForOutcome(
  preview: RatingDeltaPreview | undefined,
  outcome: MatchRatingOutcome,
): number | undefined {
  return preview?.[outcome];
}

export function deriveDuelRatingDeltas(params: {
  snapshot: Snapshot | null;
  selfUserId: string;
  opponentUserId: string;
  outcome: MatchRatingOutcome;
}): MatchRatingDeltas {
  const { snapshot, selfUserId, opponentUserId, outcome } = params;
  if (
    !snapshot ||
    snapshot.mode !== "duel" ||
    snapshot.unranked ||
    snapshot.state !== "ended"
  ) {
    return {};
  }

  const selfPlayer = snapshot.players?.[selfUserId];
  const opponentPlayer = snapshot.players?.[opponentUserId];
  if (!selfPlayer || !opponentPlayer) return {};

  const deltas: MatchRatingDeltas = {};
  if (!selfPlayer.isGuest) {
    deltas.selfRatingDelta = getRatingDeltaForOutcome(
      snapshot.ratingPreview?.[selfUserId],
      outcome,
    );
  }
  if (!opponentPlayer.isGuest) {
    deltas.opponentRatingDelta = getRatingDeltaForOutcome(
      snapshot.ratingPreview?.[opponentUserId],
      opponentOutcome(outcome),
    );
  }
  return deltas;
}
