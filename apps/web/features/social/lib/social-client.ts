import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import type {
  CompactPlayer,
  FriendRequest,
  PartyInvitation,
  SocialSettings,
} from "../types";

async function socialFetch<T>(
  config: RuntimeConfig,
  token: string,
  path: string,
  init?: RequestInit,
): Promise<T> {
  const response = await apiFetch(config, path, {
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
  friendsPage: (config: RuntimeConfig, token: string, partyId?: string) =>
    socialFetch<FriendsPage>(
      config,
      token,
      partyId
        ? `/v1/me/friends-page?partyId=${encodeURIComponent(partyId)}`
        : "/v1/me/friends-page",
    ),
  search: (config: RuntimeConfig, token: string, query: string) =>
    socialFetch<{ players: CompactPlayer[] }>(
      config,
      token,
      `/v1/player-search?q=${encodeURIComponent(query)}`,
    ),
  settings: (config: RuntimeConfig, token: string) =>
    socialFetch<SocialSettings>(config, token, "/v1/me/social-settings"),
  updateSettings: (config: RuntimeConfig, token: string, settings: SocialSettings) =>
    socialFetch<SocialSettings>(config, token, "/v1/me/social-settings", {
      method: "PATCH",
      body: JSON.stringify(settings),
    }),
  sendRequest: (config: RuntimeConfig, token: string, userId: string) =>
    socialFetch(config, token, "/v1/friend-requests", {
      method: "POST",
      body: JSON.stringify({ userId }),
    }),
  respondRequest: (
    config: RuntimeConfig,
    token: string,
    requestId: string,
    action: "accept" | "decline" | "cancel",
  ) => socialFetch(config, token, `/v1/friend-requests/${requestId}/${action}`, { method: "POST" }),
  removeFriend: (config: RuntimeConfig, token: string, userId: string) =>
    socialFetch(config, token, `/v1/friends/${userId}`, { method: "DELETE" }),
  block: (config: RuntimeConfig, token: string, userId: string) =>
    socialFetch(config, token, `/v1/blocks/${userId}`, { method: "POST" }),
  createCode: (config: RuntimeConfig, token: string) =>
    socialFetch<{ code: string; expiresAt: string }>(config, token, "/v1/me/friend-code", {
      method: "POST",
    }),
  resolveCode: (config: RuntimeConfig, token: string, code: string) =>
    socialFetch<CompactPlayer>(config, token, `/v1/friend-codes/${encodeURIComponent(code)}`),
  requestByCode: (config: RuntimeConfig, token: string, code: string) =>
    socialFetch(config, token, `/v1/friend-codes/${encodeURIComponent(code)}/request`, {
      method: "POST",
    }),
  inviteToParty: (config: RuntimeConfig, token: string, partyId: string, userId: string) =>
    socialFetch<PartyInvitation>(config, token, `/v1/parties/${partyId}/invitations`, {
      method: "POST",
      body: JSON.stringify({ userId }),
    }),
  createPartyAndInvite: (config: RuntimeConfig, token: string, userId: string) =>
    socialFetch<{ invitation: PartyInvitation; party: { inviteCode: string } }>(
      config,
      token,
      "/v1/party-invitations",
      { method: "POST", body: JSON.stringify({ userId }) },
    ),
  respondPartyInvite: (
    config: RuntimeConfig,
    token: string,
    invitationId: string,
    action: "accept" | "decline",
  ) => socialFetch<PartyInvitation>(
    config,
    token,
    `/v1/party-invitations/${invitationId}/${action}`,
    { method: "POST" },
  ),
  relationship: (config: RuntimeConfig, token: string, nickname: string) =>
    socialFetch<{ state: CompactPlayer["relationship"]; requestId?: string }>(
      config,
      token,
      `/v1/players/${encodeURIComponent(nickname)}/relationship`,
    ),
};
