import type { RuntimeConfig } from "../../../lib/runtime-config";
import { apiFetch } from "../../../lib/http";
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

export type PartyPatch = {
  revision: number;
  state?: PartySnapshot["state"];
  ownerUserId?: string;
  mode?: PartyMode;
  mapScope?: string;
  mapName?: string;
  mapLocationCount?: number;
  config?: MatchConfig;
  activeMatchId?: string;
  lastMatchId?: string;
  upsertMembers?: PartyMember[];
  removeMemberIds?: string[];
};

export type PartyEvent =
  | { type: "party_snapshot"; party: PartySnapshot }
  | { type: "party_patch"; patch: PartyPatch }
  | { type: "match_found"; matchId: string }
  | { type: "party_error"; message: string };

function authHeaders(accessToken: string) {
  return { Authorization: `Bearer ${accessToken}` };
}

export async function createParty(config: RuntimeConfig, accessToken: string, mode: PartyMode = "duel", matchConfig?: MatchConfig): Promise<Pick<PartySnapshot, "id" | "inviteCode">> {
  const resp = await apiFetch(config, "/api/v2/parties", {
    method: "POST",
    headers: { ...authHeaders(accessToken), "Content-Type": "application/json" },
    body: JSON.stringify({ mode, config: matchConfig }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Party unavailable");
  return resp.json();
}

export function applyPartyPatch(party: PartySnapshot | null, patch: PartyPatch): PartySnapshot | null {
  if (!party) return party;
  const next: PartySnapshot = {
    ...party,
    state: patch.state ?? party.state,
    ownerUserId: patch.ownerUserId ?? party.ownerUserId,
    mode: patch.mode ?? party.mode,
    config: patch.config ?? party.config,
    mapScope: patch.mapScope ?? party.mapScope,
    mapName: patch.mapName ?? party.mapName,
    mapLocationCount: patch.mapLocationCount ?? party.mapLocationCount,
    activeMatchId: patch.activeMatchId ?? party.activeMatchId,
    lastMatchId: patch.lastMatchId ?? party.lastMatchId,
    members: party.members,
  };
  if (patch.upsertMembers?.length || patch.removeMemberIds?.length) {
    const removed = new Set(patch.removeMemberIds || []);
    const members = new Map(next.members.filter((member) => !removed.has(member.userId)).map((member) => [member.userId, member]));
    for (const member of patch.upsertMembers || []) {
      members.set(member.userId, member);
    }
    next.members = Array.from(members.values());
  }
  return next;
}

export async function joinParty(config: RuntimeConfig, code: string, accessToken: string): Promise<Pick<PartySnapshot, "id" | "inviteCode">> {
  const resp = await apiFetch(config, `/api/v2/parties/${encodeURIComponent(code)}/join`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not join party");
  return resp.json();
}
