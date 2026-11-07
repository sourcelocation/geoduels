import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";
import type {
  CompactPlayer,
  FriendRequest,
  PartyInvitation,
  SocialSettings,
} from "../types";

async function socialFetch<T>(
  token: string,
  path: string,
  init?: RequestInit,
): Promise<T> {
  const response = await apiFetch(path, {
    ...init,
    headers: mergeHeaders(
      init?.body ? { "Content-Type": "application/json" } : undefined,
      authHeaders(token),
      init?.headers,
    ),
  });
  if (!response.ok) throw new Error(await readError(response, "Social action failed"));
  if (response.status === 204) return undefined as T;
  return response.json();
}

export type FriendsPage = {
  friends: CompactPlayer[];
  requests: { incoming: FriendRequest[]; outgoing: FriendRequest[] };
  recentPlayers: CompactPlayer[];
};

export const socialClient = {
  friendsPage: (token: string, partyId?: string) =>
    socialFetch<FriendsPage>(
      token,
      partyId
        ? `/api/me/friends-page?partyId=${encodeURIComponent(partyId)}`
        : "/api/me/friends-page",
    ),
  search: (token: string, query: string) =>
    socialFetch<{ players: CompactPlayer[] }>(
      token,
      `/api/player-search?q=${encodeURIComponent(query)}`,
    ),
  settings: (token: string) =>
    socialFetch<SocialSettings>(token, "/api/me/social-settings"),
  updateSettings: (token: string, settings: SocialSettings) =>
    socialFetch<SocialSettings>(token, "/api/me/social-settings", {
      method: "PATCH",
      body: JSON.stringify(settings),
    }),
  sendRequest: (token: string, userId: string) =>
    socialFetch(token, "/api/friend-requests", {
      method: "POST",
      body: JSON.stringify({ userId }),
    }),
  respondRequest: (
    token: string,
    requestId: string,
    action: "accept" | "decline" | "cancel",
  ) => socialFetch(token, `/api/friend-requests/${requestId}/${action}`, { method: "POST" }),
  removeFriend: (token: string, userId: string) =>
    socialFetch(token, `/api/friends/${userId}`, { method: "DELETE" }),
  block: (token: string, userId: string) =>
    socialFetch(token, `/api/blocks/${userId}`, { method: "POST" }),
  createCode: (token: string) =>
    socialFetch<{ code: string; expiresAt: string }>(token, "/api/me/friend-code", {
      method: "POST",
    }),
  resolveCode: (token: string, code: string) =>
    socialFetch<CompactPlayer>(token, `/api/friend-codes/${encodeURIComponent(code)}`),
  requestByCode: (token: string, code: string) =>
    socialFetch(token, `/api/friend-codes/${encodeURIComponent(code)}/request`, {
      method: "POST",
    }),
  inviteToParty: (token: string, partyId: string, userId: string) =>
    socialFetch<PartyInvitation>(token, `/api/parties/${partyId}/invitations`, {
      method: "POST",
      body: JSON.stringify({ userId }),
    }),
  createPartyAndInvite: (token: string, userId: string) =>
    socialFetch<{ invitation: PartyInvitation; party: { inviteCode: string } }>(
      token,
      "/api/party-invitations",
      { method: "POST", body: JSON.stringify({ userId }) },
    ),
  relationship: (token: string, nickname: string) =>
    socialFetch<{ state: CompactPlayer["relationship"]; requestId?: string }>(
      token,
      `/api/players/${encodeURIComponent(nickname)}/relationship`,
    ),
};
