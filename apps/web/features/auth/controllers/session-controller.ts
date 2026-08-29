import type { RuntimeConfig } from "../../../lib/runtime-config";
import { ObservableStore } from "../../../lib/observable-store";
import { INITIAL_MMR, INITIAL_RATING_RD } from "../../../lib/elo";
import { decodeAccessTokenExpiry } from "../lib/token-expiry";
import type { PlayerBadgeInfo } from "../../players/components/PlayerBadge";
import {
  emptyAuthSession,
  type AuthSessionSnapshot,
} from "../session";
import type { AuthGateway } from "../auth-gateway";

type SessionUser = {
  id?: string;
  email?: string;
  display_name?: string;
  avatar_url?: string;
  isGuest?: boolean;
  isAdmin?: boolean;
  isModerator?: boolean;
};

type AuthPopupPayload = {
  ok?: boolean;
  error?: string;
  accessToken?: string;
  nicknameRequired?: boolean;
  authMigrationRequired?: boolean;
  recoveryAvailable?: boolean;
  linkedProviders?: string[];
  canPlay?: boolean;
  suggestedNickname?: string;
  user?: SessionUser;
};

type SessionPatch = Partial<
  Pick<
    SessionState,
    | "userEmail"
    | "displayName"
    | "userAvatar"
    | "isGuest"
    | "isAdmin"
    | "isModerator"
    | "mmr"
    | "ratingRd"
    | "gamesPlayed"
    | "wins"
    | "rankedGamesPlayed"
    | "rankedWins"
    | "leaderboard"
    | "authLoading"
    | "authError"
    | "nicknameError"
    | "nicknameSaving"
    | "authMigrationRequired"
    | "recoveryAvailable"
    | "linkedProviders"
    | "badges"
    | "selectedBadge"
    | "canPlay"
  >
>;

type ProfileSnapshot = {
  email?: unknown;
  display_name?: unknown;
  avatar_url?: unknown;
  isGuest?: unknown;
  isAdmin?: unknown;
  isModerator?: unknown;
  isBanned?: unknown;
  banReason?: unknown;
  mmr?: unknown;
  ratingRd?: unknown;
  gamesPlayed?: unknown;
  wins?: unknown;
  rankedGamesPlayed?: unknown;
  rankedWins?: unknown;
  badges?: unknown;
  selectedBadge?: unknown;
  linkedProviders?: unknown;
};

export type SessionState = {
  userId: string;
  userEmail: string;
  displayName: string;
  userAvatar: string;
  isGuest: boolean;
  isAdmin: boolean;
  isModerator?: boolean;
  mmr: number;
  ratingRd: number;
  gamesPlayed: number;
  wins: number;
  rankedGamesPlayed: number;
  rankedWins: number;
  leaderboard: LeaderboardSummary | null;
  accessToken: string;
  nicknameRequired: boolean;
  authMigrationRequired?: boolean;
  recoveryAvailable?: boolean;
  linkedProviders?: string[];
  badges?: PlayerBadgeInfo[];
  selectedBadge?: PlayerBadgeInfo | null;
  canPlay?: boolean;
  nicknameInput: string;
  nicknameError: string;
  nicknameSaving: boolean;
  authLoading: boolean;
  authError: string;
  googleSignInEnabled: boolean;
  googleClientId: string;
  discordSignInEnabled?: boolean;
  discordClientId?: string;
};

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

const initialState: SessionState = {
  userId: "",
  userEmail: "",
  displayName: "",
  userAvatar: "",
  isGuest: false,
  isAdmin: false,
  isModerator: false,
  mmr: INITIAL_MMR,
  ratingRd: INITIAL_RATING_RD,
  gamesPlayed: 0,
  wins: 0,
  rankedGamesPlayed: 0,
  rankedWins: 0,
  leaderboard: null,
  accessToken: "",
  nicknameRequired: false,
  authMigrationRequired: false,
  recoveryAvailable: false,
  linkedProviders: [],
  badges: [],
  selectedBadge: null,
  canPlay: false,
  nicknameInput: "",
  nicknameError: "",
  nicknameSaving: false,
  authLoading: false,
  authError: "",
  googleSignInEnabled: false,
  googleClientId: "",
  discordSignInEnabled: false,
  discordClientId: "",
};

