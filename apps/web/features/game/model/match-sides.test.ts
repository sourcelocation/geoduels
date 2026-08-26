import { describe, expect, it } from "vitest";
import type { Snapshot } from "./types";
import { deriveMatchSides, withRoundSideResults } from "./match-sides";
import { getTeamPresentation } from "../../../lib/team-presentation";

function teamSnapshot(): Snapshot {
  return {
    matchId: "match-1",
    mode: "team_duel",
    state: "ended",
    phase: "round_result",
    roundPhase: "round_result",
    phaseStartedAt: 0,
    phaseEndsAt: 0,
    roundMsLeft: 0,
    eventSequence: 1,
    players: {
      blueTwo: {
        userId: "blueTwo",
        displayName: "Blue Two",
        mmr: 1900,
        avatarUrl: "https://example.com/blue-two.png",
        selectedBadge: {
          id: "badge-1",
          kind: "rank",
          label: "Badge",
          description: "",
          imageUrl: "/badge.png",
          owned: true,
          unobtainable: false,
        },
        teamId: "b",
        hp: 3200,
        finalized: false,
        disconnected: false,
      },
      redSelf: {
        userId: "redSelf",
        displayName: "Red Self",
        mmr: 2100,
        avatarUrl: "https://example.com/red-self.png",
        teamId: "a",
        hp: 6000,
        finalized: false,
        disconnected: false,
      },
      blueOne: {
        userId: "blueOne",
        displayName: "Blue One",
        mmr: 2000,
        teamId: "b",
        hp: 3200,
        finalized: false,
        disconnected: true,
      },
      redMate: {
        userId: "redMate",
        displayName: "Red Mate",
        mmr: 1800,
        teamId: "a",
        hp: 6000,
        finalized: false,
        disconnected: false,
      },
    },
    teams: {
      a: {
        teamId: "a",
        name: "Team Red",
        hp: 6000,
        players: ["redSelf", "redMate"],
      },
      b: {
        teamId: "b",
        name: "Team Blue",
        hp: 3200,
        players: ["blueOne", "blueTwo"],
      },
    },
  };
}

const fallbackSelf = {
  id: "redSelf",
  name: "Red Self",
  avatarFallback: "R",
};

describe("deriveMatchSides", () => {
  it("derives team identity and state without promoting a member to the team card", () => {
    const derived = deriveMatchSides({
      snapshot: teamSnapshot(),
      selfUserId: "redSelf",
      fallbackSelf,
    });

    expect(derived.sides.self.participant).toEqual({
      kind: "team",
      id: "a",
      name: "Team Red",
      avatarFallback: "R",
      avatarColor: getTeamPresentation("a").color,
      members: expect.arrayContaining([
        expect.objectContaining({ id: "redSelf" }),
        expect.objectContaining({ id: "redMate" }),
      ]),
    });
    expect(derived.sides.opponent.participant.kind).toBe("team");
    expect(derived.sides.opponent.hp).toBe(3200);
    expect(derived.sides.opponent.connection).toBe("degraded");
    expect(derived.sides.opponent.participant).not.toHaveProperty("rating");
    expect(derived.sides.opponent.participant).not.toHaveProperty("selectedBadge");
    expect(derived.sides.opponent.participant).not.toHaveProperty("avatarUrl");
  });

  it("keeps team identity stable regardless of player insertion order", () => {
    const snapshot = teamSnapshot();
    snapshot.players = {
      redMate: snapshot.players.redMate,
      blueOne: snapshot.players.blueOne,
      redSelf: snapshot.players.redSelf,
      blueTwo: snapshot.players.blueTwo,
    };

    const derived = deriveMatchSides({
      snapshot,
      selfUserId: "redSelf",
      fallbackSelf,
    });

    expect(derived.sides.self.id).toBe("a");
    expect(derived.sides.opponent.id).toBe("b");
    expect(derived.sides.opponent.participant.name).toBe("Team Blue");
  });

  it("uses aggregate team round results instead of member results", () => {
    const snapshot = teamSnapshot();
    const derived = deriveMatchSides({
      snapshot,
      selfUserId: "redSelf",
      fallbackSelf,
    });
    const roundSides = withRoundSideResults(derived.sides, {
      roundId: "round-1",
      roundNumber: 1,
      actualLocation: { lat: 0, lng: 0 },
      players: {
        redSelf: {
          userId: "redSelf",
          lat: 0,
          lng: 0,
          score: 100,
          distanceKm: 1000,
          hpAfterRound: 111,
        },
        blueOne: {
          userId: "blueOne",
          lat: 0,
          lng: 0,
          score: 200,
          distanceKm: 900,
          hpAfterRound: 222,
        },
      },
      teams: {
        a: {
          teamId: "a",
          representativeUserId: "redMate",
          lat: 0,
          lng: 0,
          score: 4800,
          distanceKm: 8,
          hpAfterRound: 6000,
        },
        b: {
          teamId: "b",
          representativeUserId: "blueTwo",
          lat: 0,
          lng: 0,
          score: 2300,
          distanceKm: 600,
          hpAfterRound: 3500,
        },
      },
    });

    expect(roundSides.self).toEqual(
      expect.objectContaining({ score: 4800, distanceKm: 8, hp: 6000 }),
    );
    expect(roundSides.opponent).toEqual(
      expect.objectContaining({ score: 2300, distanceKm: 600, hp: 3500 }),
    );
  });
});
