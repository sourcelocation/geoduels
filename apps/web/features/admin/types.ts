export type Player = {
  userId: string;
  email?: string;
  displayName: string;
  avatarUrl?: string;
  mmr: number;
  gamesPlayed: number;
  wins: number;
  rankedGamesPlayed: number;
  trackedMatches: number;
  rankedMatches: number;
  duelMatches: number;
  singleplayerRuns: number;
  losses: number;
  isGuest?: boolean;
  isAdmin: boolean;
  isModerator: boolean;
  isBanned: boolean;
  banReason?: string;
  bannedAt?: string;
  banExpiresAt?: string;
  chatMuteReason?: string;
  chatMutedAt?: string;
  chatMutedUntil?: string;
  reportMuteReason?: string;
  reportMutedAt?: string;
  lastIpAddress?: string;
  reportMutedUntil?: string;
  identities?: AdminUserIdentity[];
};

export type AdminUserIdentity = {
  provider: string;
  providerUserId: string;
  email?: string;
  providerName?: string;
  lastSeenAt?: string;
  deletedAt?: string;
};

export type ModerationSignal = {
  id: number;
  subjectUserId: string;
  subjectName?: string;
  signalType: string;
  source: string;
  severity: string;
  evidenceStrength: string;
  detectorKey?: string;
  detectorVersion?: string;
  reasonCode: string;
  score: number;
  recommendedQueue: boolean;
  reporterUserId?: string;
  reporterName?: string;
  matchId?: string;
  payload?: Record<string, unknown>;
  occurredAt: string;
  createdAt: string;
  reviewedAt?: string;
  reviewedBy?: string;
  outcome?: string;
};

export type ModerationTimelineItem = {
  id: number;
  subjectUserId?: string;
  subjectName?: string;
  actorUserId?: string;
  actorName?: string;
  action: string;
  reason?: string;
  expiresAt?: string;
  signalIds?: number[];
  createdAt: string;
};

export type MatchHistory = {
  matchId: string;
  mode: string;
  startedAt?: string;
  endedAt: string;
  winnerUserId?: string;
};

export type PlayerDetail = {
  player: Player;
};

export type CheatingBanSummary = {
  userId: string;
  reason?: string;
  refunds?: {
    refundsIssued: number;
    totalRefunded: number;
  };
  ipSignupBanned?: boolean;
};

export type ModerationSubjectProfile = {
  player: Player;
  signals: ModerationSignal[];
  log: ModerationTimelineItem[];
};

export type UserRoleGrant = {
  userId: string;
  displayName?: string;
  email?: string;
  role: string;
  grantedBy?: string;
  grantedAt: string;
  reason?: string;
};

export type IPBan = {
  id: number;
  ipAddress: string;
  reason?: string;
  createdAt: string;
};
