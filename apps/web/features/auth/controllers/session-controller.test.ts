import { describe, expect, it, vi } from "vitest";
import { createRuntimeConfigFixture } from "../../../test/runtime-config.fixture";
import { AuthGateway } from "../auth-gateway";
import { SessionController } from "./session-controller";

function base64URL(value: string) {
  return btoa(value).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function tokenWithExp(expiresAtMs: number) {
  return [base64URL(JSON.stringify({ alg: "none", typ: "JWT" })), base64URL(JSON.stringify({ exp: Math.floor(expiresAtMs / 1000) })), "signature"].join(".");
}

function createController() {
  const config = createRuntimeConfigFixture();
  return new SessionController({ config, authGateway: new AuthGateway(config), onResetSession: vi.fn() });
}

describe("SessionController projection", () => {
  it("updates leaderboard state separately from profile state", () => {
    const subject = createController();
    subject.applyProfileSnapshot({ display_name: "Player", mmr: 1234, wins: 7 });
    subject.applyLeaderboardSummary({
      mode: "duel", season: "s2", nextResetAt: "2026-07-01T21:00:00Z", selfRank: 3, totalPlayers: 99,
      entries: [{ rank: 1, userId: "top", displayName: "Top", avatarUrl: "", mmr: 1500, gamesPlayed: 20, wins: 15 }],
    });
    expect(subject.getState().displayName).toBe("Player");
    expect(subject.getState().mmr).toBe(1234);
    expect(subject.getState().leaderboard?.selfRank).toBe(3);
    expect(subject.getState().leaderboard?.entries).toHaveLength(1);
    subject.destroy();
  });

  it("applies a committed match rating to the active profile", () => {
    const subject = createController();
    subject.applySessionSnapshot(
      { userId: "self", accessToken: tokenWithExp(Date.now() + 60 * 60_000), nicknameRequired: false, nicknameInput: "Player" },
      { mmr: 1000, ratingRd: 220, gamesPlayed: 9, wins: 5, rankedGamesPlayed: 7, rankedWins: 4 },
    );
    subject.applyCommittedRating(1025, 180);
    expect(subject.getState()).toEqual(expect.objectContaining({ mmr: 1025, ratingRd: 180, gamesPlayed: 9, wins: 5, rankedGamesPlayed: 7, rankedWins: 4 }));
    subject.destroy();
  });
});
