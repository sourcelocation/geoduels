import type { StaffRole } from "../roles";
import { apiFetch, authHeaders, mergeHeaders, readError } from "../../../lib/http";

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
    roles?: StaffRole[];
    isGuest?: boolean;
    isAdmin?: boolean;
    isModerator?: boolean;
  };
};

export type BootstrapViewer = {
  roles?: StaffRole[];
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
  badges?: unknown[] | null;
  selectedBadge?: unknown | null;
};

export type AppBootstrapPayload = {
  auth: AuthSessionPayload | null;
  viewer: BootstrapViewer | null;
  preferences: { revision: number; value: unknown } | null;
  client: {
    googleClientId: string;
    discordClientId: string;
    turnstileSiteKey: string;
    googleEmbedKey: string;
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

export async function requestBootstrap(): Promise<AppBootstrapPayload> {
  const resp = await apiFetch("/api/bootstrap", { credentials: "include" });
  if (!resp.ok) {
    throw new AuthSessionError(resp.status, await readError(resp, "Application bootstrap failed"));
  }
  return resp.json();
}

export async function requestGuestSession(
  turnstileToken?: string,
): Promise<AuthSessionPayload> {
  const resp = await apiFetch("/api/auth/guest", {
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

export async function requestRefreshSession(): Promise<AuthSessionPayload | null> {
  const resp = await apiFetch("/api/auth/refresh", {
    method: "POST",
    credentials: "include",
  });
  if (resp.status === 401) return null;
  if (!resp.ok) {
    throw new AuthSessionError(resp.status, await readError(resp, "Session refresh failed"));
  }
  return resp.json();
}


export async function requestSupportDonation(
  accessToken: string,
): Promise<{ donationUrl: string }> {
  const resp = await apiFetch("/api/support/donate", {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  if (!resp.ok) {
    throw new Error(await readError(resp, "Could not start donation"));
  }
  return resp.json();
}


export async function requestLogout() {
  await apiFetch("/api/auth/logout", {
    method: "POST",
    credentials: "include",
  });
}

export async function requestUpdateNickname(
  accessToken: string,
  nickname: string,
) {
  const resp = await apiFetch("/api/me/nickname", {
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
  accessToken: string,
  badgeId: string,
) {
  const resp = await apiFetch("/api/me/badge", {
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
  params: {
    intent?: OAuthIntent;
    accessToken?: string;
    returnTo?: string;
  } = {},
) {
  const resp = await apiFetch("/api/auth/google/start", {
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
  params: {
    intent?: OAuthIntent;
    accessToken?: string;
    returnTo?: string;
  } = {},
) {
  const resp = await apiFetch("/api/auth/discord/start", {
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
  accessToken: string,
  provider: "google" | "discord",
) {
  const resp = await apiFetch(
    `/api/me/auth-providers/${encodeURIComponent(provider)}`,
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
  accessToken: string,
) {
  const resp = await apiFetch("/api/me", {
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

export async function requestLobbyChangelog() {
  const resp = await apiFetch("/api/content/lobby-changelog");
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
  accessToken: string,
  matchId: string,
  reportedUserId: string,
  category = "cheating",
  reason = "",
) {
  const resp = await apiFetch(
    `/api/matches/${encodeURIComponent(matchId)}/reports`,
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
