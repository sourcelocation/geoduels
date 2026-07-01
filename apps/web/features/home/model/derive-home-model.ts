import type {
  RoundResultOverlayProps,
  Snapshot,
} from "../../../components/ui/types";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import { getTeamPresentation } from "../../../lib/team-presentation";
import { deriveUIPhase } from "../../../lib/uiPhase";
import type { SessionState } from "../../auth/controllers/session-controller";
import type { GameState } from "../../game/controllers/game-controller";
import {
  deriveMatchSides,
  withRoundSideResults,
} from "../../game/model/match-sides";
import type { MaintenanceStatus } from "../../matchmaking/lib/queue-client";
import type { MatchState } from "../../matchmaking/controllers/match-controller";
import type { HomeViewModel } from "./types";

type Params = {
  auth: SessionState;
  match: MatchState;
  game: GameState;
  config: RuntimeConfig;
  homeResumeMatchId?: string;
  routeMatchId?: string | null;
  leaderboardLoading?: boolean;
  maintenance?: MaintenanceStatus | null;
  changelogEyebrow?: string;
  changelogTitle?: string;
  changelogMarkdown?: string;
  changelogSlug?: string;
  changelogUpdatedAt?: string;
};

const SINGLEPLAYER_TOTAL_ROUNDS = 5;
const DEFAULT_PRESSURE_TIME_MS = 15_000;

export function formatHpPct(maxHP: number, hp: number) {
  return `${Math.max(0, Math.min(100, (hp / maxHP) * 100))}%`;
}

function buildStreetViewSrc(snapshot: Snapshot | null, googleEmbedKey: string) {
  if (!snapshot?.currentRound) return "";
  const loc = snapshot.currentRound.location;
  if (!loc.panoId) return "";
  const params = new URLSearchParams({
    pano: loc.panoId,
    key: googleEmbedKey,
    fov: "100",
    language: "en",
  });
  if (Number.isFinite(loc.heading)) {
    params.set("heading", String(loc.heading));
  }
  if (Number.isFinite(loc.pitch)) {
    params.set("pitch", String(loc.pitch));
  }
  return `https://www.google.com/maps/embed/v1/streetview?${params.toString()}`;
}

function getSelfQueueName(params: {
  displayName: string;
  nicknameInput: string;
  userEmail: string;
  userId: string;
}) {
  const direct = params.displayName.trim();
  if (direct && direct !== params.userId) return direct;
  const nick = params.nicknameInput.trim();
  if (nick && nick !== params.userId) return nick;
  const emailLocal = params.userEmail.split("@")[0]?.trim() || "";
  if (emailLocal && emailLocal !== params.userId) return emailLocal;
  return direct;
}

function avatarFallback(
  value: string,
  fallback: string,
  noLinkedAccount = false,
) {
  if (noLinkedAccount) return "?";
  return (value || fallback).slice(0, 1).toUpperCase();
}

