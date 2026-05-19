import type { LeaderboardSummary } from "../../auth/controllers/session-controller";
import type { UserNotification } from "../../auth/lib/auth-client";
import type { LobbyRuntimeStatus } from "../../lobby/controllers/lobby-controller";
import type { LobbySnapshot, LobbyTeamId, PartyMode } from "../../lobby/lib/lobby-client";
import type { MaintenanceStatus } from "../../matchmaking/lib/queue-client";
import type {
  GameRuleset,
  MatchConfig,
} from "../../matchmaking/lib/queue-client";
import type {
  ChatEmote,
  ChatMessage,
  RatingDeltaPreview,
  RoundResult,
  RoundResultOverlayProps,
  UIPhase,
} from "../../../components/ui/types";
import type { PlayerBadgeInfo } from "../../../components/ui/PlayerBadge";
import type { ParticipantIdentityView } from "../../../components/ui/PlayerIdentity";

export type HomeAuthView = {
  userId: string;
  accessToken: string;
  userEmail: string;
  displayName: string;
  userAvatar: string;
  onboardingRequired: boolean;
  authMigrationRequired?: boolean;
  recoveryAvailable?: boolean;
  linkedProviders?: string[];
  badges?: PlayerBadgeInfo[];
  selectedBadge?: PlayerBadgeInfo | null;
  canPlay?: boolean;
  isAdmin: boolean;
  isModerator?: boolean;
  isGuest: boolean;
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

export type HomeLobbyView = {
  inGame: boolean;
  connected: boolean;
  mmr: number;
  gamesPlayed: number;
  winsPct: number;
  leaderboard: LeaderboardSummary | null;
  leaderboardLoading: boolean;
  status: string;
  queueStartedAt: number | null;
  queueError: string;
  onlinePlayers: number;
  canStartSingleplayer: boolean;
  maintenance: MaintenanceStatus | null;
  changelogEyebrow: string;
  changelogTitle: string;
  changelogMarkdown: string;
  privateLobby: {
    status: LobbyRuntimeStatus;
    snapshot: LobbySnapshot | null;
    inviteCode: string;
    isMember: boolean;
    isOwner: boolean;
    busy: boolean;
    error: string;
  };
};

export type HomeGameView = {
  inGame: boolean;
  mode: "duel" | "singleplayer" | "team_duel" | "free_for_all";
  isSingleplayer: boolean;
  isPointsMode: boolean;
  uiPhase: UIPhase;
  showResultStage: boolean;
  showMatchEndPage: boolean;
  streetViewSrc: string;
  roundResult?: RoundResult;
  roundResults: RoundResult[];
  resultOverlay?: Omit<RoundResultOverlayProps, "mapNode">;
  resultPlayerAvatars: Record<string, string | undefined>;
  resultPlayerFallbacks: Record<string, string | undefined>;
  resultPlayerBorderColors: Record<string, string | undefined>;
  resultPlayerNames: Record<string, string | undefined>;
  participantsById: Record<string, ParticipantIdentityView>;
  selfParticipant: ParticipantIdentityView;
  opponentParticipant: ParticipantIdentityView;
  selfName: string;
  selfAvatarUrl?: string;
  selfFallback: string;
  selfAvatarColor?: string;
  selfIsAdmin: boolean;
  selfSelectedBadge?: PlayerBadgeInfo | null;
  opponentName: string;
  opponentIsAdmin: boolean;
  opponentSelectedBadge?: PlayerBadgeInfo | null;
  opponentDisconnected: boolean;
  oppAvatarUrl?: string;
  oppFallback: string;
  oppAvatarColor?: string;
  mm: string;
  ss: string;
  isRoundTimerRunning: boolean;
  timerProgressPct: number;
  isTimerCritical: boolean;
  isTimerPulseActive: boolean;
  resultMode: boolean;
  selfHP: number;
  oppHP: number;
  totalScore: number;
  currentRoundScore: number;
  currentRoundDistanceKm: number;
  canFinalizeGuess: boolean;
  canAdvanceRound: boolean;
  guess: { lat: number; lng: number } | undefined;
  currentRoundId: string;
  currentRoundNumber: number;
  totalRounds?: number;
  userAvatar: string;
  selfElo: number;
  opponentElo: number;
  selfRatingPreview?: RatingDeltaPreview;
  opponentRatingPreview?: RatingDeltaPreview;
  damageMultiplier: number;
  guessSubmitted: boolean;
  opponentGuessAlert: boolean;
  connectionIssue: string;
  modeName: string;
  mapName: string;
  streetViewInteractive: boolean;
  selfUserId: string;
};

export type HomeOverlaysView = {
  onboardingOpen: boolean;
  notifications: UserNotification[];
  endMatch:
    | {
        open: true;
        mode: "duel" | "singleplayer" | "team_duel" | "free_for_all";
        outcome?: "win" | "lose" | "draw";
        selfName: string;
        opponentName?: string;
        opponentUserId?: string;
        selfElo?: number;
        opponentElo?: number;
        selfEloDelta?: number;
        opponentEloDelta?: number;
        selfHP: number;
        oppHP?: number;
        selfAvatarUrl?: string;
        oppAvatarUrl?: string;
        selfFallback: string;
        oppFallback?: string;
        selfAvatarColor?: string;
        oppAvatarColor?: string;
        selfIsAdmin: boolean;
        opponentIsAdmin?: boolean;
        selfSelectedBadge?: PlayerBadgeInfo | null;
        opponentSelectedBadge?: PlayerBadgeInfo | null;
        totalScore: number;
        roundResults: RoundResult[];
        resultPlayerNames: Record<string, string | undefined>;
        resultPlayerAvatars: Record<string, string | undefined>;
        resultPlayerFallbacks: Record<string, string | undefined>;
        resultPlayerBorderColors: Record<string, string | undefined>;
        participantsById: Record<string, ParticipantIdentityView>;
        selfParticipant: ParticipantIdentityView;
        opponentParticipant?: ParticipantIdentityView;
      }
    | { open: false };
};

export type HomeChatView = {
  conversationId: string;
  messages: ChatMessage[];
  selfUserId: string;
  error: string;
};

export type HomeViewModel = {
  auth: HomeAuthView;
  lobby: HomeLobbyView;
  game: HomeGameView;
  chat: HomeChatView;
  overlays: HomeOverlaysView;
  meta: {
    activeMatchId: string;
    sourceLobbyInviteCode: string;
    appVersion: string;
    maxHP: number;
  };
};

export type HomeActions = {
  joinQueue: (rulesets?: GameRuleset[]) => void;
  startSingleplayer: (ruleset?: GameRuleset) => Promise<string>;
  cancelQueue: () => void;
  createInviteLobby: (mode?: PartyMode) => Promise<boolean>;
  joinInviteLobby: (inviteCode?: string) => Promise<boolean>;
  leavePrivateLobby: () => Promise<void>;
  kickLobbyMember: (userId: string) => Promise<void>;
  transferLobbyOwner: (userId: string) => Promise<void>;
  startPrivateLobby: () => Promise<void>;
  updatePrivateLobbySettings: (config: MatchConfig, mode?: PartyMode) => Promise<void>;
  switchPrivateLobbyTeam: (teamId: LobbyTeamId) => Promise<void>;
  placeGuess: (lat: number, lng: number) => void;
  finalizeGuess: () => void;
  advanceRound: () => boolean;
  forfeitMatch: () => boolean;
  leaveGame: () => void;
  sendChatMessage: (body: string) => boolean;
  sendChatEmote: (emote: ChatEmote) => boolean;
  reportPlayer: (
    reportedUserId: string,
    category?: string,
    reason?: string,
  ) => Promise<void>;
  devLogin: () => Promise<unknown>;
  triggerGoogleSignIn: () => Promise<void>;
  triggerDiscordSignIn?: () => Promise<void>;
  linkAuthProvider: (provider: "google" | "discord") => Promise<void>;
  upgradeGuestWithProvider: (provider: "google" | "discord") => Promise<void>;
  unlinkAuthProvider: (provider: "google" | "discord") => Promise<void>;
  loadLeaderboard: () => void;
  clearAuthSession: (message?: string) => void;
  deleteAccount: () => Promise<void>;
  submitOnboardingNickname: () => Promise<void>;
  submitProfileNickname: () => Promise<boolean>;
  selectBadge: (badgeId: string) => Promise<void>;
  setNicknameInput: (value: string) => void;
  dismissNotification: (notificationId: number) => Promise<void>;
};

export type HomeModel = {
  view: HomeViewModel;
  actions: HomeActions;
};
