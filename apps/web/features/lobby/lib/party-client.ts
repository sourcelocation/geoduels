import type { RuntimeConfig } from "../../../lib/runtime-config";
import { normalizeHTTPBase, normalizeWSBase } from "../../../lib/runtime-config";
import type { AuthSessionSnapshot } from "../../auth/session";
import type { MatchConfig } from "../../matchmaking/lib/queue-client";
import type { PlayerBadgeInfo } from "../../../components/ui/PlayerBadge";

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
  state: "open" | "in_match" | "started" | "closed" | "expired";
  mode: PartyMode;
  mapScope: string;
  mapName?: string;
  mapLocationCount?: number;
  config?: MatchConfig;
  activeMatchId?: string;
  lastMatchId?: string;
  startedMatchId?: string;
  members: PartyMember[];
};

export type PartyPatch = {
  revision: number;
  state?: PartySnapshot["state"];
  ownerUserId?: string;
  mode?: PartyMode;
  config?: MatchConfig;
  activeMatchId?: string;
  lastMatchId?: string;
  startedMatchId?: string;
  upsertMembers?: PartyMember[];
  removeMemberIds?: string[];
};

export type PartyAssignment = {
  matchId: string;
  mode?: string;
  config?: MatchConfig;
  node: string;
  ticket: string;
  wsPath: string;
  sourcePartyId?: string;
  sourcePartyInviteCode?: string;
};

export type PartyEvent =
  | { type: "party_snapshot"; party: PartySnapshot }
  | { type: "party_patch"; patch: PartyPatch }
  | { type: "match_assigned"; assignment: PartyAssignment }
  | { type: "party_error"; message: string };

function authHeaders(accessToken: string) {
  return { Authorization: `Bearer ${accessToken}` };
}

function partyHTTPBase(config: RuntimeConfig) {
  return normalizeHTTPBase(config.queueURL).replace(/\/$/, "");
}

function partyWSTarget(config: RuntimeConfig, partyId: string, accessToken: string) {
  return `${normalizeWSBase(config.queueURL).replace(/\/$/, "")}/parties/${encodeURIComponent(partyId)}/ws?accessToken=${encodeURIComponent(accessToken)}`;
}

export async function createParty(config: RuntimeConfig, accessToken: string, mode: PartyMode = "duel", matchConfig?: MatchConfig): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties`, {
    method: "POST",
    headers: { ...authHeaders(accessToken), "Content-Type": "application/json" },
    body: JSON.stringify({ mode, config: matchConfig }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Party unavailable");
  return resp.json();
}

export async function updatePartySettings(config: RuntimeConfig, partyId: string, accessToken: string, matchConfig: MatchConfig, mode?: PartyMode): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/settings`, {
    method: "PATCH",
    headers: { ...authHeaders(accessToken), "Content-Type": "application/json" },
    body: JSON.stringify({ config: matchConfig, mode }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not update party settings");
  return resp.json();
}

export async function updatePartyTeam(config: RuntimeConfig, partyId: string, accessToken: string, teamId: PartyTeamId): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/team`, {
    method: "PATCH",
    headers: { ...authHeaders(accessToken), "Content-Type": "application/json" },
    body: JSON.stringify({ teamId }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not switch team");
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
    activeMatchId: patch.activeMatchId ?? party.activeMatchId,
    lastMatchId: patch.lastMatchId ?? party.lastMatchId,
    startedMatchId: patch.startedMatchId ?? party.startedMatchId,
    members: party.members,
  };
  if (patch.upsertMembers?.length || patch.removeMemberIds?.length) {
    const removed = new Set(patch.removeMemberIds || []);
    const members = new Map(next.members.filter((member) => !removed.has(member.userId)).map((member) => [member.userId, member]));
    for (const member of patch.upsertMembers || []) {
      members.set(member.userId, { ...(members.get(member.userId) || {} as PartyMember), ...member });
    }
    next.members = Array.from(members.values());
  }
  return next;
}

export async function fetchParty(config: RuntimeConfig, code: string, signal?: AbortSignal): Promise<PartySnapshot | null> {
  const target = `${partyHTTPBase(config)}/parties/${encodeURIComponent(code)}`;
  const resp = signal ? await fetch(target, { signal }) : await fetch(target);
  if (resp.status === 404) return null;
  if (!resp.ok) throw new Error("Party unavailable");
  return resp.json();
}

export async function touchPartyPresence(config: RuntimeConfig, partyId: string, accessToken: string, signal?: AbortSignal): Promise<void> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/presence`, {
    method: "POST",
    headers: authHeaders(accessToken),
    signal,
  });
  if (!resp.ok) throw new Error("Party presence unavailable");
}

export async function joinParty(config: RuntimeConfig, code: string, accessToken: string): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(code)}/join`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not join party");
  return resp.json();
}

export async function leaveParty(config: RuntimeConfig, partyId: string, accessToken: string): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/leave`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not leave party");
  return resp.json();
}

export async function kickPartyMember(config: RuntimeConfig, partyId: string, accessToken: string, userId: string): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/kick`, {
    method: "POST",
    headers: { ...authHeaders(accessToken), "Content-Type": "application/json" },
    body: JSON.stringify({ userId }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not kick player");
  return resp.json();
}

export async function transferPartyOwner(config: RuntimeConfig, partyId: string, accessToken: string, userId: string): Promise<PartySnapshot> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/transfer-owner`, {
    method: "POST",
    headers: { ...authHeaders(accessToken), "Content-Type": "application/json" },
    body: JSON.stringify({ userId }),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not transfer leader");
  return resp.json();
}

export async function startParty(config: RuntimeConfig, partyId: string, accessToken: string): Promise<PartyAssignment> {
  const resp = await fetch(`${partyHTTPBase(config)}/parties/${encodeURIComponent(partyId)}/start`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) throw new Error((await resp.text()) || "Could not start party");
  const data = await resp.json();
  return data.assignment;
}

export async function streamParty(
  config: RuntimeConfig,
  session: AuthSessionSnapshot,
  partyId: string,
  signal: AbortSignal,
  onEvent: (event: PartyEvent) => void,
) {
  const target = partyWSTarget(config, partyId, session.accessToken);
  await new Promise<void>((resolve, reject) => {
    let settled = false;
    const ws = new WebSocket(target);
    const cleanup = () => signal.removeEventListener("abort", abort);
    const settle = (fn: typeof resolve | typeof reject, value?: unknown) => {
      if (settled) return;
      settled = true;
      cleanup();
      fn(value as never);
    };
    const abort = () => {
      ws.close();
      settle(reject, new DOMException("Aborted", "AbortError"));
    };
    signal.addEventListener("abort", abort, { once: true });
    ws.onerror = () => settle(reject, new Error("Party connection failed"));
    ws.onclose = () => {
      if (signal.aborted) settle(reject, new DOMException("Aborted", "AbortError"));
      else settle(resolve);
    };
    ws.onmessage = (evt) => {
      let msg: any;
      try {
        msg = JSON.parse(String(evt.data));
      } catch {
        settle(reject, new Error("Party connection failed"));
        return;
      }
      const payload = msg?.payload ?? {};
      if (msg?.type === "party_snapshot") onEvent({ type: "party_snapshot", party: payload as PartySnapshot });
      if (msg?.type === "party_patch") onEvent({ type: "party_patch", patch: payload as PartyPatch });
      if (msg?.type === "match_assigned") onEvent({ type: "match_assigned", assignment: payload as PartyAssignment });
      if (msg?.type === "party_error") onEvent({ type: "party_error", message: payload?.message || "Party unavailable" });
    };
  });
}