const guestSessionExpiredMessage =
  "Guest session expired. Please start again or sign in.";

export class SessionController extends ObservableStore<SessionState> {
  private readonly config: RuntimeConfig;
  private state: SessionState = initialState;
  private session: AuthSessionSnapshot = emptyAuthSession();
  private mounted = true;
  private started = false;
  private readonly onResetSession: () => void;
  private readonly authGateway: AuthGateway;
  private readonly unsubscribeGateway?: () => void;
  private readonly messageHandler: (event: MessageEvent) => void;

  constructor(params: { config: RuntimeConfig; onResetSession: () => void; authGateway: AuthGateway }) {
    super();
    this.config = params.config;
    this.state = {
      ...initialState,
      googleSignInEnabled: !!params.config.googleClientId,
      googleClientId: params.config.googleClientId,
      discordSignInEnabled: !!params.config.discordClientId,
      discordClientId: params.config.discordClientId,
    };
    this.onResetSession = params.onResetSession;
    this.authGateway = params.authGateway;
    this.unsubscribeGateway = this.authGateway.subscribe((session) => {
        if (!session) {
          if (this.state.userId) {
            this.clearAuthSession(
              this.state.isGuest ? guestSessionExpiredMessage : "Session expired. Please sign in again.",
              { skipGateway: true },
            );
          }
          return;
        }
        this.syncGatewaySession(session);
    });
    this.syncGatewaySession(this.authGateway.getSnapshot());
    this.messageHandler = (event: MessageEvent) => {
      const expectedOrigin = (() => {
        if (!this.config.apiURL.trim()) {
          return typeof window !== "undefined" ? window.location.origin : "";
        }
        try {
          return new URL(this.config.apiURL).origin;
        } catch {
          return "";
        }
      })();
      if (expectedOrigin && event.origin !== expectedOrigin) return;
      if (
        !event.data ||
        (event.data.type !== "geoduels:auth" &&
          event.data.type !== "geoduels:google-auth")
      )
        return;
      const payload = (event.data.payload || {}) as AuthPopupPayload;
      if (!payload.ok) {
        this.patchState({
          authLoading: false,
          authError: payload.error || "Login failed",
        });
        return;
      }
      void this.applyLoginPayload(payload);
    };
  }

  start() {
    if (this.started || typeof window === "undefined") return;
    this.mounted = true;
    this.started = true;
    this.syncGoogleSignInState();
    window.addEventListener("message", this.messageHandler);
    this.emit();
  }

  destroy() {
    const wasStarted = this.started;
    this.mounted = false;
    this.started = false;
    if (wasStarted && typeof window !== "undefined") {
      window.removeEventListener("message", this.messageHandler);
    }
    this.unsubscribeGateway?.();
  }

  getState() {
    return this.state;
  }

  private patchState(patch: Partial<SessionState>) {
    this.state = { ...this.state, ...patch };
    if (this.mounted) {
      this.emit();
    }
  }

  private setSessionSnapshot(patch: Partial<AuthSessionSnapshot>) {
    this.session = this.normalizeSessionSnapshot({ ...this.session, ...patch });
  }

  private normalizeSessionSnapshot(
    session: AuthSessionSnapshot,
  ): AuthSessionSnapshot {
    const expiresAt =
      typeof session.expiresAt === "number" && session.expiresAt > 0
        ? session.expiresAt
        : decodeAccessTokenExpiry(session.accessToken);
    return {
      ...session,
      expiresAt,
      authMigrationRequired: !!session.authMigrationRequired,
      recoveryAvailable: !!session.recoveryAvailable,
      linkedProviders: Array.isArray(session.linkedProviders)
        ? session.linkedProviders
        : [],
      canPlay:
        typeof session.canPlay === "boolean"
          ? session.canPlay
          : !session.nicknameRequired && !session.authMigrationRequired,
    };
  }

