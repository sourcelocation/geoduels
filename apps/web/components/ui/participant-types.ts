import type { PlayerBadgeInfo } from "./PlayerBadge";
import type { RatingDeltaPreview } from "./types";

export type PlayerIdentityView = {
  kind: "player";
  id: string;
  name: string;
  avatarUrl?: string;
  avatarFallback: string;
  selectedBadge?: PlayerBadgeInfo | null;
  isAdmin?: boolean;
  isGuest?: boolean;
  rating?: number;
  ratingDelta?: number;
  ratingPreview?: RatingDeltaPreview;
  disconnected?: boolean;
};

export type TeamIdentityView = {
  kind: "team";
  id: string;
  name: string;
  avatarFallback: string;
  avatarColor: string;
  members: PlayerIdentityView[];
};

export type ParticipantIdentityView = PlayerIdentityView | TeamIdentityView;

export type MatchSideConnection =
  | "connected"
  | "degraded"
  | "disconnected";

export type MatchSideView = {
  id: string;
  participant: ParticipantIdentityView;
  hp?: number;
  score?: number;
  distanceKm?: number;
  connection: MatchSideConnection;
};

export type MatchSidesView = {
  self: MatchSideView;
  opponent: MatchSideView;
};
