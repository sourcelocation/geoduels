import type { PlayerBadgeInfo } from "../../components/ui/PlayerBadge";

export type PublicPlayerProfile = {
  userId: string;
  displayName: string;
  avatarUrl?: string;
  mmr: number;
  ratingRd?: number;
  seasonId?: string;
  gamesPlayed: number;
  wins: number;
  rankedGamesPlayed: number;
  rankedWins: number;
  badges?: PlayerBadgeInfo[];
  selectedBadge?: PlayerBadgeInfo | null;
};

export type PlayerMatchSummary = {
  matchId: string;
  mode: string;
  startedAt?: string;
  endedAt: string;
  winnerUserId?: string;
  outcome: "win" | "loss" | "draw" | "completed";
  ranked: boolean;
  ratingDelta?: number;
  totalScore?: number;
  opponentUserId?: string;
  opponentDisplayName?: string;
};

export type PlayerMatchesPage = {
  matches: PlayerMatchSummary[];
  nextCursor?: string;
};

export type OptionalViewer = {
  userId: string;
  accessToken: string;
  isGuest: boolean;
  isAdmin?: boolean;
  isModerator?: boolean;
  displayName: string;
  avatarUrl?: string;
  mmr?: number;
  selectedBadge?: PlayerBadgeInfo | null;
};