  private syncGoogleSignInState() {
    if (typeof window === "undefined") return;
    if (!this.config.googleClientId) {
      this.patchState({ googleSignInEnabled: false });
      return;
    }
    const currentOrigin = window.location.origin;
    if (this.config.googleAllowedOrigins.length === 0) {
      const isLocalHost =
        window.location.hostname === "localhost" ||
        window.location.hostname === "127.0.0.1";
      if (isLocalHost) {
        this.patchState({
          googleSignInEnabled: false,
          authError: `Google sign-in is disabled on ${currentOrigin} until NEXT_PUBLIC_GOOGLE_ALLOWED_ORIGINS is set.`,
        });
        return;
      }
      this.patchState({ googleSignInEnabled: true });
      return;
    }
    const allowed = this.config.googleAllowedOrigins.includes(currentOrigin);
    this.patchState({
      googleSignInEnabled: allowed,
      authError: allowed
        ? this.state.authError
        : `Google sign-in is disabled for ${currentOrigin}. Add this origin to Google OAuth and NEXT_PUBLIC_GOOGLE_ALLOWED_ORIGINS.`,
    });
  }

  clearAuthSession = (message?: string, options?: { skipGateway?: boolean }) => {
    const hadSession = !!this.state.userId || !!this.session.userId;
    this.session = emptyAuthSession();
    this.patchState({
      ...initialState,
      googleSignInEnabled: this.state.googleSignInEnabled,
      googleClientId: this.config.googleClientId,
      discordSignInEnabled: this.state.discordSignInEnabled,
      discordClientId: this.config.discordClientId,
      authError: message || "",
    });
    if (hadSession) this.onResetSession();
    // Clear the canonical source only after the local projection is empty. The
    // gateway listener then observes an already-cleared projection and cannot
    // recursively reset the gameplay runtime a second time.
    if (!options?.skipGateway) this.authGateway.clear();
  };

  bootstrapSession = async (options?: { force?: boolean }): Promise<AuthSessionSnapshot | null> => {
    const session = await this.authGateway.bootstrap(options);
    if (!session && this.state.userId) {
      this.clearAuthSession(this.state.isGuest ? guestSessionExpiredMessage : "Session expired. Please sign in again.");
    }
    this.syncGatewaySession(session);
    return session;
  };

  refreshSession = async (): Promise<AuthSessionSnapshot | null> => {
    const session = await this.authGateway.refresh();
    if (!session && this.state.userId) {
      this.clearAuthSession(this.state.isGuest ? guestSessionExpiredMessage : "Session expired. Please sign in again.");
    }
    this.syncGatewaySession(session);
    return session;
  };

  private async applyLoginPayload(data: AuthPopupPayload) {
    const displayName =
      data.user?.display_name || data.suggestedNickname || "Player";
    const sessionSnapshot: AuthSessionSnapshot = {
      userId: data.user?.id || "",
      accessToken: data.accessToken || "",
      nicknameRequired: !!data?.nicknameRequired,
      authMigrationRequired: !!data?.authMigrationRequired,
      recoveryAvailable: !!data?.recoveryAvailable,
      linkedProviders: Array.isArray(data.linkedProviders)
        ? data.linkedProviders.filter((provider): provider is string => typeof provider === "string")
        : [],
      canPlay:
        typeof data.canPlay === "boolean"
          ? data.canPlay
          : !data?.nicknameRequired && !data?.authMigrationRequired,
      nicknameInput: data.suggestedNickname || displayName,
    };
    this.applySessionSnapshot(sessionSnapshot, {
      userEmail: data.user?.email || "",
      displayName,
      userAvatar: data.user?.avatar_url || "",
      isGuest:
        typeof data.user?.isGuest === "boolean" ? data.user.isGuest : false,
      isAdmin:
        typeof data.user?.isAdmin === "boolean" ? data.user.isAdmin : false,
      isModerator:
        typeof data.user?.isModerator === "boolean"
          ? data.user.isModerator
          : false,
      nicknameError: "",
      authError: "",
      authLoading: false,
      mmr: INITIAL_MMR,
      ratingRd: INITIAL_RATING_RD,
      gamesPlayed: 0,
      wins: 0,
      rankedGamesPlayed: 0,
      rankedWins: 0,
      leaderboard: null,
    });
  }

