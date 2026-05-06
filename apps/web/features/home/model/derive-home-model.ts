import type {
  RoundResultOverlayProps,
  Snapshot,
} from "../../../components/ui/types";
import { INITIAL_MMR } from "../../../lib/elo";
import type { RuntimeConfig } from "../../../lib/runtime-config";
import { deriveUIPhase } from "../../../lib/uiPhase";
import type { SessionState } from "../../auth/controllers/session-controller";
import type { GameState } from "../../game/controllers/game-controller";
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
};

const SINGLEPLAYER_TOTAL_ROUNDS = 5;

export function formatHpPct(maxHP: number, hp: number) {
  return `${Math.max(0, Math.min(100, (hp / maxHP) * 100))}%`;
}

function buildStreetViewSrc(snapshot: Snapshot | null, googleEmbedKey: string) {
  if (!googleEmbedKey || !snapshot?.currentRound) return "";
  const loc = snapshot.currentRound.location;
  if (!loc.panoId) return "";
  const params = new URLSearchParams({
    key: googleEmbedKey,
    fov: "100",
    language: "en",
    pano: loc.panoId,
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
}: Params): HomeViewModel {
  const activeMatchId =
    homeResumeMatchId || match.activeMatchId || match.snapshot?.matchId || "";
  const snapshot =
    routeMatchId && match.snapshot?.matchId === routeMatchId
      ? match.snapshot
      : null;
  const mode = snapshot?.mode === "singleplayer" ? "singleplayer" : "duel";
  const isSingleplayer = mode === "singleplayer";
  const playerIds = Object.keys(snapshot?.players || {});
  const selfId = auth.userId;
  const oppId = playerIds.find((id) => id !== selfId) || "";
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
  const liveSelfHP =
    game.displayHP[selfId] ?? snapshot?.players?.[selfId]?.hp ?? 0;
  const liveOppHP =
    game.displayHP[oppId] ?? snapshot?.players?.[oppId]?.hp ?? 0;
  const totalScore = snapshot?.players?.[selfId]?.totalScore ?? 0;
  const selfHP = showResultStage ? game.resultShownHP.self : liveSelfHP;
  const oppHP = showResultStage ? game.resultShownHP.opp : liveOppHP;
  const resultMode = uiPhase === "round_result" || uiPhase === "match_end";
  const selfPlayer = snapshot?.players?.[selfId];
  const oppPlayer = oppId ? snapshot?.players?.[oppId] : undefined;
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
  const opponentName = oppPlayer?.displayName || "Opponent";
  const selfElo = selfPlayer?.mmr || auth.mmr;
  const opponentElo = oppPlayer?.mmr || INITIAL_MMR;
  const selfIsAdmin = !!(selfPlayer?.isAdmin ?? auth.isAdmin);
  const opponentIsAdmin = !!oppPlayer?.isAdmin;
  const opponentDisconnected = !!oppPlayer?.disconnected;
  const selfAvatarUrl = selfPlayer?.avatarUrl || auth.userAvatar;
  const oppAvatarUrl = oppPlayer?.avatarUrl || "";
  const selfHasNoLinkedAccount =
    !auth.userEmail || !!(selfPlayer?.isGuest ?? auth.isGuest);
  const selfFallback = avatarFallback(
    selfName || auth.userEmail,
    "Y",
    selfHasNoLinkedAccount,
  );
  const oppFallback = avatarFallback(
    oppId || opponentName,
    "O",
    !!oppPlayer?.isGuest,
  );
  const resultPlayerAvatars: Record<string, string | undefined> = {};
  const resultPlayerFallbacks: Record<string, string | undefined> = {};
  const resultPlayerNames: Record<string, string | undefined> = {};
  Object.entries(snapshot?.players || {}).forEach(([id, player]) => {
    resultPlayerNames[id] = player.displayName || player.userId;
    resultPlayerAvatars[id] = player.avatarUrl;
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
  const currentRoundScore = resultSelf?.score || 0;
  const currentRoundDistanceKm = resultSelf?.distanceKm || 0;
  const resultWinner: "self" | "opp" | "tie" =
    isSingleplayer || !resultSelf || !resultOpp
      ? "tie"
      : resultSelf.score === resultOpp.score
        ? "tie"
        : resultSelf.score > resultOpp.score
          ? "self"
          : "opp";
  const resultDamage =
    isSingleplayer || !resultSelf || !resultOpp
      ? 0
      : Math.abs(resultSelf.score - resultOpp.score);
  const showScoreReveal = game.resultPhase !== "base";
  const hudStatusLabel = isSingleplayer
    ? showResultStage
      ? "Round Result"
      : "Singleplayer"
    : showResultStage
      ? "Round Result"
      : match.matchmaking.status;
  const showHudStatus =
    !!hudStatusLabel && hudStatusLabel.toLowerCase() !== "matched";
  const isTimerCritical =
    isRoundTimerRunning &&
    !isSingleplayer &&
    snapshot?.phase === "live" &&
    game.roundMSLeft <= 15_000;
  const isTimerPulseActive =
    isRoundTimerRunning &&
    !isSingleplayer &&
    snapshot?.phase === "live" &&
    game.roundMSLeft < 5_000;
  const timerProgressPct =
    isRoundTimerRunning && !isSingleplayer && snapshot?.phase === "live"
      ? Math.max(
          0,
          Math.min(100, (game.roundMSLeft / config.roundDurationMs) * 100),
        )
      : 100;
  const matchOutcome: "win" | "lose" | "draw" =
    selfHP === oppHP ? "draw" : selfHP > oppHP ? "win" : "lose";
  const isRankedDuel = !isSingleplayer && !snapshot?.unranked;
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

  let resultOverlay: Omit<RoundResultOverlayProps, "mapNode"> | undefined;
  if (roundResult && !isSingleplayer) {
    resultOverlay = {
      roundNumber: roundResult.roundNumber,
      phase: game.resultPhase,
      showScoreReveal,
      winner: resultWinner,
      damage: resultDamage,
      damageMultiplier,
      players: {
        self: {
          name: selfName,
          avatarUrl: selfAvatarUrl,
          fallback: selfFallback,
          hp: game.resultShownHP.self,
          score: resultSelf?.score,
          distanceKm: resultSelf?.distanceKm,
        },
        opp: {
          name: opponentName,
          avatarUrl: oppAvatarUrl,
          fallback: oppFallback,
          hp: game.resultShownHP.opp,
          score: resultOpp?.score,
          distanceKm: resultOpp?.distanceKm,
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
      onboardingRequired: auth.onboardingRequired,
      nicknameInput: auth.nicknameInput,
      nicknameError: auth.nicknameError,
      nicknameSaving: auth.nicknameSaving,
      authLoading: auth.authLoading,
      authError: auth.authError,
      googleSignInEnabled: auth.googleSignInEnabled,
      googleClientId: auth.googleClientId,
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
      privateLobby: {
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
      uiPhase,
      showResultStage,
      showMatchEndPage: game.showMatchEndPage,
      streetViewSrc,
      roundResult,
      roundResults,
      resultOverlay,
      resultPlayerAvatars,
      resultPlayerFallbacks,
      selfName,
      selfAvatarUrl,
      selfFallback,
      selfIsAdmin,
      opponentName,
      opponentIsAdmin,
      opponentDisconnected,
      oppAvatarUrl,
      oppFallback,
      mm,
      ss,
      isRoundTimerRunning,
      timerProgressPct,
      isTimerCritical,
      isTimerPulseActive,
      showHudStatus,
      hudStatusLabel,
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
      totalRounds: isSingleplayer ? SINGLEPLAYER_TOTAL_ROUNDS : undefined,
      userAvatar: auth.userAvatar,
      selfElo,
      opponentElo,
      selfRatingPreview,
      opponentRatingPreview,
      damageMultiplier,
      guessSubmitted: game.guessSubmitted,
      opponentGuessAlert: isSingleplayer ? false : game.opponentGuessAlert,
      connectionIssue: match.connectionIssue,
      modeName: isSingleplayer ? "Practice" : "Moving",
      mapName: "A Source World",
    },
    overlays: {
      onboardingOpen: auth.onboardingRequired && !!auth.userId,
      endMatch:
        uiPhase === "match_end" && game.showMatchEndPage
          ? {
              open: true,
              mode,
              outcome: isSingleplayer ? undefined : matchOutcome,
              selfName,
              opponentName: isSingleplayer ? undefined : opponentName,
              opponentUserId: isSingleplayer ? undefined : oppId,
              selfElo: isSingleplayer ? undefined : selfElo,
              opponentElo: isSingleplayer ? undefined : opponentElo,
              selfEloDelta: selfReceivesEloDelta ? selfEloDelta : undefined,
              opponentEloDelta: opponentReceivesEloDelta
                ? opponentEloDelta
                : undefined,
              selfHP,
              oppHP: isSingleplayer ? undefined : oppHP,
              selfAvatarUrl,
              oppAvatarUrl: isSingleplayer ? undefined : oppAvatarUrl,
              selfFallback,
              oppFallback: isSingleplayer ? undefined : oppFallback,
              selfIsAdmin,
              opponentIsAdmin: isSingleplayer ? undefined : opponentIsAdmin,
              totalScore,
              roundResults,
              resultPlayerNames,
              resultPlayerAvatars,
              resultPlayerFallbacks,
            }
          : { open: false },
    },
    meta: {
      activeMatchId,
      sourceLobbyInviteCode: match.sourceLobbyInviteCode,
      appVersion: config.appVersion,
      maxHP: config.maxHP,
    },
  };
}
