import type { AuthSessionSnapshot } from "../../auth/session";
import type { HomeModel } from "./types";

type AuthResponseUser = {
  id?: string;
  email?: string;
  display_name?: string;
  avatar_url?: string;
  isGuest?: boolean;
  isAdmin?: boolean;
  isModerator?: boolean;
};

export type AuthResponse = {
  accessToken?: string;
  nicknameRequired?: boolean;
  authMigrationRequired?: boolean;
  recoveryAvailable?: boolean;
  linkedProviders?: string[];
  canPlay?: boolean;
  suggestedNickname?: string;
  authURL?: string;
  user?: AuthResponseUser;
};

export type GuestVerificationView = HomeModel["view"]["overlays"]["guestVerification"];

export function currentReturnTo() {
  if (typeof window === "undefined") return "/";
  const path = `${window.location.pathname}${window.location.search}${window.location.hash}`;
  return path.startsWith("/") ? path : "/";
}

/** @deprecated Prefer clearAuthCallbackParams from features/auth/lib/auth-callback. */
export { clearAuthCallbackParams as clearGoogleAuthParams } from "../../auth/lib/auth-callback";

export function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

export function buildSessionFromAuthResponse(
  data: AuthResponse,
  fallback: { userId: string; nicknameInput: string },
): AuthSessionSnapshot {
  return {
    userId:
      typeof data.user?.id === "string" && data.user.id
        ? data.user.id
        : fallback.userId,
    accessToken: data.accessToken || "",
    nicknameRequired: !!data.nicknameRequired,
    authMigrationRequired: !!data.authMigrationRequired,
    recoveryAvailable: !!data.recoveryAvailable,
    linkedProviders: Array.isArray(data.linkedProviders)
      ? data.linkedProviders.filter((provider): provider is string => typeof provider === "string")
      : [],
    canPlay:
      typeof data.canPlay === "boolean"
        ? data.canPlay
        : !data.nicknameRequired && !data.authMigrationRequired,
    nicknameInput: data.suggestedNickname || fallback.nicknameInput,
  };
}
