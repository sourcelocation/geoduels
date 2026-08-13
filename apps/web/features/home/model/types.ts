import type { LeaderboardSummary } from "../../auth/controllers/session-controller";
import type { UserNotification } from "../../auth/lib/auth-client";
import type { PartyRuntimeStatus } from "../../lobby/controllers/party-controller";
import type { PartySnapshot, PartyTeamId, PartyMode } from "../../lobby/lib/party-client";
import type { MaintenanceStatus } from "../../matchmaking/lib/queue-client";
import type {
  GameRuleset,
  MatchConfig,
  QueueVariant,
  StreetNamesVisibility,
} from "../../matchmaking/lib/queue-client";
import type {
  ChatEmote,
  ChatMessage,
  RoundResult,
  RoundResultOverlayProps,
  UIPhase,
} from "../../../components/ui/types";
import type { PlayerBadgeInfo } from "../../../components/ui/PlayerBadge";
import type {
  MatchSidesView,
  PlayerIdentityView,
} from "../../../components/ui/ParticipantIdentity";

export type HomeAuthView = {
  userId: string;
  accessToken: string;
  userEmail: string;
  displayName: string;
  userAvatar: string;
  nicknameRequired: boolean;
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
  changelogSlug: string;
  changelogUpdatedAt: string;
  party: {
    status: PartyRuntimeStatus;
    snapshot: PartySnapshot | null;
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
  participantsById: Record<string, PlayerIdentityView>;
  sides: MatchSidesView;
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
  damageMultiplier: number;
  guessSubmitted: boolean;
  opponentGuessAlert: boolean;
  connectionIssue: string;
  modeName: string;
  mapName: string;
  autoZoomBounds?: { minLat: number; maxLat: number; minLng: number; maxLng: number };
  backLabel?: "Back to lobby" | "Back to party";
  streetViewInteractive: boolean;
  ruleset: GameRuleset;
  streetNames: StreetNamesVisibility;
  selfUserId: string;
};

export type HomeOverlaysView = {
  nicknameRequiredOpen: boolean;
  notifications: UserNotification[];
  guestVerification: {
    open: boolean;
    siteKey: string;
    status: "checking" | "creating" | "error";
    error: string;
    resetKey: number;
  };
  endMatch:
    | {
        open: true;
        mode: "duel" | "singleplayer" | "team_duel" | "free_for_all";
        outcome?: "win" | "lose" | "draw";
        sides: MatchSidesView;
        selfUserId: string;
        totalScore: number;
        roundResults: RoundResult[];
        resultPlayerNames: Record<string, string | undefined>;
        resultPlayerAvatars: Record<string, string | undefined>;
        resultPlayerFallbacks: Record<string, string | undefined>;
        resultPlayerBorderColors: Record<string, string | undefined>;
        participantsById: Record<string, PlayerIdentityView>;
        matchConfig?: MatchConfig;
        backLabel?: "Back to lobby" | "Back to party";
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
    sourcePartyInviteCode: string;
    appVersion: string;
    maxHP: number;
  };
};

export type HomeActions = {
  joinQueue: (queues?: QueueVariant[]) => void;
  startSingleplayer: (config?: MatchConfig) => Promise<string>;
  cancelQueue: () => void;
  createParty: (mode?: PartyMode, config?: MatchConfig) => Promise<boolean>;
  joinParty: (inviteCode?: string) => Promise<boolean>;
  leaveParty: () => Promise<void>;
  kickPartyMember: (userId: string) => Promise<void>;
  transferPartyOwner: (userId: string) => Promise<void>;
  startParty: () => Promise<void>;
  updatePartySettings: (config: MatchConfig, mode?: PartyMode) => Promise<void>;
  switchPartyTeam: (teamId: PartyTeamId) => Promise<void>;
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
  submitRequiredNickname: () => Promise<void>;
  submitProfileNickname: () => Promise<boolean>;
  selectBadge: (badgeId: string) => Promise<void>;
  startSupportDonation: () => Promise<void>;
  setNicknameInput: (value: string) => void;
  dismissNotification: (notificationId: number) => Promise<void>;
  submitGuestVerificationToken: (token: string) => void;
  markGuestVerificationExpired: (message?: string) => void;
  cancelGuestVerification: () => void;
};

export type HomeModel = {
  view: HomeViewModel;
  actions: HomeActions;
};
