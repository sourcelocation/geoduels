export type Player = {
  userId: string;
  email?: string;
  displayName: string;
  avatarUrl?: string;
  mmr: number;
  gamesPlayed: number;
  wins: number;
  rankedGamesPlayed: number;
  isGuest?: boolean;
  isAdmin: boolean;
  isModerator: boolean;
  isBanned: boolean;
  banReason?: string;
  bannedAt?: string;
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

export type ModerationIncident = {
  id: number;
  subjectUserId: string;
  subjectName?: string;
  status: string;
  severity: string;
  evidenceStrength: string;
  reasonCode: string;
  summary?: string;
  signalCount: number;
  uniqueReporterCount: number;
  latestSignalAt: string;
  assignedTo?: string;
  watchUntil?: string;
  resolvedAt?: string;
  resolutionNote?: string;
  createdAt: string;
  updatedAt: string;
};

export type ModerationTask = {
  id: number;
  incidentId: number;
  status: string;
  queue: string;
  priority: string;
  assignedTo?: string;
  claimedAt?: string;
  claimExpiresAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
  incident: ModerationIncident;
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
};

export type ModerationVerdict = {
  id: number;
  incidentId: number;
  taskId?: number;
  actorUserId?: string;
  actorName?: string;
  verdict: string;
  reasonCode: string;
  note?: string;
  enforcementAction?: string;
  createdAt: string;
};

export type ModerationTimelineItem = {
  id: number;
  incidentId?: number;
  taskId?: number;
  actorUserId?: string;
  eventType: string;
  reasonCode?: string;
  body?: string;
  createdAt: string;
};

export type ModerationReporterState = {
  userId: string;
  reportsSubmitted: number;
  reportsUseful: number;
  reportsDismissed: number;
  reportsInconclusive: number;
  reportsAbusive: number;
  reportWeight: number;
  mutedUntil?: string;
};

export type ModerationMatch = {
  matchId: string;
  mode?: string;
  startedAt?: string;
  endedAt?: string;
  winnerUserId?: string;
  roundCount: number;
  players: Array<{
    userId: string;
    displayName: string;
    totalScore: number;
    finalHp: number;
  }>;
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
  stats: {
    totalMatches: number;
    rankedMatches: number;
    duelMatches: number;
    singleplayerRuns: number;
    wins: number;
    losses: number;
  };
  eloHistory: Array<{
    date: string;
    mmr: number;
    delta: number;
    played: number;
  }>;
};

export type EnforcementAction = {
  id: number;
  targetUserId: string;
  targetName?: string;
  actorUserId?: string;
  actorName?: string;
  sourceIncidentId?: number;
  sourceVerdictId?: number;
  actionType: string;
  reasonCode?: string;
  reasonNote?: string;
  createdAt: string;
};

export type CheatingBanSummary = {
  userId: string;
  reason?: string;
  refunds?: {
    refundsIssued: number;
    totalRefunded: number;
  };
  ipSignupBanned?: boolean;
  incidentIds?: number[];
};

export type ModerationIncidentDetail = {
  incident: ModerationIncident;
  subjectPlayer?: Player;
  tasks: ModerationTask[];
  signals: ModerationSignal[];
  matches: ModerationMatch[];
  verdicts: ModerationVerdict[];
  auditLog: ModerationTimelineItem[];
  reporterState?: ModerationReporterState[];
};

export type ModerationSubjectProfile = {
  player: Player;
  incidents: ModerationIncident[];
  signals: ModerationSignal[];
  enforcement: EnforcementAction[];
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
