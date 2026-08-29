import type { RuntimeConfig } from "../../../lib/runtime-config";
import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";
import type { LeaderboardSummary } from "../controllers/session-controller";

export type OAuthIntent = "signin" | "link" | "upgrade_guest";

export type AuthSessionPayload = {
  accessToken?: string;
  nicknameRequired?: boolean;
  authMigrationRequired?: boolean;
  recoveryAvailable?: boolean;
  linkedProviders?: string[];
  canPlay?: boolean;
  suggestedNickname?: string;
  user?: {
    id?: string;
    email?: string;
    display_name?: string;
    avatar_url?: string;
    isGuest?: boolean;
    isAdmin?: boolean;
    isModerator?: boolean;
  };
};

export type BootstrapViewer = {
  id: string;
  email?: string;
  displayName: string;
  avatarUrl?: string;
  accountType: "guest" | "registered";
  mmr: number;
  ratingRd?: number;
  gamesPlayed: number;
  wins: number;
  rankedGamesPlayed: number;
  rankedWins: number;
  isAdmin?: boolean;
  isModerator?: boolean;
  isBanned?: boolean;
  banReason?: string;
  linkedProviders?: string[];
  selectedBadge?: unknown | null;
};

export type AppBootstrapPayload = {
  version: 1;
  auth: AuthSessionPayload | null;
  viewer: BootstrapViewer | null;
  preferences: { revision: number; value: unknown } | null;
  activity: {
    activeMatch: { status: "match"; matchId: string; mode?: string } | null;
    notifications: UserNotification[];
  };
  global: {
    onlinePlayers: number;
    maintenance: {
      phase: "normal" | "warning" | "active";
      startsAt?: string;
      endsAt?: string;
      queuePaused?: boolean;
      playPaused?: boolean;
      message?: string;
    };
  };
};

export class AuthSessionError extends Error {
  constructor(public readonly status: number, message: string) { super(message); }
}

export async function requestBootstrap(config: RuntimeConfig): Promise<AppBootstrapPayload> {
  const resp = await apiFetch(config, "/v1/bootstrap", {
    credentials: "include",
  });
  if (!resp.ok) {
    throw new AuthSessionError(resp.status, await readError(resp, "Application bootstrap failed"));
  }
  return resp.json();
}