export function deriveHomeModel({
  auth,
  match,
  game,
  config,
  homeResumeMatchId = "",
  routeMatchId,
  leaderboardLoading = false,
  maintenance = null,
  changelogEyebrow = "Latest News",
  changelogTitle = "GeoDuels v1.1",
  changelogMarkdown = "",
  changelogSlug = "",
  changelogUpdatedAt = "",
}: Params): HomeViewModel {
  const activeMatchId =
    homeResumeMatchId || match.activeMatchId || match.snapshot?.matchId || "";
  const snapshot =
    routeMatchId && match.snapshot?.matchId === routeMatchId
      ? match.snapshot
      : null;
  const mode =
    snapshot?.mode === "singleplayer" ||
    snapshot?.mode === "team_duel" ||
    snapshot?.mode === "free_for_all"
      ? snapshot.mode
      : "duel";
  const isSingleplayer = mode === "singleplayer";
  const isTeamDuel = mode === "team_duel";
  const isFreeForAll = mode === "free_for_all";
  const isPointsMode = isSingleplayer || isFreeForAll;
  const playerIds = Object.keys(snapshot?.players || {});
  const selfId = auth.userId;
  const selfTeamId = snapshot?.players?.[selfId]?.teamId || "a";
  const oppId = isTeamDuel
    ? playerIds.find((id) => (snapshot?.players?.[id]?.teamId || "a") !== selfTeamId) || ""
    : playerIds.find((id) => id !== selfId) || "";
  const derivedUIPhase = deriveUIPhase({
    snapshot,
    status: match.matchmaking.status,
  });
  const uiPhase =
    derivedUIPhase === "prematch_countdown" && game.roundMSLeft <= 0
      ? "live_round"
      : derivedUIPhase;
  const roundResult =
    snapshot?.lastRoundResult ||
    (snapshot?.matchId &&
    game.persistedRoundResultCtx?.matchId === snapshot.matchId
      ? game.persistedRoundResultCtx.result
      : undefined);
  const roundResults =
    snapshot?.roundResults && snapshot.roundResults.length > 0
      ? snapshot.roundResults
      : roundResult
        ? [roundResult]
        : [];
  const showResultStage =
    uiPhase === "round_result" ||
    (uiPhase === "prematch_countdown" && !!roundResult) ||
    (uiPhase === "match_end" && !game.showMatchEndPage);
  const inGame =
    uiPhase === "live_round" ||
    uiPhase === "prematch_countdown" ||
    uiPhase === "round_result" ||
    uiPhase === "match_end";
  const canFinalizeGuess =
    !!snapshot?.currentRound &&
    !!game.guess &&
    !game.guessSubmitted &&
    snapshot.phase === "live" &&
    snapshot.roundPhase === "round_live" &&
    uiPhase === "live_round";
  const canAdvanceRound =
    isSingleplayer &&
    snapshot?.phase === "round_result" &&
    snapshot.state !== "ended";
  const winsPct =
    auth.gamesPlayed === 0
      ? 0
      : Math.round((auth.wins / auth.gamesPlayed) * 100);
  const currentRoundNumber =
    snapshot?.currentRound?.roundNumber || roundResult?.roundNumber || 1;
  const damageMultiplier =
    currentRoundNumber <= 2 ? 1 : 1 + 0.5 * (currentRoundNumber - 2);
  const isRoundTimerRunning =
    isSingleplayer ||
    !!(
      snapshot?.phase === "live" &&
      snapshot.roundPhase === "round_live" &&
      snapshot.currentRound?.timerStarted === true
    );
  const hideRoundClock = uiPhase === "live_round" && !isRoundTimerRunning;
  const mm = !hideRoundClock
    ? String(Math.floor(game.displayRoundSeconds / 60)).padStart(2, "0")
    : "--";
  const ss = !hideRoundClock
    ? String(game.displayRoundSeconds % 60).padStart(2, "0")
    : "--";
  const selfTeamHP = isTeamDuel ? snapshot?.teams?.[selfTeamId]?.hp : undefined;
  const oppTeamId = isTeamDuel ? Object.keys(snapshot?.teams || {}).find((teamId) => teamId !== selfTeamId) || "" : "";
  const oppTeamHP = isTeamDuel && oppTeamId ? snapshot?.teams?.[oppTeamId]?.hp : undefined;
  const liveSelfHP =
    selfTeamHP ?? game.displayHP[selfId] ?? snapshot?.players?.[selfId]?.hp ?? 0;
  const liveOppHP =
    oppTeamHP ?? game.displayHP[oppId] ?? snapshot?.players?.[oppId]?.hp ?? 0;
  const totalScore = snapshot?.players?.[selfId]?.totalScore ?? 0;
  const selfHP = showResultStage ? game.resultShownHP.self : liveSelfHP;
  const oppHP = showResultStage ? game.resultShownHP.opp : liveOppHP;
  const resultMode = uiPhase === "round_result" || uiPhase === "match_end";
  const selfPlayer = snapshot?.players?.[selfId];
  const oppPlayer = oppId ? snapshot?.players?.[oppId] : undefined;
  const matchConfig = snapshot?.config || {};
  const ruleset =
    matchConfig.ruleset === "nmpz" || matchConfig.ruleset === "no_move"
      ? matchConfig.ruleset
      : "moving";
  const streetNames =
    matchConfig.streetNames === "hidden" ? "hidden" : "shown";
  const pressureTimeLimitMs =
    typeof matchConfig.pressureTimeLimitMs === "number" &&
    matchConfig.pressureTimeLimitMs > 0
      ? matchConfig.pressureTimeLimitMs
      : matchConfig.roundTimerMode === "pressure"
        ? DEFAULT_PRESSURE_TIME_MS
        : 0;
  const finalizedCount = Object.values(snapshot?.players || {}).filter((player) => player.finalized).length;
  const playerCount = Object.keys(snapshot?.players || {}).length;
  const pressureTimerActive =
    pressureTimeLimitMs > 0 &&
    finalizedCount > 0 &&
    finalizedCount < playerCount;
  const roundTimeLimitMs =
    pressureTimerActive
      ? pressureTimeLimitMs
      : matchConfig.roundTimerMode === "fixed" &&
          typeof matchConfig.roundTimeLimitMs === "number"
        ? matchConfig.roundTimeLimitMs
        : config.roundDurationMs;
  const streetViewSrc = buildStreetViewSrc(snapshot, config.googleEmbedKey);
  const selfQueueName = getSelfQueueName({
    displayName: auth.displayName,
    nicknameInput: auth.nicknameInput,
    userEmail: auth.userEmail,
    userId: auth.userId,
  });
  const selfName =
    (selfPlayer?.displayName && selfPlayer.displayName !== auth.userId
      ? selfPlayer.displayName
      : "") ||
    selfQueueName ||
    "You";
  const selfElo = selfPlayer?.mmr || auth.mmr;
  const selfIsAdmin = !!(selfPlayer?.isAdmin ?? auth.isAdmin);
  const selfSelectedBadge = selfPlayer?.selectedBadge || auth.selectedBadge || null;
  const selfHasNoLinkedAccount =
    !auth.userEmail || !!(selfPlayer?.isGuest ?? auth.isGuest);
  const selfFallback = isTeamDuel
    ? getTeamPresentation(selfTeamId).fallback
    : avatarFallback(
        selfName || auth.userEmail,
        "Y",
        selfHasNoLinkedAccount,
      );
  const resultPlayerAvatars: Record<string, string | undefined> = {};
  const resultPlayerFallbacks: Record<string, string | undefined> = {};
  const resultPlayerBorderColors: Record<string, string | undefined> = {};
  const resultPlayerNames: Record<string, string | undefined> = {};
  Object.entries(snapshot?.players || {}).forEach(([id, player]) => {
    resultPlayerNames[id] = player.displayName || player.userId;
    resultPlayerAvatars[id] = player.avatarUrl;
    resultPlayerBorderColors[id] = isTeamDuel
      ? getTeamPresentation(player.teamId).color
      : undefined;
    resultPlayerFallbacks[id] = avatarFallback(
      player.displayName || player.userId,
      "P",
      id === selfId ? selfHasNoLinkedAccount : !!player.isGuest,
    );
  });

  const resultSelf =
    roundResult && selfId ? roundResult.players[selfId] : undefined;
  const resultOpp =
    roundResult && oppId ? roundResult.players[oppId] : undefined;
  const resultSelfTeam =
    isTeamDuel && roundResult ? roundResult.teams?.[selfTeamId] : undefined;
  const resultOppTeam =
    isTeamDuel && roundResult && oppTeamId ? roundResult.teams?.[oppTeamId] : undefined;
  const overlaySelfScore = resultSelfTeam?.score ?? resultSelf?.score;
  const overlayOppScore = resultOppTeam?.score ?? resultOpp?.score;
  const overlaySelfDistanceKm = resultSelfTeam?.distanceKm ?? resultSelf?.distanceKm;
  const overlayOppDistanceKm = resultOppTeam?.distanceKm ?? resultOpp?.distanceKm;
  const currentRoundScore = resultSelf?.score || 0;
  const currentRoundDistanceKm = resultSelf?.distanceKm || 0;
  const resultWinner: "self" | "opp" | "tie" =
    isPointsMode || overlaySelfScore === undefined || overlayOppScore === undefined
      ? "tie"
      : overlaySelfScore === overlayOppScore
        ? "tie"
        : overlaySelfScore > overlayOppScore
          ? "self"
          : "opp";
  const resultDamage =
    isPointsMode || overlaySelfScore === undefined || overlayOppScore === undefined
      ? 0
      : Math.abs(overlaySelfScore - overlayOppScore);
  const showScoreReveal = game.resultPhase !== "base";
  const isTimerCritical =
    isRoundTimerRunning &&
    snapshot?.phase === "live" &&
    game.roundMSLeft <= 15_000;
  const isTimerPulseActive =
    isRoundTimerRunning &&
    snapshot?.phase === "live" &&
    game.roundMSLeft < 5_000;
  const timerProgressPct =
    isRoundTimerRunning && roundTimeLimitMs > 0 && snapshot?.phase === "live"
      ? Math.max(0, Math.min(100, (game.roundMSLeft / roundTimeLimitMs) * 100))
      : 100;
  const matchOutcome: "win" | "lose" | "draw" =
    selfHP === oppHP ? "draw" : selfHP > oppHP ? "win" : "lose";
  const isRankedDuel = mode === "duel" && !snapshot?.unranked;
  const selfRatingPreview =
    isRankedDuel && selfId ? snapshot?.ratingPreview?.[selfId] : undefined;
  const opponentRatingPreview =
    isRankedDuel && oppId ? snapshot?.ratingPreview?.[oppId] : undefined;
  const selfIsGuest = selfPlayer?.isGuest ?? auth.isGuest;
  const oppIsGuest = oppPlayer?.isGuest ?? false;
  const selfReceivesEloDelta =
    isRankedDuel &&
    snapshot?.state === "ended" &&
    !!selfPlayer &&
    !!oppPlayer &&
    !selfIsGuest;
  const opponentReceivesEloDelta =
    isRankedDuel &&
    snapshot?.state === "ended" &&
    !!selfPlayer &&
    !!oppPlayer &&
    !oppIsGuest;
  const selfEloDelta =
    matchOutcome === "win"
      ? selfRatingPreview?.win
      : matchOutcome === "lose"
        ? selfRatingPreview?.lose
        : selfRatingPreview?.draw;
  const opponentEloDelta =
    matchOutcome === "win"
      ? opponentRatingPreview?.lose
      : matchOutcome === "lose"
        ? opponentRatingPreview?.win
        : opponentRatingPreview?.draw;

  const derivedSides = deriveMatchSides({
    snapshot,
    selfUserId: selfId,
    fallbackSelf: {
      id: selfId || "self",
      name: selfName,
      avatarUrl: auth.userAvatar,
      avatarFallback: selfFallback,
      isAdmin: selfIsAdmin,
      isGuest: selfHasNoLinkedAccount,
      selectedBadge: selfSelectedBadge,
      rating: selfElo,
    },
    competitive: {
      selfRatingDelta: selfReceivesEloDelta ? selfEloDelta : undefined,
      opponentRatingDelta: opponentReceivesEloDelta
        ? opponentEloDelta
        : undefined,
      selfRatingPreview,
      opponentRatingPreview,
    },
  });
  const participantsById = derivedSides.playersById;
  const sides = derivedSides.sides;
  const resultSides = withRoundSideResults(sides, roundResult);

  let resultOverlay: Omit<RoundResultOverlayProps, "mapNode"> | undefined;
  if (roundResult && !isPointsMode) {
    resultOverlay = {
      roundNumber: roundResult.roundNumber,
      phase: game.resultPhase,
      showScoreReveal,
      winner: resultWinner,
      damage: resultDamage,
      damageMultiplier,
      sides: {
        self: {
          ...resultSides.self,
          hp: game.resultShownHP.self,
        },
        opponent: {
          ...resultSides.opponent,
          hp: game.resultShownHP.opp,
        },
      },
      hpPct: (hp) => formatHpPct(config.maxHP, hp),
    };
  }

  return {
    auth: {
      userId: auth.userId,
      accessToken: auth.accessToken,
      userEmail: auth.userEmail,
      displayName: auth.displayName,
      userAvatar: auth.userAvatar,
      nicknameRequired: auth.nicknameRequired,
      authMigrationRequired: auth.authMigrationRequired,
      recoveryAvailable: auth.recoveryAvailable,
      linkedProviders: auth.linkedProviders,
      badges: auth.badges,
      selectedBadge: auth.selectedBadge,
      canPlay: auth.canPlay,
      nicknameInput: auth.nicknameInput,
      nicknameError: auth.nicknameError,
      nicknameSaving: auth.nicknameSaving,
      authLoading: auth.authLoading,
      authError: auth.authError,
      googleSignInEnabled: auth.googleSignInEnabled,
      googleClientId: auth.googleClientId,
      discordSignInEnabled: auth.discordSignInEnabled,
      discordClientId: auth.discordClientId,
      isAdmin: auth.isAdmin,
      isModerator: auth.isModerator,
      isGuest: auth.isGuest,
    },
    lobby: {
      inGame,
      connected: match.connected,
      mmr: auth.mmr,
      gamesPlayed: auth.gamesPlayed,
      winsPct,
      leaderboard: auth.leaderboard,
      leaderboardLoading,
      status: match.matchmaking.status,
      queueStartedAt: match.matchmaking.queueStartedAt,
      queueError: match.queueError,
      onlinePlayers: match.onlinePlayers,
      canStartSingleplayer: !inGame && match.matchmaking.status !== "queueing",
      maintenance,
      changelogEyebrow,
      changelogTitle,
      changelogMarkdown,
      changelogSlug,
      changelogUpdatedAt,
      party: {
        status: "idle",
        snapshot: null,
        inviteCode: "",
        isMember: false,
        isOwner: false,
        busy: false,
        error: "",
      },
    },
    game: {
      inGame,
      mode,
      isSingleplayer,
      isPointsMode,
      uiPhase,
      showResultStage,
      showMatchEndPage: game.showMatchEndPage,
      streetViewSrc,
      roundResult,
      roundResults,
      resultOverlay,
      resultPlayerAvatars,
      resultPlayerFallbacks,
      resultPlayerNames,
      participantsById,
      sides,
      resultPlayerBorderColors,
      mm,
      ss,
      isRoundTimerRunning,
      timerProgressPct,
      isTimerCritical,
      isTimerPulseActive,
      resultMode,
      selfHP,
      oppHP,
      totalScore,
      currentRoundScore,
      currentRoundDistanceKm,
      canFinalizeGuess,
      canAdvanceRound,
      guess: game.guess,
      currentRoundId: snapshot?.currentRound?.roundId || "",
      currentRoundNumber,
      totalRounds: isSingleplayer || isFreeForAll ? SINGLEPLAYER_TOTAL_ROUNDS : undefined,
      userAvatar: auth.userAvatar,
      damageMultiplier,
      guessSubmitted: game.guessSubmitted,
      opponentGuessAlert: isPointsMode ? false : game.opponentGuessAlert,
      connectionIssue: match.connectionIssue,
      modeName: isSingleplayer
        ? "Practice"
        : isTeamDuel
          ? "Team Duel"
          : isFreeForAll
            ? "Free for All"
        : ruleset === "nmpz"
          ? "NMPZ"
          : ruleset === "no_move"
            ? "No Move"
            : "Moving",
      mapName: match.snapshot?.config?.mapName || (ruleset === "nmpz" ? "A Location World" : "A Source World"),
      backLabel: match.sourcePartyInviteCode
        ? "Back to party"
        : "Back to lobby",
      streetViewInteractive: ruleset !== "nmpz",
      ruleset,
      streetNames,
      selfUserId: selfId,
    },
    chat: {
      conversationId: "",
      messages: [],
      selfUserId: selfId,
      error: "",
    },
    overlays: {
      nicknameRequiredOpen: auth.nicknameRequired && !!auth.userId,
      notifications: [],
      guestVerification: {
        open: false,
        siteKey: "",
        status: "checking",
        error: "",
        resetKey: 0,
      },
      endMatch:
        uiPhase === "match_end" && game.showMatchEndPage
          ? {
              open: true,
              mode,
              outcome: isPointsMode ? undefined : matchOutcome,
              sides: {
                self: { ...sides.self, hp: selfHP },
                opponent: { ...sides.opponent, hp: oppHP },
              },
              selfUserId: selfId,
              totalScore,
              roundResults,
              resultPlayerNames,
              resultPlayerAvatars,
              resultPlayerFallbacks,
              resultPlayerBorderColors,
              participantsById,
              matchConfig: snapshot?.config,
              backLabel: match.sourcePartyInviteCode
                ? "Back to party"
                : "Back to lobby",
            }
          : { open: false },
    },
    meta: {
      activeMatchId,
      sourcePartyInviteCode: match.sourcePartyInviteCode,
      appVersion: config.appVersion,
      maxHP: config.maxHP,
    },
  };
}
