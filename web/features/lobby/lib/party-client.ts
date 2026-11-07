
import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import type { PlayerBadgeInfo } from "../../players/components/PlayerBadge";

export type PartyMode = "duel" | "team_duel" | "free_for_all";
export type PartyTeamId = "a" | "b";

export type PartyMember = {
  userId: string;
  displayName: string;
  avatarUrl?: string;
  isGuest?: boolean;
  isAdmin?: boolean;
  selectedBadge?: PlayerBadgeInfo | null;
  teamId?: PartyTeamId | "";
  role: string;
  ready?: boolean;
  connected?: boolean;
  presenceStatus?: "online" | "away" | "offline";
  inActiveMatch?: boolean;
};

export type PartySnapshot = {
  id: string;
  inviteCode: string;
  // Clients keep the highest they have seen, so a late update never undoes a newer one.
  revision: number;
  ownerUserId: string;
  state: "open" | "in_match" | "closed" | "expired";
  mode: PartyMode;
  mapScope: string;
  mapName?: string;
  mapLocationCount?: number;
  config?: MatchConfig;
  activeMatchId?: string;
  lastMatchId?: string;
  members: PartyMember[];
};

async function partyRequest(token: string, path: string, init: RequestInit, failure: string): Promise<PartySnapshot | null> {
  const response = await apiFetch(path, {
    ...init,
    headers: mergeHeaders(authHeaders(token), init.body ? { "Content-Type": "application/json" } : undefined),
  });
  if (!response.ok) throw new Error(await readError(response, failure));
  return response.status === 204 ? null : response.json();
}

export async function createParty(token: string, mode: PartyMode = "duel", config?: MatchConfig) {
  const party = await partyRequest(token, "/api/v2/parties", { method: "POST", body: JSON.stringify({ mode, config }) }, "Party unavailable");
  if (!party) throw new Error("Party unavailable");
  return party;
}

export async function joinParty(code: string, token: string) {
  const party = await partyRequest(token, `/api/v2/parties/${encodeURIComponent(code)}/join`, { method: "POST" }, "Could not join party");
  if (!party) throw new Error("Could not join party");
  return party;
}

/** Something a member does to their party. The answer is the party as it now is, if it still exists. */
export function partyCommand(token: string, partyId: string, action: string, init: { method: "POST" | "PUT"; body?: unknown }, failure: string) {
  return partyRequest(
    token,
    `/api/v2/parties/${encodeURIComponent(partyId)}/${action}`,
    { method: init.method, body: init.body === undefined ? undefined : JSON.stringify(init.body) },
    failure,
  );
}