  setNicknameInputAndClearError = (value: string) => {
    this.setSessionSnapshot({ nicknameInput: value });
    this.patchState({
      nicknameInput: value,
      nicknameError: "",
    });
  };

  getSessionSnapshot = (): AuthSessionSnapshot | null => {
    return this.authGateway.getSnapshot();
  };

  async ensureFreshSession(
    minValidityMs = 60_000,
    options?: { allowNicknameRequired?: boolean; forceRefresh?: boolean },
  ): Promise<AuthSessionSnapshot | null> {
    const allowNicknameRequired = !!options?.allowNicknameRequired;
    const forceRefresh = !!options?.forceRefresh;
    const session = await this.authGateway.ensureFreshSession(minValidityMs, {
      forceRefresh,
      allowNicknameRequired,
    });
    this.syncGatewaySession(session);
    return session;
  }

  getPlayableSession = async (): Promise<AuthSessionSnapshot | null> => {
    const session = await this.authGateway.ensurePlayableSession();
    this.syncGatewaySession(session);
    return session;
  };

  setAuthPending(
    patch: Pick<
      Partial<SessionState>,
      "authLoading" | "authError" | "nicknameSaving" | "nicknameError"
    >,
  ) {
    this.patchState(patch);
  }

  applySessionSnapshot(session: AuthSessionSnapshot, patch: SessionPatch) {
    this.authGateway.applySnapshot(session, {
      isGuest: patch.isGuest,
      isAdmin: patch.isAdmin,
      isModerator: patch.isModerator,
      displayName: patch.displayName,
      email: patch.userEmail,
      avatarUrl: patch.userAvatar,
    });
    this.session = this.normalizeSessionSnapshot(session);
    this.patchState({
      userId: this.session.userId,
      accessToken: this.session.accessToken,
      nicknameRequired: this.session.nicknameRequired,
      authMigrationRequired: this.session.authMigrationRequired,
      recoveryAvailable: this.session.recoveryAvailable,
      linkedProviders: this.session.linkedProviders,
      canPlay: this.session.canPlay,
      nicknameInput: this.session.nicknameInput,
      ...patch,
    });
  }

  private syncGatewaySession(session: AuthSessionSnapshot | null) {
    if (!session) return;
    const user = this.authGateway.getPayload()?.user;
    const viewer = this.authGateway.getBootstrapPayload()?.viewer;
    this.session = this.normalizeSessionSnapshot(session);
    this.patchState({
      userId: session.userId,
      accessToken: session.accessToken,
      nicknameRequired: session.nicknameRequired,
      authMigrationRequired: session.authMigrationRequired,
      recoveryAvailable: session.recoveryAvailable,
      linkedProviders: session.linkedProviders,
      canPlay: session.canPlay,
      nicknameInput: session.nicknameInput,
      userEmail: viewer?.email || user?.email || this.state.userEmail,
      displayName:
        viewer?.displayName || user?.display_name || user?.email || this.state.displayName,
      userAvatar: viewer?.avatarUrl || user?.avatar_url || this.state.userAvatar,
      isGuest:
        viewer?.accountType ? viewer.accountType === "guest" : typeof user?.isGuest === "boolean" ? user.isGuest : this.state.isGuest,
      isAdmin:
        typeof viewer?.isAdmin === "boolean" ? viewer.isAdmin : typeof user?.isAdmin === "boolean" ? user.isAdmin : this.state.isAdmin,
      isModerator:
        typeof user?.isModerator === "boolean"
          ? user.isModerator
          : viewer?.isModerator ?? this.state.isModerator,
      mmr: viewer?.mmr ?? this.state.mmr,
      ratingRd: viewer?.ratingRd ?? this.state.ratingRd,
      gamesPlayed: viewer?.gamesPlayed ?? this.state.gamesPlayed,
      wins: viewer?.wins ?? this.state.wins,
      rankedGamesPlayed: viewer?.rankedGamesPlayed ?? this.state.rankedGamesPlayed,
      rankedWins: viewer?.rankedWins ?? this.state.rankedWins,
      selectedBadge: viewer?.selectedBadge as PlayerBadgeInfo | null | undefined,
    });
  }