export async function requestGuestSession(
  config: RuntimeConfig,
  turnstileToken?: string,
): Promise<AuthSessionPayload> {
  const resp = await apiFetch(config, "/v1/auth/guest", {
    method: "POST",
    credentials: "include",
    headers: turnstileToken
      ? { "Content-Type": "application/json" }
      : undefined,
    body: turnstileToken
      ? JSON.stringify({ turnstileToken })
      : undefined,
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Guest login failed"));
  }
  return resp.json();
}

export async function requestRefreshSession(config: RuntimeConfig): Promise<AuthSessionPayload | null> {
  const resp = await apiFetch(config, "/v1/auth/refresh", {
    method: "POST",
    credentials: "include",
  });
  if (resp.status === 401) return null;
  if (!resp.ok) {
    throw new AuthSessionError(resp.status, await readError(resp, "Session refresh failed"));
  }
  return resp.json();
}

export type UserNotification = {
  id: number;
  type: string;
  payload: {
    badge?: {
      id: string;
      kind: string;
      label: string;
      description: string;
      imageUrl: string;
      rarity?: string;
      seasonId?: string;
      rank?: number;
      owned?: boolean;
      unobtainable?: boolean;
    };
    refundDelta?: number;
    matchId?: string;
    requestId?: string;
    invitationId?: string;
    actorUserId?: string;
    cheaterUserId?: string;
    mmrBefore?: number;
    mmrAfter?: number;
  };
  readAt?: string;
  createdAt: string;
};

export async function requestUserNotifications(
  config: RuntimeConfig,
  accessToken: string,
  filter: "unread" | "all" = "unread",
  options?: { limit?: number; beforeId?: number },
): Promise<{ notifications: UserNotification[] }> {
  const query = new URLSearchParams();
  if (filter === "all") query.set("filter", "all");
  if (options?.limit) query.set("limit", String(options.limit));
  if (options?.beforeId) query.set("beforeId", String(options.beforeId));
  const suffix = query.size ? `?${query.toString()}` : "";
  const resp = await apiFetch(config, `/v1/me/notifications${suffix}`, {
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) {
    return { notifications: [] };
  }
  return resp.json();
}

export async function markAllUserNotificationsRead(
  config: RuntimeConfig,
  accessToken: string,
) {
  await apiFetch(config, "/v1/me/notifications/read-all", {
    method: "POST",
    headers: authHeaders(accessToken),
  });
}

export async function markUserNotificationRead(
  config: RuntimeConfig,
  accessToken: string,
  notificationId: number,
) {
  await apiFetch(
    config,
    `/v1/me/notifications/${encodeURIComponent(notificationId)}/read`,
    {
      method: "POST",
      headers: authHeaders(accessToken),
    },
  );
}

export async function requestSupportDonation(
  config: RuntimeConfig,
  accessToken: string,
): Promise<{ donationUrl: string }> {
  const resp = await apiFetch(config, "/v1/support/donate", {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Could not start donation"));
  }
  return resp.json();
}

export async function requestLeaderboard(
  config: RuntimeConfig,
  accessToken?: string,
): Promise<LeaderboardSummary | null> {
  const resp = await apiFetch(config, "/v1/leaderboard", {
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) {
    return null;
  }
  return resp.json();
}

export async function requestLogout(config: RuntimeConfig) {
  await apiFetch(config, "/v1/auth/logout", {
    method: "POST",
    credentials: "include",
  });
}

export async function requestUpdateNickname(
  config: RuntimeConfig,
  accessToken: string,
  nickname: string,
) {
  const resp = await apiFetch(config, "/v1/me/nickname", {
    method: "PUT",
    credentials: "include",
    headers: mergeHeaders({
      "content-type": "application/json",
    }, authHeaders(accessToken)),
    body: JSON.stringify({ nickname }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to save nickname"));
  }
  return resp.json();
}

export async function requestUpdateSelectedBadge(
  config: RuntimeConfig,
  accessToken: string,
  badgeId: string,
) {
  const resp = await apiFetch(config, "/v1/me/badge", {
    method: "PATCH",
    credentials: "include",
    headers: mergeHeaders({
      "content-type": "application/json",
    }, authHeaders(accessToken)),
    body: JSON.stringify({ badgeId }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to update badge"));
  }
  return resp.json();
}

export async function requestGoogleStart(
  config: RuntimeConfig,
  params: {
    intent?: OAuthIntent;
    accessToken?: string;
    returnTo?: string;
  } = {},
) {
  const resp = await apiFetch(config, "/v1/auth/google/start", {
    method: "POST",
    credentials: "include",
    headers: mergeHeaders({
      "content-type": "application/json",
    }, authHeaders(params.accessToken)),
    body: JSON.stringify({
      intent: params.intent || "signin",
      returnTo: params.returnTo,
    }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to start Google sign-in"));
  }
  return resp.json();
}

export async function requestDiscordStart(
  config: RuntimeConfig,
  params: {
    intent?: OAuthIntent;
    accessToken?: string;
    returnTo?: string;
  } = {},
) {
  const resp = await apiFetch(config, "/v1/auth/discord/start", {
    method: "POST",
    credentials: "include",
    headers: mergeHeaders({
      "content-type": "application/json",
    }, authHeaders(params.accessToken)),
    body: JSON.stringify({
      intent: params.intent || "signin",
      returnTo: params.returnTo,
    }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to start Discord sign-in"));
  }
  return resp.json();
}

export async function requestUnlinkAuthProvider(
  config: RuntimeConfig,
  accessToken: string,
  provider: "google" | "discord",
) {
  const resp = await apiFetch(
    config,
    `/v1/me/auth-providers/${encodeURIComponent(provider)}`,
    {
      method: "DELETE",
      credentials: "include",
      headers: authHeaders(accessToken),
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to unlink sign-in method"));
  }
  return resp.json();
}

export async function requestDeleteAccount(
  config: RuntimeConfig,
  accessToken: string,
) {
  const resp = await apiFetch(config, "/v1/me", {
    method: "DELETE",
    credentials: "include",
    headers: mergeHeaders({
      "content-type": "application/json",
    }, authHeaders(accessToken)),
    body: JSON.stringify({ confirm: "DELETE" }),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to delete account"));
  }
}

export async function requestLobbyChangelog(config: RuntimeConfig) {
  const resp = await apiFetch(config, "/v1/content/lobby-changelog");
  if (!resp.ok) {
    return null;
  }
  return resp.json() as Promise<{
    eyebrow?: string;
    title?: string;
    markdown?: string;
    slug?: string;
    updatedAt?: string;
  }>;
}

export async function requestMatchReport(
  config: RuntimeConfig,
  accessToken: string,
  matchId: string,
  reportedUserId: string,
  category = "cheating",
  reason = "",
) {
  const resp = await apiFetch(
    config,
    `/v1/matches/${encodeURIComponent(matchId)}/reports`,
    {
      method: "POST",
      headers: mergeHeaders({
        "content-type": "application/json",
      }, authHeaders(accessToken)),
      body: JSON.stringify({ reportedUserId, category, reason }),
    },
  );
  if (!resp.ok) {
    throw new Error(await readError(resp, "Failed to send report"));
  }
}
