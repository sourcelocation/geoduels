import { describe, expect, it } from "vitest";
import {
  deriveDuelRatingDeltas,
  getRatingDeltaForOutcome,
} from "./match-rating";
import type { Snapshot } from "./types";

function createSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    matchId: "match-1",
    mode: "duel",
    state: "ended",
    phase: "ended",
    roundPhase: "ended",
    phaseStartedAt: 0,
    phaseEndsAt: 0,
    roundMsLeft: 0,
    players: {
      self: {
        userId: "self",
        displayName: "Self",
        mmr: 1200,
        hp: 5000,
        finalized: false,
        disconnected: false,
      },
      opponent: {
        userId: "opponent",
        displayName: "Opponent",
        mmr: 1200,
        hp: 4000,
        finalized: false,
        disconnected: false,
      },
    },
    ratingPreview: {
      self: { win: 20, lose: -18, draw: 1 },
      opponent: { win: 18, lose: -20, draw: -1 },
    },
    eventSequence: 1,
    ...overrides,
  };
}

describe("match rating helpers", () => {
  it("selects a preview value for a player's outcome", () => {
    expect(
      getRatingDeltaForOutcome(
        { win: 20, lose: -18, draw: 1 },
        "lose",
      ),
    ).toBe(-18);
  });

  it("derives both players' deltas from one match outcome", () => {
    expect(
      deriveDuelRatingDeltas({
        snapshot: createSnapshot(),
        selfUserId: "self",
        opponentUserId: "opponent",
        outcome: "win",
      }),
    ).toEqual({ selfRatingDelta: 20, opponentRatingDelta: -20 });
  });

  it("only returns a delta for the registered player", () => {
    const snapshot = createSnapshot({
      players: {
        ...createSnapshot().players,
        opponent: {
          ...createSnapshot().players.opponent,
          isGuest: true,
        },
      },
    });

    expect(
      deriveDuelRatingDeltas({
        snapshot,
        selfUserId: "self",
        opponentUserId: "opponent",
        outcome: "win",
      }),
    ).toEqual({ selfRatingDelta: 20 });
  });

  it("does not derive deltas for unranked or live snapshots", () => {
    expect(
      deriveDuelRatingDeltas({
        snapshot: createSnapshot({ unranked: true }),
        selfUserId: "self",
        opponentUserId: "opponent",
        outcome: "win",
      }),
    ).toEqual({});
    expect(
      deriveDuelRatingDeltas({
        snapshot: createSnapshot({ state: "live", phase: "live", roundPhase: "round_live" }),
        selfUserId: "self",
        opponentUserId: "opponent",
        outcome: "win",
      }),
    ).toEqual({});
  });
});