  applyProfileSnapshot(profile: ProfileSnapshot) {
    const nextDisplayName =
      typeof profile.display_name === "string" && profile.display_name
        ? profile.display_name
        : this.state.displayName;
    this.patchState({
      userEmail:
        typeof profile.email === "string"
          ? profile.email
          : this.state.userEmail,
      displayName: nextDisplayName,
      userAvatar:
        typeof profile.avatar_url === "string"
          ? profile.avatar_url
          : this.state.userAvatar,
      isGuest:
        typeof profile.isGuest === "boolean"
          ? profile.isGuest
          : this.state.isGuest,
      isAdmin:
        typeof profile.isAdmin === "boolean"
          ? profile.isAdmin
          : this.state.isAdmin,
      isModerator:
        typeof profile.isModerator === "boolean"
          ? profile.isModerator
          : this.state.isModerator,
      mmr: typeof profile.mmr === "number" ? profile.mmr : this.state.mmr,
      ratingRd:
        typeof profile.ratingRd === "number"
          ? profile.ratingRd
          : this.state.ratingRd,
      gamesPlayed:
        typeof profile.gamesPlayed === "number"
          ? profile.gamesPlayed
          : this.state.gamesPlayed,
      wins: typeof profile.wins === "number" ? profile.wins : this.state.wins,
      rankedGamesPlayed:
        typeof profile.rankedGamesPlayed === "number"
          ? profile.rankedGamesPlayed
          : this.state.rankedGamesPlayed,
      rankedWins:
        typeof profile.rankedWins === "number"
          ? profile.rankedWins
          : this.state.rankedWins,
      badges: normalizeBadges(profile.badges),
      selectedBadge: normalizeBadge(profile.selectedBadge),
      linkedProviders: Array.isArray(profile.linkedProviders)
        ? profile.linkedProviders.filter((provider): provider is string => typeof provider === "string")
        : this.state.linkedProviders,
      nicknameInput: this.state.nicknameInput || nextDisplayName,
    });
  }

  applyCommittedRating(mmr: number, ratingRd?: number) {
    this.patchState({
      mmr,
      ratingRd:
        typeof ratingRd === "number"
          ? ratingRd
          : this.state.ratingRd,
    });
  }

  applyBadgeSelection(payload: unknown) {
    if (!payload || typeof payload !== "object") return;
    const raw = payload as Record<string, unknown>;
    this.patchState({
      badges: normalizeBadges(raw.badges),
      selectedBadge: normalizeBadge(raw.selectedBadge),
    });
  }

  applyLeaderboardSummary(summary: unknown) {
    this.patchState({
      leaderboard: normalizeLeaderboardSummary(summary),
    });
  }
}

function normalizeBadge(value: unknown): PlayerBadgeInfo | null {
  if (!value || typeof value !== "object") return null;
  const raw = value as Record<string, unknown>;
  return {
    id: typeof raw.id === "string" ? raw.id : "",
    kind: typeof raw.kind === "string" ? raw.kind : "",
    label: typeof raw.label === "string" ? raw.label : "",
    description: typeof raw.description === "string" ? raw.description : "",
    imageUrl: typeof raw.imageUrl === "string" ? raw.imageUrl : "",
    rarity: typeof raw.rarity === "string" ? raw.rarity : undefined,
    level: typeof raw.level === "number" ? raw.level : undefined,
    maxLevel: typeof raw.maxLevel === "number" ? raw.maxLevel : undefined,
    extra: typeof raw.extra === "number" ? raw.extra : undefined,
    owned: typeof raw.owned === "boolean" ? raw.owned : false,
    unobtainable:
      typeof raw.unobtainable === "boolean" ? raw.unobtainable : undefined,
  };
}

function normalizeBadges(value: unknown): PlayerBadgeInfo[] {
  return Array.isArray(value)
    ? value
        .map((badge) => normalizeBadge(badge))
        .filter((badge): badge is PlayerBadgeInfo => !!badge?.id)
    : [];
}

function normalizeLeaderboardSummary(
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
