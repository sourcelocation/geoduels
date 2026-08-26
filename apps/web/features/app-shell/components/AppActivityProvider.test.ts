import { describe, expect, it, vi } from "vitest";
import { initialMatchmakingState } from "../../../lib/matchmaking";
import { deriveAppActivities } from "./AppActivityProvider";

const match = {
  matchmaking: initialMatchmakingState,
  connected: false,
  snapshot: null,
  activeMatchId: "",
  sourcePartyId: "",
  sourcePartyInviteCode: "",
  queueError: "",
  singleplayerError: "",
  connectionIssue: "",
  onlinePlayers: 0,
  teamPings: [],
};

const party = {
  status: "ready" as const,
  partyId: "party-1",
  inviteCode: "ABCD12",
  snapshot: { id: "party-1", inviteCode: "ABCD12", members: [] } as never,
  self: { userId: "self" } as never,
  error: "",
};

describe("deriveAppActivities", () => {
  it("keeps party status on profile routes and hides it inside the party route", () => {
    const input = { party, match, nowMs: 0, cancelQueue: vi.fn() };
    expect(deriveAppActivities({ ...input, pathname: "/players/[id]" })).toEqual([
      { kind: "party", label: "In party — ABCD12", href: "/party/ABCD12" },
    ]);
    expect(deriveAppActivities({ ...input, pathname: "/party/[code]" })).toEqual([]);
  });

  it("projects queue cancellation from the global match runtime", () => {
    const cancelQueue = vi.fn();
    const tasks = deriveAppActivities({
      party: { ...party, status: "idle", snapshot: null, self: null },
      match: {
        ...match,
        matchmaking: { ...initialMatchmakingState, status: "queueing", queueStartedAt: 1_000 },
      },
      pathname: "/players/[id]",
      nowMs: 6_000,
      cancelQueue,
    });
    expect(tasks[0]).toMatchObject({ kind: "queue", label: "Finding a duel · 0:05" });
    if (tasks[0]?.kind === "queue") tasks[0].onCancel();
    expect(cancelQueue).toHaveBeenCalledOnce();
  });
});
