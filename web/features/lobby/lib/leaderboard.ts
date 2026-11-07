import { apiFetch, authHeaders } from "../../../lib/http";
import { INITIAL_MMR } from "../../../lib/elo";

/** What a visitor sees is the same for everyone; a player's own has their rank in it. */
export function leaderboardKey(userId: string) {
  return ["leaderboard", userId];
}

export type LeaderboardEntrySummary = {
  rank: number;
  userId: string;
  displayName: string;
  avatarUrl: string;
  mmr: number;
  gamesPlayed: number;
  wins: number;
};

export type LeaderboardSummary = {
  mode: string;
  season: string;
  nextResetAt?: string;
  selfRank: number;
  totalPlayers: number;
  entries: LeaderboardEntrySummary[];
};

export function normalizeLeaderboardSummary(
  value: unknown,
): LeaderboardSummary | null {
  if (!value || typeof value !== "object") return null;
  const raw = value as Record<string, unknown>;
  const entries = Array.isArray(raw.entries)
    ? raw.entries
        .map((entry) => normalizeLeaderboardEntry(entry))
        .filter((entry): entry is LeaderboardEntrySummary => entry !== null)
    : [];
  return {
    mode: typeof raw.mode === "string" ? raw.mode : "duel",
    season: typeof raw.season === "string" ? raw.season : "s2",
    nextResetAt:
      typeof raw.nextResetAt === "string" ? raw.nextResetAt : undefined,
    selfRank: typeof raw.selfRank === "number" ? raw.selfRank : 0,
    totalPlayers:
      typeof raw.totalPlayers === "number" ? raw.totalPlayers : entries.length,
    entries,
  };
}

function normalizeLeaderboardEntry(
  value: unknown,
): LeaderboardEntrySummary | null {
  if (!value || typeof value !== "object") return null;
  const raw = value as Record<string, unknown>;
  return {
    rank: typeof raw.rank === "number" ? raw.rank : 0,
    userId: typeof raw.userId === "string" ? raw.userId : "",
    displayName: typeof raw.displayName === "string" ? raw.displayName : "",
    avatarUrl: typeof raw.avatarUrl === "string" ? raw.avatarUrl : "",
    mmr: typeof raw.mmr === "number" ? raw.mmr : INITIAL_MMR,
    gamesPlayed: typeof raw.gamesPlayed === "number" ? raw.gamesPlayed : 0,
    wins: typeof raw.wins === "number" ? raw.wins : 0,
  };
}

export async function requestLeaderboard(accessToken?: string): Promise<LeaderboardSummary | null> {
  const resp = await apiFetch("/api/leaderboard", { headers: authHeaders(accessToken) });
  return resp.ok ? resp.json() : null;
}
