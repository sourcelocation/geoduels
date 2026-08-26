import type { ReactNode } from "react";
import type { PlayerBadgeInfo } from "../../players/components/PlayerBadge";
import type { MatchSidesView } from "../components/overlays/participant-types";

export type RoundPlayerResult = {
  userId: string;
  lat: number;
  lng: number;
  distanceKm: number;
  score: number;
  damageDealt?: number;
  damageTaken?: number;
  hpAfterRound?: number;
  guessUnixMs?: number;
  guessMs?: number;
};

export type RoundTeamResult = {
  teamId: string;
  representativeUserId?: string;
  lat: number;
  lng: number;
  distanceKm: number;
  score: number;
  damageDealt?: number;
  damageTaken?: number;
  hpAfterRound?: number;
};

export type ResultPhase =
  | "base"
  | "scores"
  | "crush"
  | "damage_travel"
  | "damage_multiplier"
  | "hp_apply";

export type RoundResultWinner = "self" | "opp" | "tie";

export type RoundResultAnimationConfig = {
  timeline: {
    scoresAtMs: number;
    crushAtMs: number;
    damageTravelAtMs: number;
    damageMultiplierAtMs: number;
    hpApplyAtMs: number;
    endPageDelayMs: number;
  };
  overlayEnter: {
    fadeDuration: number;
    mapScaleFrom: number;
    mapSpringStiffness: number;
    mapSpringDamping: number;
    mapDelay: number;
  };
  roundBadge: {
    yFrom: number;
    duration: number;
    ease: string | [number, number, number, number];
  };
  panelGlow: {
    durationMs: number;
  };
  scoreReveal: {
    yFrom: number;
    scaleFrom: number;
    springStiffness: number;
    springDamping: number;
  };
  scoreTravel: {
    durationMs: number;
    minDurationMs: number;
    durationScale: number;
    ease: string | [number, number, number, number];
    exitDurationMs: number;
    exitEase: string | [number, number, number, number];
  };
  scoreImpact: {
    y: number;
    scale: number;
    durationMs: number;
    ease: string | [number, number, number, number];
    idleDurationMs: number;
    idleEase: string | [number, number, number, number];
  };
  hpBar: {
    durationMs: number;
    timingFunction: string;
  };
  hpShake: {
    keyframesX: number[];
    durationMs: number;
  };
};

export type RoundResultOverlayProps = {
  roundNumber: number;
  mapNode: ReactNode;
  phase: ResultPhase;
  showScoreReveal: boolean;
  winner: RoundResultWinner;
  damage: number;
  damageMultiplier: number;
  sides: MatchSidesView;
  hpPct: (hp: number) => string;
};

export type RoundResult = {
  roundId: string;
  roundNumber: number;
  actualLocation: { lat: number; lng: number };
  players: Record<string, RoundPlayerResult>;
  teams?: Record<string, RoundTeamResult>;
  damageMultiplier?: number;
};

export type SnapshotPlayer = {
  userId: string;
  displayName: string;
  mmr: number;
  ratingRd?: number;
  rankedGamesPlayed?: number;
  avatarUrl?: string;
  isGuest?: boolean;
  isAdmin?: boolean;
  selectedBadge?: PlayerBadgeInfo | null;
  teamId?: string;
  hp: number;
  totalScore?: number;
  finalized: boolean;
  disconnected: boolean;
  damageMultiplier?: number;
};

export type RatingDeltaPreview = {
  win: number;
  lose: number;
  draw: number;
};

export type Snapshot = {
  matchId: string;
  mode?: "duel" | "singleplayer" | "team_duel" | "free_for_all";
  config?: {
    ruleset?: "moving" | "no_move" | "nmpz";
    streetNames?: "shown" | "hidden";
    mapKey?: string;
    mapId?: string;
    mapName?: string;
    roundTimerMode?: "none" | "pressure" | "fixed";
    roundTimeLimitMs?: number;
    pressureTimeLimitMs?: number;
    multiplierMode?: "shared" | "individual";
  };
  unranked?: boolean;
  state: string;
  phase: "live" | "round_result" | "ended";
  roundPhase:
    | "round_intro"
    | "round_live"
    | "round_result"
    | "round_transition"
    | "ended";
  phaseStartedAt: number;
  phaseEndsAt: number;
  roundMsLeft: number;
  currentRound?: {
    roundId: string;
    roundNumber: number;
    timerStarted?: boolean;
    location: {
      panoId: string;
      heading?: number;
      pitch?: number;
    };
  };
  lastRoundResult?: RoundResult;
  roundResults?: RoundResult[];
  players: Record<string, SnapshotPlayer>;
  teams?: Record<string, { teamId: string; name?: string; hp?: number; players: string[] }>;
  self?: {
    userId: string;
    currentGuess?: { lat: number; lng: number };
  };
  team?: {
    guesses?: Record<string, { lat: number; lng: number }>;
  };
  ratingPreview?: Record<string, RatingDeltaPreview>;
  eventSequence: number;
  serverUnixMs?: number;
  graceWindowSec?: number;
};

export type TeamPing = {
  id: string;
  roundId: string;
  senderUserId: string;
  lat: number;
  lng: number;
  expiresAt: number;
};

export type UIPhase =
  | "lobby"
  | "queueing"
  | "prematch_countdown"
  | "live_round"
  | "round_result"
  | "match_end";
