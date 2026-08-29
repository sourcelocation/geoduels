import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { getRuntimeConfig } from "../../../lib/runtime-config";
import {
  requestDiscordStart,
  requestGoogleStart,
  type OAuthIntent,
} from "../lib/auth-client";
import { clearAuthCallbackParams, readAuthCallback } from "../lib/auth-callback";
import type { AppBootstrapPayload, AuthSessionPayload } from "../lib/auth-client";
import type { PlayerBadgeInfo } from "../../players/components/PlayerBadge";
import { getAuthGateway } from "../auth-gateway";
import { UserLiveProvider } from "./UserLiveProvider";
import AppModalShell from "../../../components/ui/AppModalShell";
import { Button } from "../../../components/ui/button";

export type AuthStatus = "bootstrapping" | "anonymous" | "guest" | "registered";

export type AuthState = {
  status: AuthStatus;
  session: AuthSessionPayload | null;
  accessToken: string;
  userId: string;
  isGuest: boolean;
  isRegistered: boolean;
  isAdmin: boolean;
  isModerator: boolean;
  displayName: string;
  avatarUrl: string;
  email: string;
  mmr?: number;
  selectedBadge?: PlayerBadgeInfo | null;
  canPlayUnranked: boolean;
  canPlayRanked: boolean;
  canUseSocial: boolean;
  canManageMaps: boolean;
  bootstrap: AppBootstrapPayload | null;
};

export type AuthActions = {
  signInOpen: boolean;
  authLoading: boolean;
  authError: string;
  googleEnabled: boolean;
  discordEnabled: boolean;
  openSignIn: () => void;
  closeSignIn: () => void;
  startProvider: (provider: "google" | "discord") => Promise<void>;
  devLogin: () => Promise<void>;
};

const anonymousState: AuthState = {
  status: "anonymous",
  session: null,
  accessToken: "",
  userId: "",
  isGuest: false,
  isRegistered: false,
  isAdmin: false,
  isModerator: false,
  displayName: "",
  avatarUrl: "",
  email: "",
  canPlayUnranked: false,
  canPlayRanked: false,
  canUseSocial: false,
  canManageMaps: false,
  bootstrap: null,
};

const AuthContext = createContext<AuthState>(anonymousState);
const defaultAuthActions: AuthActions = {
  signInOpen: false,
  authLoading: false,
  authError: "",
  googleEnabled: false,
  discordEnabled: false,
  openSignIn: () => undefined,
  closeSignIn: () => undefined,
  startProvider: async () => undefined,
  devLogin: async () => undefined,
};
const AuthActionsContext = createContext<AuthActions>(defaultAuthActions);

export function AuthStateProvider({ value, children }: { value: AuthState; children: ReactNode }) {
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

function profileValue(value: unknown, key: string) {
  return value && typeof value === "object" && key in value
    ? (value as Record<string, unknown>)[key]
    : undefined;
}

function stringValue(value: unknown, key: string) {
  const result = profileValue(value, key);
  return typeof result === "string" ? result : "";
}

export function deriveAuthState(
  session: AuthSessionPayload | null | undefined,
  isFetched: boolean,
  profile?: Record<string, unknown> | null,
  bootstrap: AppBootstrapPayload | null = null,
): AuthState {
  if (!isFetched) return { ...anonymousState, status: "bootstrapping", bootstrap };
  const token = session?.accessToken || "";
  if (!session?.user?.id || !token) return { ...anonymousState, bootstrap };
  const user = session.user;
  const isGuest = !!user.isGuest;
  const isRegistered = !isGuest;
  const viewer = bootstrap?.viewer;
  const canPlay = session.canPlay !== false && !session.nicknameRequired && !session.authMigrationRequired;
  const displayName =
    viewer?.displayName || stringValue(profile, "display_name") ||
    user.display_name || user.email || (isGuest ? "Guest" : "Player");
  const avatarUrl = viewer?.avatarUrl || stringValue(profile, "avatar_url") || user.avatar_url || "";
  const profileMmr = viewer?.mmr ?? profileValue(profile, "mmr");
  return {
    status: isGuest ? "guest" : "registered",
    session,
    accessToken: token,
    userId: user.id || "",
    isGuest,
    isRegistered,
    isAdmin: !!(viewer?.isAdmin ?? profileValue(profile, "isAdmin") ?? user.isAdmin),
    isModerator: !!(viewer?.isModerator ?? profileValue(profile, "isModerator") ?? user.isModerator),
    displayName,
    avatarUrl,
    email: user.email || stringValue(profile, "email"),
    mmr: typeof profileMmr === "number" ? profileMmr : undefined,
    selectedBadge: (viewer?.selectedBadge as PlayerBadgeInfo | null | undefined) || (profileValue(profile, "selectedBadge") as PlayerBadgeInfo | null | undefined) || null,
    canPlayUnranked: canPlay,
    canPlayRanked: isRegistered && canPlay,
    canUseSocial: isRegistered,
    canManageMaps: isRegistered,
    bootstrap,
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const config = getRuntimeConfig();
  const gateway = getAuthGateway(config);
  const session = useSyncExternalStore(gateway.subscribe.bind(gateway), gateway.getPayload.bind(gateway), gateway.getPayload.bind(gateway));
  const restored = useSyncExternalStore(gateway.subscribe.bind(gateway), gateway.isRestored.bind(gateway), gateway.isRestored.bind(gateway));
  useSyncExternalStore(gateway.subscribe.bind(gateway), gateway.getBootstrapEpoch.bind(gateway), gateway.getBootstrapEpoch.bind(gateway));
  const bootstrap = restored ? gateway.getBootstrapPayload() : null;

  useEffect(() => { void gateway.bootstrap().catch(() => undefined); }, [gateway]);

  const value = useMemo(
    () => deriveAuthState(session, restored, null, bootstrap),
    [bootstrap, restored, session],
  );

  return (
    <AuthStateProvider value={value}>
      <UserLiveProvider>
        <AuthGlobalUi>{children}</AuthGlobalUi>
      </UserLiveProvider>
    </AuthStateProvider>
  );
}

export function useAuthState() {
  return useContext(AuthContext);
}

export function useAuthSession() {
  const auth = useAuthState();
  return auth.session;
}

export function authIntentForState(auth: Pick<AuthState, "isGuest">): OAuthIntent {
  return auth.isGuest ? "upgrade_guest" : "signin";
}

export function useAuthActions() {
  return useContext(AuthActionsContext);
}

/** Test seam and a small integration boundary for shell consumers. */
export function AuthActionsProvider({ value, children }: { value: AuthActions; children: ReactNode }) {
  return <AuthActionsContext.Provider value={value}>{children}</AuthActionsContext.Provider>;
}

function currentReturnTo() {
  if (typeof window === "undefined") return "/";
  const path = `${window.location.pathname}${window.location.search}${window.location.hash}`;
  return path.startsWith("/") ? path : "/";
}

function googleSignInEnabled(config: ReturnType<typeof getRuntimeConfig>) {
  if (!config.googleClientId || typeof window === "undefined") return false;
  if (config.googleAllowedOrigins.length > 0) {
    return config.googleAllowedOrigins.includes(window.location.origin);
  }
  // Keep the existing development safety rule: localhost must be explicitly
  // allow-listed because Google will otherwise reject the OAuth redirect.
  return window.location.hostname !== "localhost" && window.location.hostname !== "127.0.0.1";
}

function AuthGlobalUi({ children }: { children: ReactNode }) {
  const auth = useAuthState();
  const config = getRuntimeConfig();
  const gateway = getAuthGateway(config);
  const [signInOpen, setSignInOpen] = useState(false);
  const [authLoading, setAuthLoading] = useState(false);
  const [authError, setAuthError] = useState("");
  const oauthCallbackHandledRef = useRef(false);
  const googleEnabled = googleSignInEnabled(config);
  const discordEnabled = !!config.discordClientId;

  useEffect(() => {
    if (typeof window === "undefined" || oauthCallbackHandledRef.current) return;
    const url = new URL(window.location.href);
    const callback = readAuthCallback(url);
    if (!callback.kind) return;
    oauthCallbackHandledRef.current = true;
    clearAuthCallbackParams(url);
    window.history.replaceState({}, "", url.toString());
    if (callback.kind === "error") {
      setAuthError(callback.errorMessage);
      setSignInOpen(true);
      return;
    }
    setAuthLoading(true);
    setAuthError("");
    void (async () => {
      try {
        const session = await gateway.oauthCompleted();
        if (!session) {
          setAuthError(
            "Sign-in completed, but restoring the session failed. Please try again.",
          );
          setSignInOpen(true);
        } else {
          setSignInOpen(false);
        }
      } catch {
        setAuthError(
          "Sign-in completed, but restoring the session failed. Please try again.",
        );
        setSignInOpen(true);
      } finally {
        setAuthLoading(false);
      }
    })();
  }, [gateway]);

  const openSignIn = useCallback(() => {
    setAuthError("");
    setSignInOpen(true);
  }, []);
  const closeSignIn = useCallback(() => {
    if (authLoading) return;
    setSignInOpen(false);
    setAuthError("");
  }, [authLoading]);
  const startProvider = useCallback(async (provider: "google" | "discord") => {
    if (provider === "google" && !googleEnabled) return;
    if (provider === "discord" && !discordEnabled) return;
    setAuthLoading(true);
    setAuthError("");
    try {
      const session = auth.accessToken
        ? await gateway.ensureFreshSession(60_000, {
            allowNicknameRequired: true,
            forceRefresh: false,
          })
        : null;
      const intent = authIntentForState(auth);
      const accessToken = auth.isGuest ? session?.accessToken || auth.accessToken : undefined;
      if (auth.isGuest && !accessToken) throw new Error("Your guest session expired. Please play again to continue.");
      const request = provider === "google" ? requestGoogleStart : requestDiscordStart;
      const data = await request(config, { intent, accessToken, returnTo: currentReturnTo() });
      if (!data.authURL) throw new Error(`Missing ${provider} auth URL`);
      if (typeof window !== "undefined") window.location.assign(data.authURL);
    } catch (error) {
      setAuthLoading(false);
      setAuthError(error instanceof Error && error.message ? error.message : `Failed to start ${provider} sign-in`);
    }
  }, [auth.accessToken, auth.isGuest, config, discordEnabled, gateway, googleEnabled]);
  const devLogin = useCallback(async () => {
    setAuthLoading(true);
    setAuthError("");
    try {
      const session = await gateway.ensurePlayableSession();
      if (!session) throw new Error("Unable to start a playable session");
      setSignInOpen(false);
      setAuthLoading(false);
    } catch (error) {
      setAuthLoading(false);
      setAuthError(error instanceof Error && error.message ? error.message : "Unable to start a playable session");
    }
  }, [gateway]);
  const actions = useMemo<AuthActions>(() => ({
    signInOpen,
    authLoading,
    authError,
    googleEnabled,
    discordEnabled,
    openSignIn,
    closeSignIn,
    startProvider,
    devLogin,
  }), [authError, authLoading, closeSignIn, devLogin, discordEnabled, googleEnabled, openSignIn, signInOpen, startProvider]);

  return (
    <AuthActionsContext.Provider value={actions}>
      {children}
      <AuthSignInModal />
    </AuthActionsContext.Provider>
  );
}

export function AuthSignInModal() {
  const actions = useAuthActions();
  if (!actions.signInOpen) return null;
  return (
    <AppModalShell title="Sign In" onClose={actions.closeSignIn} placement="center">
      <div className="space-y-3">
        {actions.googleEnabled ? (
          <ProviderButton provider="google" loading={actions.authLoading} onClick={() => void actions.startProvider("google")} />
        ) : null}
        {actions.discordEnabled ? (
          <ProviderButton provider="discord" loading={actions.authLoading} onClick={() => void actions.startProvider("discord")} />
        ) : null}
        {!actions.googleEnabled && !actions.discordEnabled ? (
          <Button type="button" variant="secondary" loading={actions.authLoading} onClick={() => void actions.devLogin()}>
            Dev Login
          </Button>
        ) : null}
        {actions.authError ? <p className="text-center text-body-sm font-semibold text-status-danger">{actions.authError}</p> : null}
      </div>
    </AppModalShell>
  );
}

function ProviderButton({ provider, loading, onClick }: { provider: "google" | "discord"; loading: boolean; onClick: () => void }) {
  const google = provider === "google";
  return (
    <Button type="button" variant="secondary" loading={loading} onClick={onClick} className="w-full rounded-xl px-3 py-2.5 text-body-sm font-strong sm:px-4">
      <span className={google ? "flex h-6 w-6 items-center justify-center rounded-full bg-content-primary text-content-inverse shadow-elev-1" : "flex h-6 w-6 items-center justify-center rounded-full bg-status-info text-content-on-action shadow-elev-1"}>
        {google ? <GoogleIcon /> : <DiscordIcon />}
      </span>
      {loading ? (google ? "Opening Google..." : "Signing In...") : google ? "Continue With Google" : "Continue With Discord"}
    </Button>
  );
}

function DiscordIcon() {
  return <svg viewBox="0 0 127.14 96.36" className="h-3.5 w-4" aria-hidden="true"><path fill="currentColor" d="M107.7 8.07A105.15 105.15 0 0 0 81.47 0a72.06 72.06 0 0 0-3.36 6.83 97.68 97.68 0 0 0-29.11 0A72.37 72.37 0 0 0 45.64 0 105.89 105.89 0 0 0 19.39 8.09C2.79 32.65-1.71 56.6.54 80.21a105.73 105.73 0 0 0 32.17 16.15 77.7 77.7 0 0 0 6.89-11.11 68.42 68.42 0 0 1-10.85-5.18c.91-.66 1.8-1.34 2.66-2.04a75.57 75.57 0 0 0 64.32 0c.87.71 1.76 1.39 2.66 2.04a68.68 68.68 0 0 1-10.87 5.19 77 77 0 0 0 6.89 11.1 105.25 105.25 0 0 0 32.19-16.14c2.64-27.38-4.52-51.11-18.9-72.15ZM42.45 65.69c-6.27 0-11.43-5.73-11.43-12.78s5.05-12.79 11.43-12.79 11.54 5.78 11.43 12.79-5.06 12.78-11.43 12.78Zm42.24 0c-6.27 0-11.43-5.73-11.43-12.78s5.05-12.79 11.43-12.79 11.54 5.78 11.43 12.79-5.05 12.78-11.43 12.78Z" /></svg>;
}

function GoogleIcon() {
  return <svg viewBox="0 0 24 24" className="h-3.5 w-3.5" aria-hidden="true"><path fill="var(--gd-brand-google-blue)" d="M21.805 10.023h-9.81v3.955h5.627c-.242 1.272-.967 2.35-2.06 3.073v2.55h3.332c1.95-1.796 3.073-4.44 3.073-7.578 0-.662-.06-1.298-.162-1.999Z" /><path fill="var(--gd-brand-google-green)" d="M11.995 22c2.79 0 5.132-.924 6.842-2.5l-3.332-2.55c-.924.62-2.102.987-3.51.987-2.699 0-4.985-1.822-5.805-4.272H2.758v2.63A10.329 10.329 0 0 0 11.995 22Z" /><path fill="var(--gd-brand-google-yellow)" d="M6.19 13.665a6.214 6.214 0 0 1-.324-1.967c0-.684.118-1.347.324-1.967v-2.63H2.758A10.329 10.329 0 0 0 1.663 11.7c0 1.66.398 3.232 1.095 4.598l3.432-2.633Z" /><path fill="var(--gd-brand-google-red)" d="M11.995 5.463c1.518 0 2.88.523 3.95 1.55l2.962-2.962C17.122 2.397 14.782 1.4 11.995 1.4 7.958 1.4 4.47 3.707 2.758 7.101l3.432 2.63c.82-2.45 3.106-4.268 5.805-4.268Z" /></svg>;
}
